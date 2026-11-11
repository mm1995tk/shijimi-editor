package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
)

func TestCaretFollowsTyping(t *testing.T) {
	a := newTestApp(t, "日本語\n")
	l := a.layout()

	keys(a, "l", "l")
	want, wantY := a.cursorScreen(a.CurText(), l)
	if x, y, ok := a.caretPos(l); !ok || x != want || y != wantY {
		t.Errorf("editor caret = (%d,%d,%v), want (%d,%d)", x, y, ok, want, wantY)
	}

	keys(a, ":", "wq")
	if x, y, ok := a.caretPos(l); !ok || x != 3 || y != a.h-1 {
		t.Errorf("cmdline caret = (%d,%d,%v), want (3,%d)", x, y, ok, a.h-1)
	}
	keys(a, "esc")

	a.Focus = core.FocusExplorer
	if _, _, ok := a.caretPos(l); ok {
		t.Error("explorer focused: want no caret")
	}
}

// The caret of each search field is right after its text on screen, counting
// wide characters as two cells.
func TestCaretInSearchFields(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.OpenSearch(core.FQuery)
	for f, text := range [core.NumFields]string{"日本語", "置換後", "含める", "除外す"} {
		a.Search.FocusField(f)
		a.Search.In[f].SetValue(text)
		a.Search.In[f].CursorEnd()
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

	a.Pal.Show(a.Root, "")
	a.Pal.Input.SetValue("日本語")
	a.Pal.Input.CursorEnd()
	check("palette", "日本語")
	a.Pal.Open = false

	a.OpenFind()
	a.FindInput.SetValue("日本語")
	a.FindInput.CursorEnd()
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
		ti := core.NewInputField(0)
		ti.SetWidth(c.width)
		ti.Focus()
		for _, r := range c.value {
			ti.Update(textKey(string(r)))
		}
		for range c.left {
			ti.Update(core.Key{Code: KeyLeft})
		}
		if got := ti.Caret(); got != c.want {
			t.Errorf("%q width %d left %d: caret = %d, want %d", c.value, c.width, c.left, got, c.want)
		}
	}
}

// With nothing to type into, the hidden cursor rests where the user looks, so
// that the input mode mark of macOS does not show up in the bottom right.
func TestRestPos(t *testing.T) {
	a := newTestApp(t, "x\n")
	l := a.layout()

	// The first screen: no file open.
	a.CloseTab(true)
	a.Focus = core.FocusEditor
	if _, _, ok := a.caretPos(l); ok {
		t.Fatal("no file: want no caret")
	}
	x, y, ok := a.restPos(l)
	if !ok || x != l.fullX+l.fullW/2 || y != l.editY+l.editH/2 {
		t.Errorf("no file: rest = (%d,%d,%v), want the middle of the editor area", x, y, ok)
	}
	if c := a.caretCursor(l); c == nil || c.Position.X != x || c.Position.Y != y {
		t.Errorf("no file: cursor = %v, want (%d,%d)", c, x, y)
	}

	// The tree: the selected row.
	a.Focus = core.FocusExplorer
	a.Explorer.Sel = min(1, len(a.Explorer.Flat)-1)
	x, y, ok = a.restPos(l)
	if !ok {
		t.Fatal("explorer: no rest position")
	}
	if want := l.top + 1 + a.Explorer.Sel - a.Explorer.Scroll; y != want {
		t.Errorf("explorer: rest y = %d, want %d", y, want)
	}
	if x >= l.sideW {
		t.Errorf("explorer: rest x = %d, outside the sidebar (%d)", x, l.sideW)
	}
}

// The CHANGES list rests the cursor on its selected row, below the tree.
func TestRestPosInChanges(t *testing.T) {
	a, _ := gitDiffApp(t)
	l := a.layout()
	a.SetFocus(core.FocusChanges)
	rows := a.ChangesOf(l.bodyH - a.ExplorerH()).Rows
	sel := -1
	for i, r := range rows {
		if r.Selected {
			sel = i
		}
	}
	if sel < 0 {
		t.Skip("no selected row in the list")
	}
	x, y, ok := a.restPos(l)
	if !ok || y != l.top+a.ExplorerH()+1+sel || x != sideRowX(rows[sel].Depth, l) {
		t.Errorf("changes: rest = (%d,%d,%v), want row %d", x, y, ok, sel)
	}
}
