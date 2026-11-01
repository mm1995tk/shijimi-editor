package tui

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
	b := a.CurText().Buf
	l := a.layout()
	half := (a.editW(a.CurText(), l) - GutterWidth(b)) / 2

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
	if want := 300 - (a.editW(a.CurText(), l) - GutterWidth(b)); b.ScrollCol != want {
		t.Fatalf("right edge: %d, want %d", b.ScrollCol, want)
	}
}

func TestHScrollWheelAndAltPgDn(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 300)+"\n")
	b := a.CurText().Buf
	l := a.layout()
	a.HandleMouse(a.locate(l.rightX+10, l.editY+1, Mouse{Action: MousePress, Button: MouseWheelRight}))
	if b.ScrollCol != HscrollWheel {
		t.Fatalf("wheel right -> %d", b.ScrollCol)
	}
	a.HandleMouse(a.locate(l.rightX+10, l.editY+1, Mouse{Action: MousePress, Button: MouseWheelDown, Shift: true}))
	if b.ScrollCol != 2*HscrollWheel {
		t.Fatalf("shift+wheel down -> %d", b.ScrollCol)
	}
	a.HandleMouse(a.locate(l.rightX+10, l.editY+1, Mouse{Action: MousePress, Button: MouseWheelLeft}))
	if b.ScrollCol != HscrollWheel || b.ScrollRow != 0 {
		t.Fatalf("wheel left -> col %d row %d", b.ScrollCol, b.ScrollRow)
	}
	a.helix = false
	a.Update(Key{Code: KeyPgDown, Mod: ModAlt})
	if b.ScrollCol <= HscrollWheel {
		t.Fatalf("alt+pgdn -> %d", b.ScrollCol)
	}
}

func TestHScrollZPrefixKeepsView(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 300)+"\n")
	b := a.CurText().Buf
	for range 3 {
		keys(a, "z")
		a.render()
		keys(a, "l")
		a.render()
	}
	if b.ScrollCol <= (a.editW(a.CurText(), a.layout())-GutterWidth(b))/2 {
		t.Fatalf("view did not keep moving: %d", b.ScrollCol)
	}
}

func TestHScrollShiftWheelOutsideTextStaysVertical(t *testing.T) {
	dir := t.TempDir()
	for i := range 60 {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.txt", i)), []byte(strings.Repeat("x", 300)+"\n"), 0o644)
	}
	a := newApp(dir, filepath.Join(dir, "f00.txt"))
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	l := a.layout()
	a.HandleMouse(a.locate(2, l.editY+2, Mouse{Action: MousePress, Button: MouseWheelDown, Shift: true}))
	if a.explorer.scroll == 0 {
		t.Fatalf("Shift+wheel over the explorer did not scroll it")
	}
	if c := a.CurText().Buf.ScrollCol; c != 0 {
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
	a := newApp(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.gd.on = true
	a.gd.base = baseHead
	a.gd.split = true
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	if a.render(); !a.SplitShown() {
		t.Skip("split is not shown")
	}
	tab := a.CurText()
	if _, _, ok := a.caretPos(a.layout()); !ok {
		t.Fatalf("caret hidden before scrolling")
	}
	for range 10 {
		a.ScrollCols(tab, 200)
	}
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "oldold") && !strings.Contains(view, "9old") {
		t.Fatalf("base side did not reach its end:\n%s", view)
	}
	if _, _, ok := a.caretPos(a.layout()); ok {
		t.Fatalf("caret still drawn after the view left it")
	}
}
