package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCaretFollowsTyping(t *testing.T) {
	a := newTestApp(t, "日本語\n")
	l := a.layout()

	keys(a, "l", "l")
	want, wantY := a.cursorScreen(a.curText(), l)
	if x, y, ok := a.caretPos(l); !ok || x != want || y != wantY {
		t.Errorf("editor caret = (%d,%d,%v), want (%d,%d)", x, y, ok, want, wantY)
	}

	keys(a, ":", "wq")
	if x, y, ok := a.caretPos(l); !ok || x != 3 || y != a.h-1 {
		t.Errorf("cmdline caret = (%d,%d,%v), want (3,%d)", x, y, ok, a.h-1)
	}
	keys(a, "esc")

	a.focus = focusExplorer
	if _, _, ok := a.caretPos(l); ok {
		t.Error("explorer focused: want no caret")
	}
}

// The caret of each search field is right after its text on screen, counting
// wide characters as two cells.
func TestCaretInSearchFields(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.openSearch(fQuery)
	for f, text := range [numFields]string{"日本語", "置換後", "含める", "除外す"} {
		a.search.focusField(f)
		a.search.in[f].SetValue(text)
		a.search.in[f].CursorEnd()
		rows := strings.Split(a.render(), "\n")
		x, y, ok := a.caretPos(a.layout())
		if !ok || y >= len(rows) {
			t.Fatalf("field %d: caret = (%d,%d,%v)", f, x, y, ok)
		}
		row := ansi.Strip(rows[y])
		i := strings.Index(row, text)
		if i < 0 {
			t.Fatalf("field %d: row %d has no text: %q", f, y, row)
		}
		if want := ansi.StringWidth(row[:i]) + 6; x != want {
			t.Errorf("field %d: caret x = %d, want %d", f, x, want)
		}
	}
}

// The palette and the find bar put the caret right after their text too.
func TestCaretInPaletteAndFind(t *testing.T) {
	a := newTestApp(t, "x\n")
	check := func(name, text string) {
		t.Helper()
		rows := strings.Split(a.render(), "\n")
		x, y, ok := a.caretPos(a.layout())
		if !ok || y >= len(rows) {
			t.Fatalf("%s: caret = (%d,%d,%v)", name, x, y, ok)
		}
		row := ansi.Strip(rows[y])
		i := strings.Index(row, text)
		if i < 0 {
			t.Fatalf("%s: row %d has no text: %q", name, y, row)
		}
		if want := ansi.StringWidth(row[:i]) + ansi.StringWidth(text); x != want {
			t.Errorf("%s: caret x = %d, want %d", name, x, want)
		}
	}

	a.pal.show(a.root, "")
	a.pal.input.SetValue("日本語")
	a.pal.input.CursorEnd()
	check("palette", "日本語")
	a.pal.open = false

	a.openFind()
	a.findInput.SetValue("日本語")
	a.findInput.CursorEnd()
	check("find", "日本語")
}

// After text longer than the field scrolls it, moving left keeps the scroll,
// and repeated characters must not make the caret drift to the right.
func TestInputCaretScrolledRepeats(t *testing.T) {
	for _, c := range []struct {
		value       string
		width, left int
		want        int
	}{
		{strings.Repeat("a", 20), 11, 1, 10},
		{"ababab", 4, 2, 2},
		{"あいうえおかきく", 6, 1, 4},
		{"日本語", 20, 0, 6},
		{"", 10, 0, 0},
	} {
		ti := newLineInput(0)
		ti.SetWidth(c.width)
		ti.Focus()
		for _, r := range c.value {
			ti.Update(textKey(string(r)))
		}
		for range c.left {
			ti.Update(Key{Code: KeyLeft})
		}
		if got := ti.Caret(); got != c.want {
			t.Errorf("%q width %d left %d: caret = %d, want %d", c.value, c.width, c.left, got, c.want)
		}
	}
}
