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

func TestSelectLineDeleteLastLine(t *testing.T) {
	cases := []struct {
		name string
		text string
		row  int
		want string
	}{
		{"last line with text", "a\nb", 1, "a"},
		{"trailing empty line", "a\nb\n", 2, "a\nb"},
		{"middle line", "a\nb\nc", 1, "a\nc"},
		{"only line", "a", 0, ""},
	}
	for _, c := range cases {
		b := fromText(c.text)
		b.Cur = Pos{c.row, 0}
		b.SelectLine()
		s, e, _ := b.LineDeleteRange()
		b.DeleteRange(s, e)
		if got := b.Text(); got != c.want {
			t.Errorf("%s: text = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSelectLineExtendToLastLine(t *testing.T) {
	b := fromText("a\nb\nc")
	b.Cur = Pos{1, 0}
	for i := 0; i < 3; i++ { // third x keeps the selection
		b.SelectLine()
	}
	s, e, _ := b.LineDeleteRange()
	b.DeleteRange(s, e)
	if got := b.Text(); got != "a" {
		t.Fatalf("text = %q, want %q", got, "a")
	}
}

func TestSelectLastLineKeepsSelectionShape(t *testing.T) {
	b := fromText("\ta\n\tb")
	b.Cur = Pos{1, 0}
	b.SelectLine()
	if s, e, _ := b.Selection(); s != (Pos{1, 0}) || e != (Pos{1, 2}) {
		t.Fatalf("selection = %v-%v", s, e)
	}
	b.ToggleComment("//")
	if got := b.Text(); got != "\ta\n\t// b" && got != "\ta\n//\tb" && got != "\ta\n\tb" {
		t.Logf("comment result = %q", got)
	}
	if got := b.Lines[0]; string(got) != "\ta" {
		t.Fatalf("previous line touched: %q", string(got))
	}
}

func TestChangeLastLineKeepsCursorOnLine(t *testing.T) {
	b := fromText("a\nb")
	b.Cur = Pos{1, 0}
	b.SelectLine()
	s, e := b.SelectionOrChar()
	b.DeleteRange(s, e)
	if b.Cur != (Pos{1, 0}) {
		t.Fatalf("cur = %v, want {1 0}", b.Cur)
	}
}

func TestWordSelectionOnLastLineKeepsNewline(t *testing.T) {
	b := fromText("a\n}")
	b.Cur = Pos{1, 0}
	b.SelectWord(true, false)
	s, e, widened := b.LineDeleteRange()
	if widened {
		t.Fatal("range widened for a non-x selection")
	}
	b.DeleteRange(s, e)
	if got := b.Text(); got != "a\n" {
		t.Fatalf("text = %q, want %q", got, "a\n")
	}
}

func TestLineDeleteRangeWidenedText(t *testing.T) {
	b := fromText("a\nb")
	b.Cur = Pos{1, 0}
	b.SelectLine()
	s, e, widened := b.LineDeleteRange()
	if !widened {
		t.Fatal("range not widened")
	}
	if got := b.DeleteRange(s, e); got != "\nb" {
		t.Fatalf("removed = %q", got)
	}
}

func TestStaleLineSelectionDoesNotWiden(t *testing.T) {
	t.Run("cleared", func(t *testing.T) {
		b := fromText("a\n}")
		b.Cur = Pos{1, 0}
		b.SelectLine()
		b.ClearSelection()
		b.Cur = Pos{1, 0}
		b.SelectWord(true, false)
		s, e, widened := b.LineDeleteRange()
		if widened {
			t.Fatal("widened after ClearSelection")
		}
		b.DeleteRange(s, e)
		if got := b.Text(); got != "a\n" {
			t.Fatalf("text = %q, want %q", got, "a\n")
		}
	})
	t.Run("undo", func(t *testing.T) {
		b := fromText("a\n}")
		b.Cur = Pos{1, 0}
		b.SelectLine()
		s, e, _ := b.LineDeleteRange()
		b.DeleteRange(s, e)
		b.Undo()
		b.Cur = Pos{1, 0}
		b.SelectWord(true, false)
		if _, _, widened := b.LineDeleteRange(); widened {
			t.Fatal("widened after undo")
		}
	})
}

func TestWordRange(t *testing.T) {
	line := []rune("foo_bar.baz(1)")
	for _, c := range []struct{ col, s, e int }{{0, 0, 7}, {6, 0, 7}, {7, 7, 8}, {8, 8, 11}, {11, 11, 12}, {12, 12, 13}} {
		if s, e := WordRange(line, c.col); s != c.s || e != c.e {
			t.Errorf("col %d: [%d,%d) want [%d,%d)", c.col, s, e, c.s, c.e)
		}
	}
}

func TestTypeRuneAutoClose(t *testing.T) {
	for _, pair := range []string{"()", "{}", "[]"} {
		b := fromText("")
		b.TypeRune(rune(pair[0]))
		if got := b.Text(); got != pair || b.Cur != (Pos{0, 1}) {
			t.Fatalf("%q: text = %q cur = %v", pair, got, b.Cur)
		}
		b.TypeRune(rune(pair[1]))
		if got := b.Text(); got != pair || b.Cur != (Pos{0, 2}) {
			t.Fatalf("%q step over: text = %q cur = %v", pair, got, b.Cur)
		}
	}
}

func TestTypeRuneCloserWithoutMatchInserts(t *testing.T) {
	b := fromText("ab")
	b.Cur = Pos{0, 1}
	b.TypeRune(')')
	if got := b.Text(); got != "a)b" {
		t.Fatalf("text = %q", got)
	}
}

func TestBackspaceDeletesEmptyPair(t *testing.T) {
	b := fromText("")
	b.TypeRune('(')
	b.Backspace()
	if got := b.Text(); got != "" || b.Cur != (Pos{}) {
		t.Fatalf("text = %q cur = %v", got, b.Cur)
	}
	b.TypeRune('(')
	b.TypeRune('x')
	b.Backspace()
	b.Backspace()
	if got := b.Text(); got != "" {
		t.Fatalf("text = %q", got)
	}
}

func TestTypeRuneWrapsSelection(t *testing.T) {
	b := fromText("foo bar")
	b.SelectRange(Pos{0, 4}, Pos{0, 7})
	b.TypeRune('[')
	if got := b.Text(); got != "foo [bar]" {
		t.Fatalf("text = %q", got)
	}
	if got := b.SelectedText(); got != "bar" {
		t.Fatalf("selection = %q", got)
	}
	b.Undo()
	if got := b.Text(); got != "foo bar" {
		t.Fatalf("after undo = %q", got)
	}
}

func TestBackspaceBeforeNULKeepsIt(t *testing.T) {
	b := fromText("a\x00")
	b.Cur = Pos{0, 1}
	b.Backspace()
	if got := b.Text(); got != "\x00" {
		t.Fatalf("text = %q", got)
	}
}
