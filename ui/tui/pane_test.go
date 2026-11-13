package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
)

// Full-width characters take two cells and a tab goes to the next tab stop.
// A combining character takes a cell of its own (the width core.RuneWidth gives, which both
// the draw and the mouse use, so the two agree). The pane keeps whole runes; the
// drawing side lays them out.
func TestPaneWideAndCombining(t *testing.T) {
	line := []rune("世é\tx")
	for _, c := range []struct{ col, vis int }{{0, 0}, {1, 2}, {2, 3}, {3, 4}, {4, 8}, {5, 9}} {
		if got := core.VisualCol(line, c.col); got != c.vis {
			t.Errorf("VisualCol(%d) = %d, want %d", c.col, got, c.vis)
		}
	}
	tb := paneTab(string(line))
	rows := renderEditor(tb, 30, 1, core.EditorOpts{})
	if got := strings.TrimRight(string([]rune(ansi.Strip(rows[0]))[core.GutterWidth(tb.Buf):]), " "); got != "世é    x" {
		t.Errorf("drawn = %q", got)
	}
}

func TestMouseClickOnWideLine(t *testing.T) {
	a := newTestApp(t, "a世éb\n")
	l := a.layout()
	gw := core.GutterWidth(a.CurText().Buf)
	// 世 occupies the text cells 1 and 2; a click on the cell after it puts the cursor after it
	a.mouseToCursor(l, l.editY-l.top, gw+3, false)
	if got := a.CurText().Buf.Cur; got != (core.Pos{Row: 0, Col: 2}) {
		t.Errorf("cursor = %+v, want col 2", got)
	}
	a.mouseToCursor(l, l.editY-l.top, gw+1, false)
	if got := a.CurText().Buf.Cur; got != (core.Pos{Row: 0, Col: 1}) {
		t.Errorf("cursor = %+v, want col 1", got)
	}
}

func TestMouseDragIncludesCharUnderPointer(t *testing.T) {
	a := newTestApp(t, "abcdef\nxy世zw\n")
	l := a.layout()
	b := a.CurText().Buf
	gw := core.GutterWidth(b)
	row := l.editY - l.top
	click := func(r, c int) { a.mouseToCursor(l, row+r, gw+c, false) }
	drag := func(r, c int) { a.mouseToCursor(l, row+r, gw+c, true) }
	sel := func() string { return b.SelectedText() }

	// rightward: the character under the pointer is selected
	click(0, 1)
	drag(0, 3)
	if got := sel(); got != "bcd" {
		t.Errorf("right drag = %q, want bcd", got)
	}
	// leftward: unchanged, the character the drag started on is selected
	click(0, 3)
	drag(0, 1)
	if got := sel(); got != "bc" {
		t.Errorf("left drag = %q, want bc", got)
	}
	// downward onto a wide character, on either of its two cells
	for _, c := range []int{2, 3} {
		click(0, 1)
		drag(1, c)
		if got := sel(); got != "bcdef\nxy世" {
			t.Errorf("drag onto wide char at cell %d = %q", c, got)
		}
	}
	// past the end of the line stays at the end
	click(0, 4)
	drag(0, 20)
	if got := sel(); got != "ef" {
		t.Errorf("drag past end = %q, want ef", got)
	}
}
