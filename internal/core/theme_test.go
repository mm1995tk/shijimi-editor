package core

import (
	"testing"
)

func TestLookupTheme(t *testing.T) {
	cases := map[string]string{
		"dark-plus":                  "dark-plus",
		"Dark_Plus":                  "dark-plus",
		"github_dark":                "github-dark",
		"github_dark_dimmed":         "github-dark",
		"github_light_high_contrast": "light-plus",
		"light":                      "light-plus",
		"light-plus":                 "light-plus",
	}
	for in, want := range cases {
		th, ok := lookupTheme(in)
		if !ok || th.Name != want {
			t.Errorf("lookupTheme(%q) = %v, %v; want %s", in, th, ok, want)
		}
	}
	if _, ok := lookupTheme("nope"); ok {
		t.Error("unknown theme should not resolve")
	}
}
