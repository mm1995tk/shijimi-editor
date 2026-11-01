package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestMenuOverlay(t *testing.T) {
	a := newTestApp(t, "x\n")
	items := a.SpaceMenuItems()
	ov := core.MenuOverlay(" Space", items)
	if ov.Kind != screen.OverlayMenu || ov.Title != " Space" || ov.Pane != "" || ov.At != nil || len(ov.Items) != len(items) {
		t.Fatalf("overlay = %+v", ov)
	}
	if got := ov.Items[0]; got.Detail != items[0].Key || got.Label != i18n.T(items[0].Label) {
		t.Errorf("item = %+v", got)
	}
	if drawMenu(core.MenuOverlay("t", nil), 80) != nil {
		t.Error("empty menu should draw nothing")
	}
	if box := drawMenu(ov, 80); len(box) < 2 {
		t.Errorf("box = %q", box)
	}
}
