package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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
	for range 20 {
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
	a.handleMouse(Mouse{X: l.rightX + 10, Y: l.editY + 1, Action: MousePress, Button: MouseWheelRight})
	if b.ScrollCol != hscrollWheel {
		t.Fatalf("wheel right -> %d", b.ScrollCol)
	}
	a.handleMouse(Mouse{X: l.rightX + 10, Y: l.editY + 1, Action: MousePress, Button: MouseWheelDown, Shift: true})
	if b.ScrollCol != 2*hscrollWheel {
		t.Fatalf("shift+wheel down -> %d", b.ScrollCol)
	}
	a.handleMouse(Mouse{X: l.rightX + 10, Y: l.editY + 1, Action: MousePress, Button: MouseWheelLeft})
	if b.ScrollCol != hscrollWheel || b.ScrollRow != 0 {
		t.Fatalf("wheel left -> col %d row %d", b.ScrollCol, b.ScrollRow)
	}
	a.helix = false
	a.Update(Key{Code: KeyPgDown, Mod: ModAlt})
	if b.ScrollCol <= hscrollWheel {
		t.Fatalf("alt+pgdn -> %d", b.ScrollCol)
	}
}

func TestHScrollZPrefixKeepsView(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 300)+"\n")
	b := a.curText().buf
	for range 3 {
		keys(a, "z")
		a.render()
		keys(a, "l")
		a.render()
	}
	if b.ScrollCol <= (a.editW(a.curText(), a.layout())-gutterWidth(b))/2 {
		t.Fatalf("view did not keep moving: %d", b.ScrollCol)
	}
}

func TestHScrollShiftWheelOutsideTextStaysVertical(t *testing.T) {
	dir := t.TempDir()
	for i := range 60 {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.txt", i)), []byte(strings.Repeat("x", 300)+"\n"), 0o644)
	}
	a := New(dir, filepath.Join(dir, "f00.txt"))
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	l := a.layout()
	a.handleMouse(Mouse{X: 2, Y: l.editY + 2, Action: MousePress, Button: MouseWheelDown, Shift: true})
	if a.explorer.scroll == 0 {
		t.Fatalf("Shift+wheel over the explorer did not scroll it")
	}
	if c := a.curText().buf.ScrollCol; c != 0 {
		t.Fatalf("the editor scrolled sideways: %d", c)
	}
}

func TestHScrollSplitDiffMovesBaseSideAndHidesCaret(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	file := filepath.Join(dir, "a.txt")
	long := func(s string) string { return strings.Repeat("0123456789", 20) + s + "\n" }
	os.WriteFile(file, []byte(long("old")), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	os.WriteFile(file, []byte(long("new")), 0o644)
	a := New(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.gd.on = true
	a.gd.base = baseHead
	a.gd.split = true
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	if a.render(); !a.splitShown() {
		t.Skip("split is not shown")
	}
	tab := a.curText()
	if _, _, ok := a.caretPos(a.layout()); !ok {
		t.Fatalf("caret hidden before scrolling")
	}
	for range 10 {
		a.scrollCols(tab, 200)
	}
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "oldold") && !strings.Contains(view, "9old") {
		t.Fatalf("base side did not reach its end:\n%s", view)
	}
	if _, _, ok := a.caretPos(a.layout()); ok {
		t.Fatalf("caret still drawn after the view left it")
	}
}
