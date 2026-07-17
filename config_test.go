package main

import "testing"

func TestValidateConfigJSONRejectsUnknownSetting(t *testing.T) {
	if err := validateConfigJSON([]byte(`{"auth_start_url":"https://example.com","unsupported_flag":true}`)); err == nil {
		t.Fatal("expected unsupported setting to be rejected")
	}
}

func TestValidateConfigRejectsDuplicateTags(t *testing.T) {
	cfg := defaultConfig()
	cfg.Destinations = []Destination{
		{Name: "One", Tag: "internal"},
		{Name: "Two", URLs: []DestinationURL{{Label: "Target", Tag: "INTERNAL"}}},
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected duplicate tags to be rejected")
	}
}
