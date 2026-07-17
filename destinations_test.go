package main

import "testing"

func boolPtr(v bool) *bool {
	return &v
}

func TestApplyDestinationMergesIgnoreCertErrors(t *testing.T) {
	t.Run("category overrides root", func(t *testing.T) {
		cfg := Config{IgnoreCertErrors: false}
		parent := &Destination{IgnoreCertErrors: boolPtr(true)}

		applyDestination(&cfg, parent, DestinationURL{})

		if !cfg.IgnoreCertErrors {
			t.Fatal("expected category ignore_cert_errors=true to override root false")
		}
	})

	t.Run("url false overrides category true", func(t *testing.T) {
		cfg := Config{IgnoreCertErrors: false}
		parent := &Destination{IgnoreCertErrors: boolPtr(true)}
		chosen := DestinationURL{IgnoreCertErrors: boolPtr(false)}

		applyDestination(&cfg, parent, chosen)

		if cfg.IgnoreCertErrors {
			t.Fatal("expected URL ignore_cert_errors=false to override category true")
		}
	})

	t.Run("url true overrides category false", func(t *testing.T) {
		cfg := Config{IgnoreCertErrors: false}
		parent := &Destination{IgnoreCertErrors: boolPtr(false)}
		chosen := DestinationURL{IgnoreCertErrors: boolPtr(true)}

		applyDestination(&cfg, parent, chosen)

		if !cfg.IgnoreCertErrors {
			t.Fatal("expected URL ignore_cert_errors=true to override category false")
		}
	})
}

func TestApplyDestinationMergesTOTPSubmitSelector(t *testing.T) {
	cfg := Config{TOTPSubmitSelector: "#root-verify"}
	parent := &Destination{TOTPSubmitSelector: "#category-verify"}
	chosen := DestinationURL{TOTPSubmitSelector: "#url-verify"}

	applyDestination(&cfg, parent, chosen)

	if cfg.TOTPSubmitSelector != "#url-verify" {
		t.Fatalf("expected URL TOTP submit selector, got %q", cfg.TOTPSubmitSelector)
	}
}

func TestApplyDestinationMergesBrowserWindowOptions(t *testing.T) {
	cfg := Config{
		Kiosk:                 false,
		KioskCloseButton:      boolPtr(false),
		KioskCloseButtonLabel: "Root Close",
		StartMaximized:        false,
		AppMode:               true,
	}
	parent := &Destination{
		Kiosk:                 boolPtr(true),
		KioskCloseButton:      boolPtr(true),
		KioskCloseButtonLabel: "Category Close",
		StartMaximized:        boolPtr(true),
		AppMode:               boolPtr(false),
	}
	chosen := DestinationURL{
		Kiosk:                 boolPtr(false),
		KioskCloseButtonLabel: "URL Close",
		AppMode:               boolPtr(true),
	}

	applyDestination(&cfg, parent, chosen)

	if cfg.Kiosk {
		t.Fatal("expected URL kiosk=false to override category true")
	}
	if cfg.KioskCloseButton == nil || !*cfg.KioskCloseButton {
		t.Fatal("expected category kiosk_close_button=true to override root false")
	}
	if cfg.KioskCloseButtonLabel != "URL Close" {
		t.Fatalf("expected URL close button label, got %q", cfg.KioskCloseButtonLabel)
	}
	if !cfg.StartMaximized {
		t.Fatal("expected category start_maximized=true to override root false")
	}
	if !cfg.AppMode {
		t.Fatal("expected URL app_mode=true to override category false")
	}
}

func TestApplyDestinationMergesInjectedControls(t *testing.T) {
	cfg := defaultConfig()
	cfg.KioskCloseButtonPosition = "top-right"
	parent := &Destination{
		KioskCloseButtonPosition:     "bottom-left",
		KioskCloseButtonSwapPosition: boolPtr(true),
		BrowserControls:              boolPtr(true),
		BrowserControlsPosition:      "bottom-right",
		BrowserControlsSwapPosition:  boolPtr(true),
	}
	chosen := DestinationURL{
		KioskCloseButtonPosition:    "top-left",
		BrowserControlsPosition:     "top-right",
		BrowserControlsSwapPosition: boolPtr(false),
	}

	applyDestination(&cfg, parent, chosen)

	if cfg.KioskCloseButtonPosition != "top-left" || !cfg.KioskCloseButtonSwapPosition {
		t.Fatalf("unexpected close control settings: position=%q swap=%t", cfg.KioskCloseButtonPosition, cfg.KioskCloseButtonSwapPosition)
	}
	if !cfg.BrowserControls || cfg.BrowserControlsPosition != "top-right" || cfg.BrowserControlsSwapPosition {
		t.Fatalf("unexpected browser controls settings: enabled=%t position=%q swap=%t", cfg.BrowserControls, cfg.BrowserControlsPosition, cfg.BrowserControlsSwapPosition)
	}
}

func TestApplyDestinationMergesBrowserSecuritySettings(t *testing.T) {
	cfg := defaultConfig()
	parent := &Destination{
		DisableContextMenu:     boolPtr(false),
		DisableDevTools:        boolPtr(false),
		DisableTranslate:       boolPtr(false),
		DisablePinch:           boolPtr(false),
		DisableTouchAdjustment: boolPtr(false),
		Incognito:              boolPtr(true),
	}
	chosen := DestinationURL{
		DisableContextMenu: boolPtr(true),
		DisableDevTools:    boolPtr(true),
	}

	applyDestination(&cfg, parent, chosen)

	if !cfg.DisableContextMenu || !cfg.DisableDevTools {
		t.Fatal("expected URL security overrides to win")
	}
	if cfg.DisableTranslate || cfg.DisablePinch || cfg.DisableTouchAdjustment {
		t.Fatal("expected destination security overrides to apply")
	}
	if !cfg.Incognito {
		t.Fatal("expected destination incognito override to apply")
	}
}
