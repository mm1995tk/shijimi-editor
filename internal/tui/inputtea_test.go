package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func TestKeyRoundTripsThroughTea(t *testing.T) {
	for _, m := range []tea.KeyPressMsg{
		{Code: 'a', Text: "a"},
		{Code: 's', Mod: tea.ModCtrl},
		{Code: tea.KeyEnter},
		{Code: tea.KeyTab, Mod: tea.ModShift},
		{Code: 'd', Mod: tea.ModCtrl | tea.ModAlt, IsRepeat: true},
		{Code: tea.KeyF5},
		{Code: 'a', ShiftedCode: 'A', Text: "A", Mod: tea.ModShift},
	} {
		k := keyFromTea(m)
		if got := keyToTea(k); got != m {
			t.Errorf("round trip of %v gave %v", m, got)
		}
		if k.String() != m.String() {
			t.Errorf("Key.String() = %q, want %q", k.String(), m.String())
		}
	}
}

func TestMouseFromTea(t *testing.T) {
	a := newTestApp(t, "hello\n")
	l := a.layout()
	m := a.mouseFromTea(tea.MouseClickMsg{X: l.rightX + 4, Y: l.editY + 2, Button: tea.MouseLeft, Mod: tea.ModShift})
	if m.Region != core.RegionEditor || m.Col != 4 || m.Row != l.editY+2-l.top || m.Button != core.MouseLeft || m.Action != core.MousePress || !m.Shift {
		t.Errorf("click in the editor: %+v", m)
	}
	m = a.mouseFromTea(tea.MouseWheelMsg{X: 1, Y: l.top + 1, Button: tea.MouseWheelDown})
	if m.Region != core.RegionExplorer || m.Button != core.MouseWheelDown {
		t.Errorf("wheel over the tree: %+v", m)
	}
	if m = a.mouseFromTea(tea.MouseReleaseMsg{X: 1, Y: l.top + 1}); m.Action != core.MouseRelease {
		t.Errorf("release: %+v", m)
	}
}

func TestHitTestRegions(t *testing.T) {
	a := newTestApp(t, "hello\n")
	a.showTerm = true
	a.showAgent = true
	l := a.layout()
	for _, c := range []struct {
		name     string
		x, y     int
		region   core.Region
		row, col int
	}{
		{"tree", 2, l.top + 3, core.RegionExplorer, 3, 2},
		{"editor title", l.rightX + 1, l.top, core.RegionEditor, 0, 1},
		{"editor text", l.rightX + 5, l.editY + 1, core.RegionEditor, l.editY - l.top + 1, 5},
		{"terminal body", l.fullX + 3, l.panelY + 2, core.RegionTerminal, 0, 3},
		{"agent body", l.agentX + 4, l.top + 2, core.RegionAgent, 2, 4},
		{"agent border", l.agentX - 1, l.top + 2, core.RegionAgent, 2, -1},
		{"separator", l.sideW, l.top + 1, core.RegionNone, 0, 0},
		{"status line", 3, l.statusY, core.RegionNone, 0, 0},
	} {
		r, row, col := a.hitTest(l, c.x, c.y)
		if r != c.region || (r != core.RegionNone && (row != c.row || col != c.col)) {
			t.Errorf("%s: got %v (%d, %d), want %v (%d, %d)", c.name, r, row, col, c.region, c.row, c.col)
		}
	}
}

// A drag keeps the region the press was in, with Row and Col counted from its
// corner, however far the pointer goes.
func TestMouseCaptureKeepsRegion(t *testing.T) {
	a := newTestApp(t, "hello\n")
	l := a.layout()
	press := a.locate(l.rightX+2, l.editY, core.Mouse{Action: core.MousePress, Button: core.MouseLeft})
	if press.Region != core.RegionEditor {
		t.Fatalf("press: %+v", press)
	}
	drag := a.locate(1, l.top+5, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft})
	if drag.Region != core.RegionEditor || drag.Col != 1-l.rightX {
		t.Errorf("drag over the tree: %+v", drag)
	}
	rel := a.locate(1, l.top+5, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft})
	if rel.Region != core.RegionEditor || a.capture != core.RegionNone {
		t.Errorf("release: %+v, capture %v", rel, a.capture)
	}
	if m := a.locate(1, l.top+5, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}); m.Region != core.RegionExplorer {
		t.Errorf("motion with no button after the release: %+v", m)
	}
}

func TestHitTestOtherPane(t *testing.T) {
	a, _ := splitApp(t)
	l := a.layout()
	r, row, col := a.hitTest(l, l.otherX+3, l.editY+1)
	if r != core.RegionOther || col != 3 || row != l.editY-l.top+1 {
		t.Errorf("other pane: %v (%d, %d)", r, row, col)
	}
	// A press there focuses the pane; the position inside it is kept.
	a.Update(a.locate(l.otherX+3, l.editY+1, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Pane == 1 || a.capture != core.RegionEditor {
		t.Errorf("pane %d, capture %v", a.Pane, a.capture)
	}
}

func TestClicksCountedByTheAdapter(t *testing.T) {
	a := newTestApp(t, "x\n")
	if n := a.clicks(3, 4); n != 1 {
		t.Fatalf("first press = %d", n)
	}
	if n := a.clicks(3, 4); n != 2 {
		t.Fatalf("second press = %d", n)
	}
	if n := a.clicks(3, 4); n != 1 {
		t.Fatalf("third press starts over, got %d", n)
	}
	if n := a.clicks(5, 4); n != 1 {
		t.Fatalf("another cell = %d", n)
	}
}

// screenCell undoes locate for every cell, in every region, so the core can
// work without the screen cell in a Mouse.
func TestScreenCellInvertsLocate(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	l := a.layout()
	for y := range a.h {
		for x := range a.w {
			m := a.locate(x, y, core.Mouse{Action: core.MouseMotion})
			if gx, gy := a.screenCell(l, m); gx != x || gy != y {
				t.Fatalf("(%d,%d) -> %+v -> (%d,%d)", x, y, m, gx, gy)
			}
		}
	}
}
