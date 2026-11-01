package core

import (
	"regexp"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
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

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func TestPaletteResolvesNames(t *testing.T) {
	for _, th := range Themes {
		p := th.Palette()
		for _, s := range p.Styles() {
			if !hexColor.MatchString(p[s]) {
				t.Errorf("%s: %s = %q, want #rrggbb", th.Name, s, p[s])
			}
		}
	}
	d := DarkPlus.Palette()
	for s, want := range map[screen.Style]string{
		screen.StyleSyntaxKeyword: "#569cd6",
		screen.StyleDiagError:     "#f14c4c",
		screen.StyleSelection:     "#264f78",
		screen.StyleModeInsert:    "#73c991",
	} {
		if d[s] != want {
			t.Errorf("dark-plus %s = %q, want %q", s, d[s], want)
		}
	}
	// fallbacks match applyTheme: Control -> Keyword, IndentGuide -> LineNo
	g := GithubDark.Palette()
	if g[screen.StyleSyntaxControl] != GithubDark.Syntax.Keyword {
		t.Errorf("control fallback = %q", g[screen.StyleSyntaxControl])
	}
	if g[screen.StyleIndent] != "#30363d" {
		t.Errorf("indent = %q", g[screen.StyleIndent])
	}
}

func TestPaletteFallbacks(t *testing.T) {
	p := (&Theme{LineNo: "#abcdef", Syntax: SyntaxColors{Keyword: "#123456"}, DiagInfo: "#654321"}).Palette()
	if p[screen.StyleIndent] != "#abcdef" {
		t.Errorf("indent fallback = %q, want LineNo", p[screen.StyleIndent])
	}
	if p[screen.StyleSyntaxControl] != "#123456" {
		t.Errorf("control fallback = %q, want Keyword", p[screen.StyleSyntaxControl])
	}
	if p[screen.StyleDiagHint] != "#654321" {
		t.Errorf("hint = %q, want DiagInfo", p[screen.StyleDiagHint])
	}
}
