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
	if m.Region != RegionEditor || m.Col != 4 || m.Row != l.editY+2-l.top || m.Button != MouseLeft || m.Action != mousePress || !m.Shift {
		t.Errorf("click in the editor: %+v", m)
	}
	m = a.mouseFromTea(tea.MouseWheelMsg{X: 1, Y: l.top + 1, Button: tea.MouseWheelDown})
	if m.Region != RegionExplorer || m.Button != MouseWheelDown {
		t.Errorf("wheel over the tree: %+v", m)
	}
	if m = a.mouseFromTea(tea.MouseReleaseMsg{X: 1, Y: l.top + 1}); m.Action != mouseRelease {
		t.Errorf("release: %+v", m)
	}
}

func TestCompositionCommitsAsText(t *testing.T) {
	a := newTestApp(t, "hello\n")
	a.focus = focusEditor
	a.mode = modeInsert
	a.curText().buf.SetCursor(0, 0, false)
	a.Update(Composition{Text: "にほ", Phase: CompositionUpdate})
	a.Update(Composition{Text: "日本", Phase: CompositionEnd})
	if got := a.curText().buf.Text(); got != "日本hello\n" {
		t.Errorf("text = %q", got)
	}
}
