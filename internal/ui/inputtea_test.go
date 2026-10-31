package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
	if m.Region != RegionEditor || m.Col != 4 || m.Row != l.editY+2-l.top || m.Button != MouseLeft || m.Action != MousePress || !m.Shift {
		t.Errorf("click in the editor: %+v", m)
	}
	m = a.mouseFromTea(tea.MouseWheelMsg{X: 1, Y: l.top + 1, Button: tea.MouseWheelDown})
	if m.Region != RegionExplorer || m.Button != MouseWheelDown {
		t.Errorf("wheel over the tree: %+v", m)
	}
	if m = a.mouseFromTea(tea.MouseReleaseMsg{X: 1, Y: l.top + 1}); m.Action != MouseRelease {
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
		region   Region
		row, col int
	}{
		{"tree", 2, l.top + 3, RegionExplorer, 3, 2},
		{"editor title", l.rightX + 1, l.top, RegionEditor, 0, 1},
		{"editor text", l.rightX + 5, l.editY + 1, RegionEditor, l.editY - l.top + 1, 5},
		{"terminal body", l.fullX + 3, l.panelY + 2, RegionTerminal, 0, 3},
		{"agent body", l.agentX + 4, l.top + 2, RegionAgent, 2, 4},
		{"agent border", l.agentX - 1, l.top + 2, RegionAgent, 2, -1},
		{"separator", l.sideW, l.top + 1, RegionNone, 0, 0},
		{"status line", 3, l.statusY, RegionNone, 0, 0},
	} {
		r, row, col := a.hitTest(l, c.x, c.y)
		if r != c.region || (r != RegionNone && (row != c.row || col != c.col)) {
			t.Errorf("%s: got %v (%d, %d), want %v (%d, %d)", c.name, r, row, col, c.region, c.row, c.col)
		}
	}
}

// A drag keeps the region the press was in, with Row and Col counted from its
// corner, however far the pointer goes.
func TestMouseCaptureKeepsRegion(t *testing.T) {
	a := newTestApp(t, "hello\n")
	l := a.layout()
	press := a.locate(Mouse{X: l.rightX + 2, Y: l.editY, Action: MousePress, Button: MouseLeft})
	if press.Region != RegionEditor {
		t.Fatalf("press: %+v", press)
	}
	drag := a.locate(Mouse{X: 1, Y: l.top + 5, Action: MouseMotion, Button: MouseLeft})
	if drag.Region != RegionEditor || drag.Col != 1-l.rightX {
		t.Errorf("drag over the tree: %+v", drag)
	}
	rel := a.locate(Mouse{X: 1, Y: l.top + 5, Action: MouseRelease, Button: MouseLeft})
	if rel.Region != RegionEditor || a.capture != RegionNone {
		t.Errorf("release: %+v, capture %v", rel, a.capture)
	}
	if m := a.locate(Mouse{X: 1, Y: l.top + 5, Action: MouseMotion, Button: MouseNone}); m.Region != RegionExplorer {
		t.Errorf("motion with no button after the release: %+v", m)
	}
}

func TestHitTestOtherPane(t *testing.T) {
	a, _ := splitApp(t)
	l := a.layout()
	r, row, col := a.hitTest(l, l.otherX+3, l.editY+1)
	if r != RegionOther || col != 3 || row != l.editY-l.top+1 {
		t.Errorf("other pane: %v (%d, %d)", r, row, col)
	}
	// A press there focuses the pane; the position inside it is kept.
	a.Update(a.locate(Mouse{X: l.otherX + 3, Y: l.editY + 1, Action: MousePress, Button: MouseLeft}))
	if a.pane == 1 || a.capture != RegionEditor {
		t.Errorf("pane %d, capture %v", a.pane, a.capture)
	}
}
