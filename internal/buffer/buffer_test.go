package buffer

import (
	"os"
	"path/filepath"
	"testing"
)

func fromText(s string) *Buffer {
	b := New()
	b.Insert(s)
	b.Cur = Pos{}
	b.undo = nil
	b.Dirty = false
	return b
}

func TestInsertAndUndo(t *testing.T) {
	b := fromText("hello\nworld")
	b.Cur = Pos{0, 5}
	for _, r := range " there" {
		b.Insert(string(r))
	}
	if got := b.Text(); got != "hello there\nworld" {
		t.Fatalf("text = %q", got)
	}
	b.Undo() // typing is grouped into one step
	if got := b.Text(); got != "hello\nworld" {
		t.Fatalf("after undo = %q", got)
	}
	b.Redo()
	if got := b.Text(); got != "hello there\nworld" {
		t.Fatalf("after redo = %q", got)
	}
}

func TestMultilineInsertAndDeleteSelection(t *testing.T) {
	b := fromText("abc")
	b.Cur = Pos{0, 1}
	b.Insert("X\nY\nZ")
	if got := b.Text(); got != "aX\nY\nZbc" {
		t.Fatalf("text = %q", got)
	}
	if b.Cur != (Pos{2, 1}) {
		t.Fatalf("cursor = %+v", b.Cur)
	}
	b.SelectRange(Pos{0, 1}, Pos{2, 1})
	if got := b.SelectedText(); got != "X\nY\nZ" {
		t.Fatalf("selected = %q", got)
	}
	b.Backspace()
	if got := b.Text(); got != "abc" {
		t.Fatalf("after delete = %q", got)
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	b := fromText("ab\ncd")
	b.Cur = Pos{1, 0}
	b.Backspace()
	if got := b.Text(); got != "abcd" || b.Cur != (Pos{0, 2}) {
		t.Fatalf("text = %q cur = %+v", got, b.Cur)
	}
	b.Cur = Pos{0, 4}
	b.Delete() // at end of buffer: no-op
	if b.Text() != "abcd" {
		t.Fatal("delete at end changed text")
	}
}

func TestNewlineKeepsIndent(t *testing.T) {
	b := fromText("    if x {")
	b.MoveCursor(End, false)
	b.Newline()
	if got := b.Text(); got != "    if x {\n        " {
		t.Fatalf("text = %q", got)
	}
}

func TestToggleComment(t *testing.T) {
	b := fromText("  a\n  b")
	b.SelectAll()
	b.ToggleComment("//")
	if got := b.Text(); got != "  // a\n  // b" {
		t.Fatalf("commented = %q", got)
	}
	b.SelectAll()
	b.ToggleComment("//")
	if got := b.Text(); got != "  a\n  b" {
		t.Fatalf("uncommented = %q", got)
	}
}

func TestFindWraps(t *testing.T) {
	b := fromText("foo\nbar foo\nbaz")
	p, ok := b.Find("FOO", Pos{0, 1}, false)
	if !ok || p != (Pos{1, 4}) {
		t.Fatalf("find = %+v %v", p, ok)
	}
	p, ok = b.Find("foo", Pos{2, 0}, false)
	if !ok || p != (Pos{0, 0}) {
		t.Fatalf("wrap = %+v %v", p, ok)
	}
	p, ok = b.Find("foo", Pos{1, 4}, true)
	if !ok || p != (Pos{0, 0}) {
		t.Fatalf("backward = %+v %v", p, ok)
	}
}

func TestVisualColumns(t *testing.T) {
	line := []rune("\tあa")
	if v := VisualCol(line, 2); v != 6 {
		t.Fatalf("visual col = %d", v)
	}
	if c := ColFromVisual(line, 6); c != 2 {
		t.Fatalf("col from visual = %d", c)
	}
}

func TestSaveRoundTripCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("a\r\nb\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Text() != "a\nb\n" {
		t.Fatalf("text = %q", b.Text())
	}
	b.Cur = Pos{1, 1}
	b.Insert("!")
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "a\r\nb!\r\n" {
		t.Fatalf("saved = %q", data)
	}
}
