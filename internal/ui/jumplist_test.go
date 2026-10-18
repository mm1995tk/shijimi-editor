package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func ctrlO(a *App) { a.Update(tea.KeyMsg{Type: tea.KeyCtrlO}) }
func ctrlI(a *App) { a.Update(tea.KeyMsg{Type: tea.KeyTab}) }

func TestJumpListBackForward(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n4\n5\n")
	b := a.curText().buf
	keys(a, "3", "g") // row 2
	keys(a, "g", "e") // last line
	end := b.Cur.Row
	ctrlO(a)
	if b.Cur.Row != 2 {
		t.Fatalf("back -> row %d, want 2", b.Cur.Row)
	}
	ctrlO(a)
	if b.Cur.Row != 0 {
		t.Fatalf("back -> row %d, want 0", b.Cur.Row)
	}
	ctrlO(a)
	if b.Cur.Row != 0 {
		t.Fatalf("back at the oldest moved to row %d", b.Cur.Row)
	}
	ctrlI(a)
	if b.Cur.Row != 2 {
		t.Fatalf("forward -> row %d, want 2", b.Cur.Row)
	}
	ctrlI(a)
	if b.Cur.Row != end {
		t.Fatalf("forward -> row %d, want %d", b.Cur.Row, end)
	}
}

func TestJumpListSkipsDuplicatesAndCaps(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n")
	keys(a, "g", "g", "g", "g")
	if len(a.jumps) != 1 {
		t.Fatalf("same position pushed twice: %d entries", len(a.jumps))
	}
	for i := 0; i < maxJumps+20; i++ {
		a.curText().buf.Cur.Row = i % 3
		a.curText().buf.Cur.Col = i % 2
		a.pushJump()
	}
	if len(a.jumps) > maxJumps {
		t.Fatalf("jump list holds %d entries", len(a.jumps))
	}
}

func TestJumpListReopensAndSkipsMissing(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n")
	dir := filepath.Dir(a.curText().buf.Path)
	gone := filepath.Join(dir, "gone.txt")
	other := filepath.Join(dir, "other.txt")
	os.WriteFile(gone, []byte("x\n"), 0o644)
	os.WriteFile(other, []byte("a\nb\n"), 0o644)
	first := a.curText().buf.Path

	a.openFile(gone)
	a.curText().buf.Cur.Row = 0
	a.pushJump() // gone.txt
	a.openFile(other)
	a.pushJump() // other.txt
	a.openFile(first)
	for len(a.tabs) > 0 {
		a.closeTab(true)
	}
	os.Remove(gone)
	a.openFile(first)
	a.setFocus(focusEditor)

	ctrlO(a)
	if got := a.curText().buf.Path; got != other {
		t.Fatalf("back -> %s, want reopened %s", got, other)
	}
	ctrlO(a)
	if got := a.curText().buf.Path; got == gone {
		t.Fatalf("back opened a deleted file")
	}
}

func TestJumpListNewJumpDropsForward(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n4\n")
	b := a.curText().buf
	keys(a, "3", "g")
	ctrlO(a)
	keys(a, "4", "g")
	ctrlI(a)
	if b.Cur.Row != 3 {
		t.Fatalf("forward after a new jump moved to row %d", b.Cur.Row)
	}
}

func TestJumpListAltArrowsInVSCodeMode(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n")
	a.helix = false
	b := a.curText().buf
	a.execCommand("3")
	a.Update(tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	if b.Cur.Row != 0 {
		t.Fatalf("Alt+Left -> row %d, want 0", b.Cur.Row)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRight, Alt: true})
	if b.Cur.Row != 2 {
		t.Fatalf("Alt+Right -> row %d, want 2", b.Cur.Row)
	}
}
