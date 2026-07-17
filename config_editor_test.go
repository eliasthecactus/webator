package main

import (
	"encoding/json"
	"testing"
)

func TestCloseButtonEditorKeys(t *testing.T) {
	for label, expected := range map[string]string{
		"Close button":               "kiosk_close_button",
		"Close button position":      "kiosk_close_button_position",
		"Close button swap position": "kiosk_close_button_swap_position",
	} {
		if got := settingKey(label); got != expected {
			t.Fatalf("settingKey(%q) = %q, want %q", label, got, expected)
		}
	}
}

func TestSplitCLIArgs(t *testing.T) {
	args, err := splitCLIArgs(`--debug --log-file "my log.json" --app-mode=false`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--debug", "--log-file", "my log.json", "--app-mode=false"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("got %v, want %v", args, want)
		}
	}
	if _, err := splitCLIArgs(`--log-file "unfinished`); err == nil {
		t.Fatal("expected an unclosed quote error")
	}
}

func TestConfigEditorSerializesDestinationsSparsely(t *testing.T) {
	cfg := defaultConfig()
	cfg.AuthStartURL = "https://example.com"
	cfg.Destinations = []Destination{{Name: "New Destination", Tag: "newdest"}}
	editor := &configEditor{enabled: map[string]bool{"auth_start_url": true, "destinations": true}, destinations: cfg.Destinations}
	data, err := editor.marshalEnabledConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Destinations []map[string]json.RawMessage `json:"destinations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Destinations) != 1 {
		t.Fatalf("expected one destination, got %d", len(raw.Destinations))
	}
	if len(raw.Destinations[0]) != 2 || raw.Destinations[0]["name"] == nil || raw.Destinations[0]["tag"] == nil {
		t.Fatalf("expected only name and tag, got %s", data)
	}
}

func TestBuildLaunchArgs(t *testing.T) {
	args := buildLaunchArgs(launchOptions{
		AuthStartURL:    "https://example.com",
		Username:        "user@example.com",
		Password:        "secret",
		TOTPSecret:      "BASE32",
		DestinationTags: "internal,admin",
		Headless:        triDisabled,
		Debug:           true,
	})
	want := []string{
		"--auth-start-url", "https://example.com",
		"--username-value", "user@example.com",
		"--password-value", "secret",
		"--totp-secret", "BASE32",
		"--destination-tags", "internal,admin",
		"--headless=false", "--debug",
	}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("got %v, want %v", args, want)
		}
	}
}
