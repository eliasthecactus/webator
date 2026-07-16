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
