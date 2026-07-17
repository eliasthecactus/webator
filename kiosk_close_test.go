package main

import (
	"strings"
	"testing"
)

func TestControlPosition(t *testing.T) {
	if got := controlPosition("bottom-left", "top-right"); got != "bottom-left" {
		t.Fatalf("expected valid position, got %q", got)
	}
	if got := controlPosition("middle", "top-right"); got != "top-right" {
		t.Fatalf("expected fallback position, got %q", got)
	}
}

func TestKioskControlsScriptIncludesSPASupportAndNavigation(t *testing.T) {
	script, err := kioskControlsScript(kioskControlsOptions{CloseEnabled: true, CloseLabel: "Close", ClosePosition: "top-right", ControlsEnabled: true, ControlsPosition: "top-left"})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"MutationObserver", "window.setInterval", "window.history.back()", "window.history.forward()", "window.location.reload()", "ContextMenuDisabled", "document.addEventListener(\"contextmenu\"", "pointerdown", "control.style.position = \"static\""} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("script does not contain %q", fragment)
		}
	}
}

func TestContextMenuBlockingDoesNotRequireVisibleControls(t *testing.T) {
	cfg := defaultConfig()
	cfg.Kiosk = false
	cfg.KioskCloseButton = boolPtr(false)
	cfg.BrowserControls = false
	cfg.DisableContextMenu = true
	if !shouldInstallInjectedUI(&cfg) {
		t.Fatal("context-menu blocking must install even without visible controls")
	}
}
