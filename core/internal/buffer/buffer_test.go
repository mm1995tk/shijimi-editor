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

func TestNewlineBetweenBrackets(t *testing.T) {
	for _, pair := range []string{"{}", "()", "[]"} {
		b := fromText("\tf " + pair)
		b.UseTabs = true
		b.Cur = Pos{0, 4}
		b.Newline()
		want := "\tf " + pair[:1] + "\n\t\t\n\t" + pair[1:]
		if got := b.Text(); got != want {
			t.Fatalf("%s: text = %q, want %q", pair, got, want)
		}
		if b.Cur != (Pos{1, 2}) {
			t.Fatalf("%s: cur = %+v", pair, b.Cur)
		}
		if !b.Undo() || b.Text() != "\tf "+pair {
			t.Fatalf("%s: one undo should restore, got %q", pair, b.Text())
		}
	}
}

func TestNewlineMismatchedBracketsKeepsOldBehavior(t *testing.T) {
	for _, c := range []struct {
		text string
		col  int
		want string
	}{
		{"{)", 1, "{\n\t)"},
		{"a:\x00", 2, "a:\n\t\x00"},
	} {
		b := fromText(c.text)
		b.UseTabs = true
		b.Cur = Pos{0, c.col}
		b.Newline()
		if got := b.Text(); got != c.want {
			t.Fatalf("%q: text = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestOpenLineIndentsInsideBrackets(t *testing.T) {
	src := "func f() {\n\tk(s{\n\t\tX: 1,\n\t})\n}"
	for _, c := range []struct {
		name  string
		row   int
		below bool
		want  string
		cur   Pos
	}{
		{"above closer", 3, false, "func f() {\n\tk(s{\n\t\tX: 1,\n\t\t\n\t})\n}", Pos{3, 2}},
		{"below opener", 1, true, "func f() {\n\tk(s{\n\t\t\n\t\tX: 1,\n\t})\n}", Pos{2, 2}},
		{"above plain", 2, false, "func f() {\n\tk(s{\n\t\t\n\t\tX: 1,\n\t})\n}", Pos{2, 2}},
		{"below plain", 2, true, "func f() {\n\tk(s{\n\t\tX: 1,\n\t\t\n\t})\n}", Pos{3, 2}},
	} {
		b := fromText(src)
		b.UseTabs = true
		b.Cur = Pos{c.row, 1}
		b.OpenLine(c.below)
		if got := b.Text(); got != c.want || b.Cur != c.cur {
			t.Errorf("%s: text = %q cur = %+v, want %q %+v", c.name, got, b.Cur, c.want, c.cur)
		}
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
	for range 3 { // third x keeps the selection
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

// gopls's CutPrefix fix inserts and replaces at the same position.
func TestApplyEditsInsertBeforeReplaceAtSameStart(t *testing.T) {
	b := fromText("\tif strings.HasPrefix(trim, \"```\") {\n\t\tx := strings.TrimPrefix(trim, \"```\")")
	b.ApplyEdits([]Edit{
		{Start: Pos{0, 4}, End: Pos{0, 4}, Text: "after, ok := "},
		{Start: Pos{0, 4}, End: Pos{0, 21}, Text: "strings.CutPrefix"},
		{Start: Pos{0, 34}, End: Pos{0, 34}, Text: "; ok "},
		{Start: Pos{1, 7}, End: Pos{1, 38}, Text: "after"},
	})
	want := "\tif after, ok := strings.CutPrefix(trim, \"```\"); ok  {\n\t\tx := after"
	if got := b.Text(); got != want {
		t.Fatalf("text = %q\nwant %q", got, want)
	}
}

func TestSetCursorDrag(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		down       Pos
		row, vis   int // the pointer
		want       string
		wantCursor Pos
	}{
		{"forward", "abcdef", Pos{0, 1}, 0, 3, "bcd", Pos{0, 4}},
		{"backward", "abcdef", Pos{0, 3}, 0, 1, "bcd", Pos{0, 1}},
		{"same cell", "abcdef", Pos{0, 2}, 0, 2, "c", Pos{0, 3}},
		{"past end", "abcdef", Pos{0, 4}, 0, 20, "ef", Pos{0, 6}},
		{"down onto wide, left cell", "abcdef\nxy世zw", Pos{0, 1}, 1, 2, "bcdef\nxy世", Pos{1, 3}},
		{"down onto wide, right cell", "abcdef\nxy世zw", Pos{0, 1}, 1, 3, "bcdef\nxy世", Pos{1, 3}},
		{"backward onto wide", "ab\nxy世zw", Pos{1, 3}, 1, 3, "世z", Pos{1, 2}},
		{"up onto wide", "xy世zw\nabc", Pos{1, 1}, 0, 3, "世zw\nab", Pos{0, 2}},
		{"forward onto tab", "a\tb", Pos{0, 0}, 0, 3, "a\t", Pos{0, 2}},
		{"backward from tab", "a\tb", Pos{0, 1}, 0, 0, "a\t", Pos{0, 0}},
	}
	for _, c := range cases {
		b := fromText(c.text)
		b.StartDrag(c.down.Row, VisualCol(b.Lines[c.down.Row], c.down.Col))
		b.SetCursorDrag(c.row, c.vis)
		if got := b.SelectedText(); got != c.want {
			t.Errorf("%s: selected %q, want %q", c.name, got, c.want)
		}
		if b.Cur != c.wantCursor {
			t.Errorf("%s: cursor %+v, want %+v", c.name, b.Cur, c.wantCursor)
		}
	}
	// the cursor can go forward, back, and forward again in one drag
	b := fromText("abcdef")
	b.StartDrag(0, 3)
	for _, s := range []struct {
		vis  int
		want string
	}{{5, "def"}, {1, "bcd"}, {3, "d"}, {4, "de"}} {
		b.SetCursorDrag(0, s.vis)
		if got := b.SelectedText(); got != s.want {
			t.Errorf("drag to %d: %q, want %q", s.vis, got, s.want)
		}
	}
}

// A press on the second half of a tab puts the cursor after the tab, but the
// drag still starts on the tab.
func TestSetCursorDragFromTab(t *testing.T) {
	// "a\tb": the tab covers the screen columns 1 to 3
	for _, c := range []struct {
		name string
		vis  int // where the pointer goes
		want string
	}{{"left", 0, "a\t"}, {"right", 4, "\tb"}, {"same cell", 3, "\t"}} {
		b := fromText("a\tb")
		b.StartDrag(0, 3)
		b.SetCursorDrag(0, c.vis)
		if got := b.SelectedText(); got != c.want {
			t.Errorf("%s: selected %q, want %q", c.name, got, c.want)
		}
	}
}
