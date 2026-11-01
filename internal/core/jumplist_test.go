package core

import (
	"os"
	"path/filepath"
	"testing"
)

func ctrlO(a *App) { a.Update(Key{Code: 'o', Mod: ModCtrl}) }
func ctrlI(a *App) { a.Update(Key{Code: KeyTab}) }

func TestJumpListBackForward(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n4\n5\n")
	b := a.CurText().Buf
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
	for i := range maxJumps + 20 {
		a.CurText().Buf.Cur.Row = i % 3
		a.CurText().Buf.Cur.Col = i % 2
		a.pushJump()
	}
	if len(a.jumps) > maxJumps {
		t.Fatalf("jump list holds %d entries", len(a.jumps))
	}
}

func TestJumpListReopensAndSkipsMissing(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n")
	dir := filepath.Dir(a.CurText().Buf.Path)
	gone := filepath.Join(dir, "gone.txt")
	other := filepath.Join(dir, "other.txt")
	os.WriteFile(gone, []byte("x\n"), 0o644)
	os.WriteFile(other, []byte("a\nb\n"), 0o644)
	first := a.CurText().Buf.Path

	a.OpenFile(gone)
	a.CurText().Buf.Cur.Row = 0
	a.pushJump() // gone.txt
	a.OpenFile(other)
	a.pushJump() // other.txt
	a.OpenFile(first)
	for len(a.Tabs) > 0 {
		a.CloseTab(true)
	}
	os.Remove(gone)
	a.OpenFile(first)
	a.SetFocus(FocusEditor)

	ctrlO(a)
	if got := a.CurText().Buf.Path; got != other {
		t.Fatalf("back -> %s, want reopened %s", got, other)
	}
	ctrlO(a)
	if got := a.CurText().Buf.Path; got == gone {
		t.Fatalf("back opened a deleted file")
	}
}

func TestJumpListNewJumpDropsForward(t *testing.T) {
	a := newTestApp(t, "1\n2\n3\n4\n")
	b := a.CurText().Buf
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
	a.Helix = false
	b := a.CurText().Buf
	a.ExecCommand("3")
	a.Update(Key{Code: KeyLeft, Mod: ModAlt})
	if b.Cur.Row != 0 {
		t.Fatalf("Alt+Left -> row %d, want 0", b.Cur.Row)
	}
	a.Update(Key{Code: KeyRight, Mod: ModAlt})
	if b.Cur.Row != 2 {
		t.Fatalf("Alt+Right -> row %d, want 2", b.Cur.Row)
	}
}
