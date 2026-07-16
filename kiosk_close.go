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

const kioskCloseBindingName = "__webatorClose"

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

func installKioskCloseButton(ctx context.Context, cfg *Config, logger *slog.Logger, onClose func()) error {
	if !kioskCloseButtonEnabled(cfg) {
		return nil
	}

	var closeOnce sync.Once
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		binding, ok := ev.(*runtime.EventBindingCalled)
		if !ok || binding.Name != kioskCloseBindingName {
			return
		}
		logger.Info("kiosk close button clicked")
		closeOnce.Do(func() {
			go onClose()
		})
	})

	script, err := kioskCloseButtonScript(kioskCloseButtonLabel(cfg))
	if err != nil {
		return err
	}

	if err := chromedp.Run(ctx,
		runtime.Enable(),
		page.Enable(),
		runtime.AddBinding(kioskCloseBindingName),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(script).
				WithRunImmediately(true).
				Do(ctx)
			return err
		}),
	); err != nil {
		return fmt.Errorf("install kiosk close button: %w", err)
	}

	logger.Info("kiosk close button installed", slog.String("label", kioskCloseButtonLabel(cfg)))
	return nil
}

func kioskCloseButtonScript(label string) (string, error) {
	labelJSON, err := json.Marshal(label)
	if err != nil {
		return "", fmt.Errorf("encode kiosk close button label: %w", err)
	}

	return fmt.Sprintf(`(() => {
  const id = "webator-kiosk-close-button";
  const binding = %q;
  const label = %s;

  function install() {
    const root = document.documentElement;
    if (!root || document.getElementById(id)) return;

    const button = document.createElement("button");
    button.id = id;
    button.type = "button";
    button.textContent = label;
    button.setAttribute("aria-label", label);
    button.style.cssText = [
      "position:fixed",
      "top:12px",
      "right:12px",
      "z-index:2147483647",
      "box-sizing:border-box",
      "min-width:72px",
      "height:36px",
      "padding:0 14px",
      "border:0",
      "border-radius:6px",
      "background:#111827",
      "color:#fff",
      "font:600 14px system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif",
      "line-height:36px",
      "text-align:center",
      "cursor:pointer",
      "box-shadow:0 4px 14px rgba(0,0,0,.28)",
      "opacity:.9"
    ].join(";");
    button.addEventListener("mouseenter", () => { button.style.opacity = "1"; });
    button.addEventListener("mouseleave", () => { button.style.opacity = ".9"; });
    button.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      if (typeof window[binding] === "function") {
        window[binding]("close");
      }
    }, true);

    (document.body || root).appendChild(button);
  }

  function start() {
    install();
    if (!window.__webatorCloseObserver && document.documentElement) {
      window.__webatorCloseObserver = new MutationObserver(install);
      window.__webatorCloseObserver.observe(document.documentElement, {
        childList: true,
        subtree: true
      });
    }
    if (!window.__webatorCloseInterval) {
      window.__webatorCloseInterval = window.setInterval(install, 1000);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start, { once: true });
  }
  start();
})();`, kioskCloseBindingName, string(labelJSON)), nil
}
