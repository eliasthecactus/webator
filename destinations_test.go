package main

import "testing"

func TestFilterDestinations_AllTagCaseInsensitive(t *testing.T) {
	dests := []Destination{
		{Name: "A", Tag: "prod"},
		{Name: "B", Tag: "stage"},
	}

	got := filterDestinations(dests, []string{"ALL"})
	if len(got) != len(dests) {
		t.Fatalf("expected all destinations, got %d", len(got))
	}
}

func TestFilterDestinations_CategoryTagCaseInsensitive(t *testing.T) {
	dests := []Destination{
		{Name: "Prod", Tag: "Production"},
		{Name: "Dev", Tag: "dev"},
	}

	got := filterDestinations(dests, []string{"production"})
	if len(got) != 1 || got[0].Name != "Prod" {
		t.Fatalf("expected only Prod destination, got %#v", got)
	}
}

func TestFilterDestinations_URLTagCaseInsensitive(t *testing.T) {
	dests := []Destination{
		{
			Name: "Apps",
			URLs: []DestinationURL{
				{Label: "Admin", Tag: "Admin"},
				{Label: "User", Tag: "user"},
			},
		},
	}

	got := filterDestinations(dests, []string{"ADMIN"})
	if len(got) != 1 {
		t.Fatalf("expected one destination, got %d", len(got))
	}
	if len(got[0].URLs) != 1 || got[0].URLs[0].Label != "Admin" {
		t.Fatalf("expected only Admin URL, got %#v", got[0].URLs)
	}
}
