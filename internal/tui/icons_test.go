package tui

import "testing"

func TestIconForFollowsTheme(t *testing.T) {
	prev := CurTheme
	t.Cleanup(func() { applyTheme(prev) })

	n := node{name: "main.go"}
	applyTheme(&darkPlus)
	if got := iconFor(&n); got != icoGo {
		t.Fatalf("dark-plus: got %+v want %+v", got, icoGo)
	}

	custom := darkPlus
	custom.Icons = map[string]iconOverride{"go": {Color: "#123456"}}
	applyTheme(&custom)
	got := iconFor(&n)
	if got.color != "#123456" || got.glyph != icoGo.glyph {
		t.Errorf("color override: got %+v", got)
	}

	custom.Icons = map[string]iconOverride{"go": {Glyph: "G"}}
	applyTheme(&custom)
	got = iconFor(&n)
	if got.glyph != "G" || got.color != icoGo.color {
		t.Errorf("glyph override: got %+v", got)
	}
}
