package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHScrollKeysAndBounds(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 300)+"\nshort\n")
	b := a.curText().buf
	l := a.layout()
	half := (a.editW(a.curText(), l) - gutterWidth(b)) / 2

	keys(a, "z", "l")
	if b.ScrollCol != half || b.Cur.Col != 0 {
		t.Fatalf("zl -> ScrollCol %d (want %d), cursor col %d", b.ScrollCol, half, b.Cur.Col)
	}
	a.render()
	if b.ScrollCol != half {
		t.Fatalf("render snapped the view back to %d", b.ScrollCol)
	}
	keys(a, "z", "h", "z", "h")
	if b.ScrollCol != 0 {
		t.Fatalf("past the left edge: %d", b.ScrollCol)
	}
	for i := 0; i < 20; i++ {
		keys(a, "z", "l")
	}
	if want := 300 - (a.editW(a.curText(), l) - gutterWidth(b)); b.ScrollCol != want {
		t.Fatalf("right edge: %d, want %d", b.ScrollCol, want)
	}
}

func TestHScrollWheelAndAltPgDn(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 300)+"\n")
	b := a.curText().buf
	l := a.layout()
	a.handleMouse(mouseEvent{X: l.rightX + 10, Y: l.editY + 1, Action: mousePress, Button: tea.MouseWheelRight})
	if b.ScrollCol != hscrollWheel {
		t.Fatalf("wheel right -> %d", b.ScrollCol)
	}
	a.handleMouse(mouseEvent{X: l.rightX + 10, Y: l.editY + 1, Action: mousePress, Button: tea.MouseWheelDown, Shift: true})
	if b.ScrollCol != 2*hscrollWheel {
		t.Fatalf("shift+wheel down -> %d", b.ScrollCol)
	}
	a.handleMouse(mouseEvent{X: l.rightX + 10, Y: l.editY + 1, Action: mousePress, Button: tea.MouseWheelLeft})
	if b.ScrollCol != hscrollWheel || b.ScrollRow != 0 {
		t.Fatalf("wheel left -> col %d row %d", b.ScrollCol, b.ScrollRow)
	}
	a.helix = false
	a.Update(tea.KeyPressMsg{Code: tea.KeyPgDown, Mod: tea.ModAlt})
	if b.ScrollCol <= hscrollWheel {
		t.Fatalf("alt+pgdn -> %d", b.ScrollCol)
	}
}

func TestHScrollZPrefixKeepsView(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 300)+"\n")
	b := a.curText().buf
	for i := 0; i < 3; i++ {
		keys(a, "z")
		a.render()
		keys(a, "l")
		a.render()
	}
	if b.ScrollCol <= (a.editW(a.curText(), a.layout())-gutterWidth(b))/2 {
		t.Fatalf("view did not keep moving: %d", b.ScrollCol)
	}
}
