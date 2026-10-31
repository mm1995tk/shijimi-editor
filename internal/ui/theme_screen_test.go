package ui

import (
	"regexp"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func TestPaletteResolvesNames(t *testing.T) {
	for _, th := range themes {
		p := th.Palette()
		for _, s := range p.Styles() {
			if !hexColor.MatchString(p[s]) {
				t.Errorf("%s: %s = %q, want #rrggbb", th.Name, s, p[s])
			}
		}
	}
	d := darkPlus.Palette()
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
	g := githubDark.Palette()
	if g[screen.StyleSyntaxControl] != githubDark.Syntax.Keyword {
		t.Errorf("control fallback = %q", g[screen.StyleSyntaxControl])
	}
	if g[screen.StyleIndent] != "#30363d" {
		t.Errorf("indent = %q", g[screen.StyleIndent])
	}
}

func TestPaletteFallbacks(t *testing.T) {
	p := (&Theme{LineNo: "#abcdef", Syntax: syntaxColors{Keyword: "#123456"}, DiagInfo: "#654321"}).Palette()
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
