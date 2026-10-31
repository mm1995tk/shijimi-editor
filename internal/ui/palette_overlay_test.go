package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestPaletteOverlay(t *testing.T) {
	a := goldenApp(t)
	a.commands = []command{{name: "Save", key: "Ctrl+S"}, {name: "Quit", key: "Ctrl+Q"}}
	a.pal.show(a.root, ">")
	keys(a, "q", "t")
	a.fit()
	ov := a.pal.overlay()
	if ov.Kind != screen.OverlayPalette || ov.At != nil || ov.Field == nil {
		t.Fatalf("overlay = %+v", ov)
	}
	if ov.Field.Text != ">qt" || ov.Field.Cursor != 3 || ov.Field.Prefix != ">" {
		t.Errorf("field = %+v", ov.Field)
	}
	if len(ov.Items) != 1 || ov.Items[0].Label != "Quit" || ov.Items[0].Detail != "Ctrl+Q" {
		t.Fatalf("items = %+v", ov.Items)
	}
	if got := ov.Items[0].Matched; len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Errorf("matched = %v, want [0 3]", got)
	}

	a.pal.ask("名前", "x", func(string) tea.Cmd { return nil })
	ov = a.pal.overlay()
	if ov.Title != "名前" || ov.Field.Prefix != ":" || ov.Hint != "" || len(ov.Items) != 0 {
		t.Errorf("prompt overlay = %+v", ov)
	}
}

func TestPaletteOverlayScrolls(t *testing.T) {
	a := goldenApp(t)
	items := make([]paletteItem, 30)
	for i := range items {
		items[i] = paletteItem{label: string(rune('a' + i%26))}
	}
	a.pal.pick("t", items, func(int) tea.Cmd { return nil })
	a.fit()
	a.pal.sel = 20
	ov := a.pal.overlay()
	if len(ov.Items) != maxPaletteItems || ov.Active != maxPaletteItems-1 {
		t.Errorf("items=%d active=%d", len(ov.Items), ov.Active)
	}
}

func TestRuneIndexes(t *testing.T) {
	got := runeIndexes("保存as", []int{3, 6, 7})
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", got)
	}
	if runeIndexes("abc", nil) != nil {
		t.Error("no offsets must give nil")
	}
}
