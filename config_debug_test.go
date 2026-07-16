package main

import (
	"strings"
	"testing"
)

func TestRedactedConfigJSONHidesSecrets(t *testing.T) {
	cfg := &Config{
		PasswordValue: "root-password",
		TOTPSecret:    "root-totp",
		Destinations: []Destination{
			{
				PasswordValue: "category-password",
				TOTPSecret:    "category-totp",
				URLs: []DestinationURL{
					{
						PasswordValue: "url-password",
						TOTPSecret:    "url-totp",
					},
				},
			},
		},
	}

	rendered, err := redactedConfigJSON(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{
		"root-password",
		"root-totp",
		"category-password",
		"category-totp",
		"url-password",
		"url-totp",
	} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("redacted config leaked secret %q in %s", secret, rendered)
		}
	}

	if count := strings.Count(rendered, redactedSecret); count != 6 {
		t.Fatalf("expected 6 redaction markers, got %d in %s", count, rendered)
	}
}
