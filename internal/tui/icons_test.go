package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"
)

func TestIconForFollowsTheme(t *testing.T) {
	prev := core.CurTheme
	t.Cleanup(func() { applyTheme(prev) })

	n := core.Node{Name: "main.go"}
	applyTheme(&core.DarkPlus)
	if got := core.IconFor(&n); got != core.IcoGo {
		t.Fatalf("dark-plus: got %+v want %+v", got, core.IcoGo)
	}

	custom := core.DarkPlus
	custom.Icons = map[string]core.IconOverride{"go": {Color: "#123456"}}
	applyTheme(&custom)
	got := core.IconFor(&n)
	if got.Color != "#123456" || got.Glyph != core.IcoGo.Glyph {
		t.Errorf("color override: got %+v", got)
	}

	custom.Icons = map[string]core.IconOverride{"go": {Glyph: "G"}}
	applyTheme(&custom)
	got = core.IconFor(&n)
	if got.Glyph != "G" || got.Color != core.IcoGo.Color {
		t.Errorf("glyph override: got %+v", got)
	}
}
