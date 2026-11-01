package tui

import (
	"fmt"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

func TestApplyThemeChangesColors(t *testing.T) {
	defer applyTheme(DefaultTheme)
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
	defer applyTheme(DefaultTheme)
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
