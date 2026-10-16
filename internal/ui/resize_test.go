package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func resizeApp(t *testing.T) *App {
	a := New(t.TempDir(), "")
	a.w, a.h = 120, 40
	return a
}

func press(a *App, key string) {
	a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func TestResizeKeys(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	side, panel := a.layout().sideW, a.layout().panelH

	a.startResize()
	a.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	a.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	l := a.layout()
	if l.sideW != side+resizeStep || l.panelH != panel+resizeStep {
		t.Fatalf("side %d->%d panel %d->%d", side, l.sideW, panel, l.panelH)
	}

	// keys do not leak to the editor while resizing, and Esc leaves the mode
	a.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.resizeMode {
		t.Fatal("still in resize mode")
	}

	// the size is kept across close and reopen
	a.toggleSide()
	a.toggleSide()
	if a.layout().sideW != side+resizeStep {
		t.Fatalf("size lost: %d", a.layout().sideW)
	}

	a.startResize()
	press(a, "=")
	if l := a.layout(); l.sideW != side || l.panelH != panel {
		t.Fatalf("reset: %d %d", l.sideW, l.panelH)
	}
}

func TestResizeClamp(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	a.startResize()
	for i := 0; i < 200; i++ {
		a.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
		a.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	if l := a.layout(); l.sideW != minSideW || l.panelH != 6 {
		t.Fatalf("min: %d %d", l.sideW, l.panelH)
	}
	for i := 0; i < 200; i++ {
		a.handleKey(tea.KeyMsg{Type: tea.KeyRight})
		a.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	}
	l := a.layout()
	if l.sideW > a.w/2 || l.panelH > a.h-8 {
		t.Fatalf("max: %d %d", l.sideW, l.panelH)
	}
	// shrinking the window pulls the stored size back into range
	a.w, a.h = 40, 12
	l = a.layout()
	if l.rightW < 10 || l.editH < 0 {
		t.Fatalf("small window: %+v", l)
	}
}

func TestResizePreviewRatio(t *testing.T) {
	a := resizeApp(t)
	l := a.layout()
	def := a.editorWidth(l.rightW)
	if def != (l.rightW-1)/2 {
		t.Fatalf("default split changed: %d", def)
	}
	a.startResize()
	press(a, ">")
	if a.editorWidth(l.rightW) <= def {
		t.Fatal("editor should grow")
	}
	for i := 0; i < 50; i++ {
		press(a, "<")
	}
	if w := a.editorWidth(l.rightW); w < minPaneW {
		t.Fatalf("editor too narrow: %d", w)
	}
}

func TestResizeMouseDrag(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	l := a.layout()
	mouse := func(act tea.MouseAction, x, y int) {
		a.handleMouse(tea.MouseMsg{X: x, Y: y, Action: act, Button: tea.MouseButtonLeft})
	}

	mouse(tea.MouseActionPress, l.sideW, 5)
	mouse(tea.MouseActionMotion, 40, 5)
	mouse(tea.MouseActionRelease, 40, 5)
	if a.layout().sideW != 40 || a.resizing != resizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}

	mouse(tea.MouseActionPress, 60, l.panelY)
	mouse(tea.MouseActionMotion, 60, l.panelY-5)
	mouse(tea.MouseActionRelease, 60, l.panelY-5)
	if got := a.layout().panelH; got != l.panelH+5 {
		t.Fatalf("panel drag: %d want %d", got, l.panelH+5)
	}
}
