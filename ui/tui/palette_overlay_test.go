package tui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

func TestPaletteOverlay(t *testing.T) {
	a := goldenApp(t)
	a.Commands = []core.Command{{Name: msg.S("Save"), Key: msg.S("Ctrl+S")}, {Name: msg.S("Quit"), Key: msg.S("Ctrl+Q")}}
	a.Pal.Show(a.Root, ">")
	keys(a, "q", "t")
	a.fit()
	ov := a.Pal.Overlay()
	if ov.Kind != screen.OverlayPalette || ov.At != nil || ov.Field == nil {
		t.Fatalf("overlay = %+v", ov)
	}
	if ov.Field.Text != ">qt" || ov.Field.Cursor != 3 || ov.Field.Prefix != ">" {
		t.Errorf("field = %+v", ov.Field)
	}
	if len(ov.Items) != 1 || tr(ov.Items[0].Label) != "Quit" || tr(ov.Items[0].Detail) != "Ctrl+Q" {
		t.Fatalf("items = %+v", ov.Items)
	}
	if got := ov.Items[0].Matched; len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Errorf("matched = %v, want [0 3]", got)
	}

	a.Pal.Ask(msg.S("名前"), "x", func(string) []core.Effect { return nil })
	ov = a.Pal.Overlay()
	if tr(ov.Title) != "名前" || ov.Field.Prefix != ":" || !ov.Hint.IsZero() || len(ov.Items) != 0 {
		t.Errorf("prompt overlay = %+v", ov)
	}
}

func TestPaletteOverlayScrolls(t *testing.T) {
	a := goldenApp(t)
	items := make([]core.PaletteItem, 30)
	for i := range items {
		items[i] = core.PaletteItem{Label: msg.S(string(rune('a' + i%26)))}
	}
	a.Pal.Pick(msg.S("t"), items, func(int) []core.Effect { return nil })
	a.fit()
	a.Pal.Sel = 20
	ov := a.Pal.Overlay()
	if len(ov.Items) != core.MaxPaletteItems || ov.Active != core.MaxPaletteItems-1 {
		t.Errorf("items=%d active=%d", len(ov.Items), ov.Active)
	}
}
