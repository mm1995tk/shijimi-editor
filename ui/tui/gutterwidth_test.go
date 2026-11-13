package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

// The line number column is a mark cell, the number and one space: the digits
// of the line count plus 2 (5 cells for a file of 3 digits).
func TestGutterWidthIsDigitsPlusTwo(t *testing.T) {
	for _, c := range []struct{ total, want int }{{1, 3}, {9, 3}, {10, 4}, {99, 4}, {100, 5}, {999, 5}, {1000, 6}} {
		if got := core.GutterWidthFor(c.total); got != c.want {
			t.Errorf("GutterWidthFor(%d) = %d, want %d", c.total, got, c.want)
		}
	}
}

// With 100 lines the text starts at the 5th cell: the mark, "  1", a space.
// A click on a cell of the text and the drawn text agree.
func TestGutterWidthDrawAndClick(t *testing.T) {
	a := newTestApp(t, strings.Repeat("abcdef\n", 99)+"abcdef")
	tb := a.CurText()
	if gw := core.GutterWidth(tb.Buf); gw != 5 {
		t.Fatalf("GutterWidth = %d, want 5", gw)
	}
	rows := renderEditor(tb, 30, 2, core.EditorOpts{})
	if got := strings.TrimRight(ansi.Strip(rows[0]), " "); got != "   1 abcdef" {
		t.Errorf("drawn = %q", got)
	}
	l := a.layout()
	a.mouseToCursor(l, l.editY-l.top, 5+3, false) // on the "d"
	if got := tb.Buf.Cur; got != (core.Pos{Row: 0, Col: 3}) {
		t.Errorf("cursor = %+v, want col 3", got)
	}
	a.mouseToCursor(l, l.editY-l.top, 4, false) // on the space of the gutter
	if got := tb.Buf.Cur; got.Col != 0 {
		t.Errorf("cursor = %+v, want col 0", got)
	}
}

// The base side of the diff has the same width as the editor.
func TestGutterWidthDiffOldLine(t *testing.T) {
	p := screen.Pane{Viewport: screen.Viewport{Total: 120}}
	pl := screen.Line{No: 6, Spans: []screen.Span{{Text: "abc"}}}
	got := ansi.Strip(drawOldLine(p, pl, 20))
	if want := "   7 abc"; strings.TrimRight(got, " ") != want {
		t.Errorf("drawn = %q, want %q", got, want)
	}
	if n := ansi.StringWidth(got); n != 20 {
		t.Errorf("width = %d, want 20", n)
	}
}

// In diff mode a click on the Nth cell of the text of the right side puts the
// cursor on that character: the text starts after the 3 cells of the line
// number column of a file of fewer than 10 lines.
func TestGutterWidthDiffClick(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte("abcdef\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	os.WriteFile(file, []byte("abcdef!\n"), 0o644)
	a := mustNew(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Gd.On = true
	a.Gd.Base = core.BaseHead
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BaseHead, nil).(core.ChangesLoadedMsg))
	tb := a.CurText()
	dv := a.DiffFor(tb)
	if dv == nil {
		t.Fatal("no diff view")
	}
	if gw := core.GutterWidth(tb.Buf); gw != 3 {
		t.Fatalf("GutterWidth = %d, want 3", gw)
	}
	l := a.layout()
	g := a.dvGeom(l)
	row, left := g.Top-l.top, g.RightX-l.rightX
	a.diffMouseToCursor(tb, dv, l, row, left+3+3, false) // the 4th cell of the text: "d"
	if got := tb.Buf.Cur; got != (core.Pos{Row: 0, Col: 3}) {
		t.Errorf("cursor = %+v, want col 3", got)
	}
	a.diffMouseToCursor(tb, dv, l, row, left+2, false) // the space after the number
	if got := tb.Buf.Cur; got.Col != 0 {
		t.Errorf("cursor = %+v, want col 0", got)
	}
}
