package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// Full-width characters take two cells and a tab goes to the next tab stop.
// A combining character takes a cell of its own (the width buffer.RuneWidth gives, which both
// the draw and the mouse use, so the two agree). The pane keeps whole runes; the
// drawing side lays them out.
func TestPaneWideAndCombining(t *testing.T) {
	line := []rune("世é\tx")
	for _, c := range []struct{ col, vis int }{{0, 0}, {1, 2}, {2, 3}, {3, 4}, {4, 8}, {5, 9}} {
		if got := buffer.VisualCol(line, c.col); got != c.vis {
			t.Errorf("VisualCol(%d) = %d, want %d", c.col, got, c.vis)
		}
	}
	tb := paneTab(string(line))
	rows := renderEditor(tb, 30, 1, EditorOpts{})
	if got := strings.TrimRight(string([]rune(ansi.Strip(rows[0]))[GutterWidth(tb.Buf):]), " "); got != "世é    x" {
		t.Errorf("drawn = %q", got)
	}
}

func TestMouseClickOnWideLine(t *testing.T) {
	a := newTestApp(t, "a世éb\n")
	l := a.layout()
	gw := GutterWidth(a.CurText().Buf)
	// 世 occupies the text cells 1 and 2; a click on the cell after it puts the cursor after it
	a.mouseToCursor(l, l.editY-l.top, gw+3, false)
	if got := a.CurText().Buf.Cur; got != (buffer.Pos{Row: 0, Col: 2}) {
		t.Errorf("cursor = %+v, want col 2", got)
	}
	a.mouseToCursor(l, l.editY-l.top, gw+1, false)
	if got := a.CurText().Buf.Cur; got != (buffer.Pos{Row: 0, Col: 1}) {
		t.Errorf("cursor = %+v, want col 1", got)
	}
}
