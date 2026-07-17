package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const kioskControlsBindingName = "__webatorBrowserControl"

func kioskCloseButtonEnabled(cfg *Config) bool {
	if cfg.KioskCloseButton != nil {
		return *cfg.KioskCloseButton
	}
	return cfg.Kiosk
}

func kioskCloseButtonLabel(cfg *Config) string {
	label := strings.TrimSpace(cfg.KioskCloseButtonLabel)
	if label == "" {
		return "Close"
	}
	return label
}

func shouldInstallInjectedUI(cfg *Config) bool {
	return kioskCloseButtonEnabled(cfg) || cfg.BrowserControls || cfg.DisableContextMenu
}

func controlPosition(value, fallback string) string {
	switch value {
	case "top-left", "top-right", "bottom-left", "bottom-right":
		return value
	default:
		return fallback
	}
}

// installKioskCloseButton installs the optional close button and navigation
// controls. The historic name is kept because callers already use it.
func installKioskCloseButton(ctx context.Context, cfg *Config, logger *slog.Logger, onClose func()) error {
	if !shouldInstallInjectedUI(cfg) {
		return nil
	}

	var closeOnce sync.Once
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		binding, ok := ev.(*runtime.EventBindingCalled)
		if !ok || binding.Name != kioskControlsBindingName {
			return
		}
		var command string
		if err := json.Unmarshal([]byte(binding.Payload), &command); err != nil {
			logger.Warn("invalid browser control command", slog.String("error", err.Error()))
			return
		}
		switch command {
		case "close":
			logger.Info("kiosk close button clicked")
			closeOnce.Do(func() { go onClose() })
		}
	})

	script, err := kioskControlsScript(kioskControlsOptions{
		CloseEnabled:        kioskCloseButtonEnabled(cfg),
		CloseLabel:          kioskCloseButtonLabel(cfg),
		ClosePosition:       controlPosition(cfg.KioskCloseButtonPosition, "top-right"),
		CloseCanSwap:        cfg.KioskCloseButtonSwapPosition,
		ControlsEnabled:     cfg.BrowserControls,
		ControlsPosition:    controlPosition(cfg.BrowserControlsPosition, "top-left"),
		ControlsCanSwap:     cfg.BrowserControlsSwapPosition,
		ContextMenuDisabled: cfg.DisableContextMenu,
	})
	if err != nil {
		return err
	}

	if err := chromedp.Run(ctx,
		runtime.Enable(),
		page.Enable(),
		runtime.AddBinding(kioskControlsBindingName),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(script).WithRunImmediately(true).Do(ctx)
			return err
		}),
	); err != nil {
		return fmt.Errorf("install browser controls: %w", err)
	}

	logger.Info("browser controls installed", slog.Bool("close_button", kioskCloseButtonEnabled(cfg)), slog.Bool("navigation", cfg.BrowserControls))
	return nil
}

type kioskControlsOptions struct {
	CloseEnabled, CloseCanSwap, ControlsEnabled, ControlsCanSwap, ContextMenuDisabled bool
	CloseLabel, ClosePosition, ControlsPosition                                       string
}

func kioskControlsScript(options kioskControlsOptions) (string, error) {
	data, err := json.Marshal(options)
	if err != nil {
		return "", fmt.Errorf("encode browser controls: %w", err)
	}
	return fmt.Sprintf(`(() => {
  const options = %s;
  const binding = %q;
  const offset = 12;
  const holdMs = 650;

  if (options.ContextMenuDisabled) {
    document.addEventListener("contextmenu", event => { event.preventDefault(); }, true);
  }

  function opposite(position) {
    return position.endsWith("left") ? position.replace("left", "right") : position.replace("right", "left");
  }
  function place(element, position) {
    element.style.top = ""; element.style.right = ""; element.style.bottom = ""; element.style.left = "";
    const [vertical, horizontal] = position.split("-");
    element.style[vertical] = offset + "px";
    element.style[horizontal] = offset + "px";
  }
  function wireSwap(element, position, enabled) {
    let current = position;
    let timer;
    const swap = () => { current = opposite(current); place(element, current); };
    if (!enabled) return;
    element.addEventListener("contextmenu", event => { event.preventDefault(); swap(); }, true);
    element.addEventListener("pointerdown", () => { timer = window.setTimeout(() => { swap(); element.__webatorSuppressClick = true; }, holdMs); }, true);
    for (const eventName of ["pointerup", "pointercancel", "pointerleave"]) {
      element.addEventListener(eventName, () => { if (timer) window.clearTimeout(timer); timer = undefined; }, true);
    }
  }
  function send(command) {
    if (typeof window[binding] === "function") window[binding](command);
  }
  function style(element) {
    element.style.cssText = [
      "position:fixed", "z-index:2147483647", "box-sizing:border-box",
      "height:36px", "border:0", "border-radius:6px", "background:#111827", "color:#fff",
      "font:600 14px system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif",
      "line-height:36px", "text-align:center", "cursor:pointer", "box-shadow:0 4px 14px rgba(0,0,0,.28)", "opacity:.9"
    ].join(";");
    element.addEventListener("mouseenter", () => { element.style.opacity = "1"; });
    element.addEventListener("mouseleave", () => { element.style.opacity = ".9"; });
  }
  function button(id, label, command) {
    const element = document.createElement("button");
    element.id = id; element.type = "button"; element.textContent = label; element.setAttribute("aria-label", label); style(element);
    element.addEventListener("click", event => {
      event.preventDefault(); event.stopPropagation();
      if (element.__webatorSuppressClick) { element.__webatorSuppressClick = false; return; }
      if (command === "back") { window.history.back(); return; }
      if (command === "forward") { window.history.forward(); return; }
      if (command === "reload") { window.location.reload(); return; }
      send(command);
    }, true);
    return element;
  }
  function install() {
    const root = document.documentElement;
    if (!root) return;
    if (options.CloseEnabled && !document.getElementById("webator-kiosk-close-button")) {
      const close = button("webator-kiosk-close-button", options.CloseLabel, "close");
      close.style.minWidth = "72px"; close.style.padding = "0 14px";
      place(close, options.ClosePosition); wireSwap(close, options.ClosePosition, options.CloseCanSwap);
      (document.body || root).appendChild(close);
    }
    if (options.ControlsEnabled && !document.getElementById("webator-browser-controls")) {
      const controls = document.createElement("div"); controls.id = "webator-browser-controls";
      controls.style.cssText = "position:fixed;z-index:2147483647;display:flex;gap:6px";
      for (const [id, label, command] of [["back", "Back", "back"], ["forward", "Forward", "forward"], ["reload", "Refresh", "reload"]]) {
        const control = button("webator-browser-" + id, label, command); control.style.position = "static"; control.style.minWidth = "36px"; control.style.padding = "0 10px"; controls.appendChild(control);
      }
      place(controls, options.ControlsPosition); wireSwap(controls, options.ControlsPosition, options.ControlsCanSwap);
      (document.body || root).appendChild(controls);
    }
  }
  function start() {
    install();
    if (!window.__webatorControlsObserver && document.documentElement) {
      window.__webatorControlsObserver = new MutationObserver(install);
      window.__webatorControlsObserver.observe(document.documentElement, { childList:true, subtree:true });
    }
    if (!window.__webatorControlsInterval) window.__webatorControlsInterval = window.setInterval(install, 1000);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start, { once:true });
  start();
})();`, string(data), kioskControlsBindingName), nil
}
