package tui

import (
	"fmt"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

func TestApplyThemeChangesColors(t *testing.T) {
	defer applyTheme(core.DefaultTheme)
	applyTheme(&core.DarkPlus)
	dark := cEditorBg
	applyTheme(&core.LightPlus)
	if cEditorBg == dark || cEditorBg != core.LightPlus.EditorBg {
		t.Fatalf("cEditorBg = %s", cEditorBg)
	}
	if core.DiagColor(1) != core.LightPlus.DiagError {
		t.Fatal("diagnostic colors not themed")
	}
	if core.ChromaStyle.Name != "light-plus" {
		t.Fatal("syntax style not themed")
	}
}

func TestTerminalDefaultColorsFollowTheme(t *testing.T) {
	defer applyTheme(core.DefaultTheme)
	for _, th := range []*core.Theme{&core.DarkPlus, &core.LightPlus} {
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
