package ui

import (
	"fmt"
	"testing"

	"github.com/mm1995tk/gh-editor/internal/vt"
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

func TestApplyThemeChangesColors(t *testing.T) {
	defer applyTheme(&darkPlus)
	applyTheme(&darkPlus)
	dark := cEditorBg
	applyTheme(&lightPlus)
	if cEditorBg == dark || cEditorBg != lightPlus.EditorBg {
		t.Fatalf("cEditorBg = %s", cEditorBg)
	}
	if diagColor(1) != lightPlus.DiagError {
		t.Fatal("diagnostic colors not themed")
	}
	if chromaStyle.Name != "light-plus" {
		t.Fatal("syntax style not themed")
	}
}

func TestTerminalDefaultColorsFollowTheme(t *testing.T) {
	defer applyTheme(&darkPlus)
	for _, th := range []*Theme{&darkPlus, &lightPlus} {
		applyTheme(th)
		bg := hexRGB(th.EditorBg)
		want := fmt.Sprintf("48;2;%d;%d;%d", bg[0], bg[1], bg[2])
		if got := sgrColor(vt.Color(0), false, false); got != want {
			t.Errorf("%s: default bg = %s, want %s", th.Name, got, want)
		}
		if bg == [3]int{} {
			t.Errorf("%s: default bg is black", th.Name)
		}
	}
}
