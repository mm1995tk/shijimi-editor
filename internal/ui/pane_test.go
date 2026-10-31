package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func paneTab(src string) *tab {
	b := buffer.New()
	b.Lines = nil
	for l := range strings.SplitSeq(src, "\n") {
		b.Lines = append(b.Lines, []rune(l))
	}
	return newTab(b)
}

// The Pane has the lines in view with their text whole, colors as names of
// the theme, the cursors and selections, diagnostics and indent guides.
func TestBuildTextPane(t *testing.T) {
	tb := paneTab("a\n    b\n\n    c")
	b := tb.buf
	b.ScrollRow = 1
	b.Cur = buffer.Pos{Row: 1, Col: 5}
	anchor := buffer.Pos{Row: 1, Col: 4}
	b.Anchor = &anchor
	diag := lsp.Diagnostic{
		Range:    lsp.Range{Start: lsp.Position{Line: 1, Character: 4}, End: lsp.Position{Line: 2, Character: 0}},
		Severity: 2, Message: "careful",
	}
	p := buildTextPane(tb, 40, 3, paneOpts{
		cursorAt: b.Cur, indentGuides: true, diags: []lsp.Diagnostic{diag},
		extras: []extraCursor{{cursor: buffer.Pos{Row: 3, Col: 4}, sel: [2]buffer.Pos{{Row: 3, Col: 4}, {Row: 3, Col: 4}}}},
	})

	if p.Kind != screen.KindText || p.CurLine != 1 {
		t.Errorf("kind %q curLine %d", p.Kind, p.CurLine)
	}
	if want := (screen.Viewport{Top: 1, Rows: 3, Cols: 40 - gutterWidth(b), Total: 4}); p.Viewport != want {
		t.Errorf("viewport = %+v, want %+v", p.Viewport, want)
	}
	if len(p.Lines) != 3 || p.Lines[0].No != 1 || p.Lines[2].No != 3 {
		t.Fatalf("lines = %+v", p.Lines)
	}
	if got := p.Lines[0].Spans; len(got) != 1 || got[0].Text != "    b" {
		t.Errorf("spans = %+v", got)
	}
	if got := p.Lines[0].Guides; len(got) != 1 || got[0] != 0 || p.Lines[0].Indent != 4 {
		t.Errorf("guides = %v indent %d", got, p.Lines[0].Indent)
	}
	// the blank line takes the deeper of its neighbours
	if l := p.Lines[1]; l.Indent != 4 || len(l.Spans) != 0 {
		t.Errorf("blank line = %+v", l)
	}
	if len(p.Cursors) != 2 || p.Cursors[0] != (screen.Pos{Line: 1, Col: 5}) || p.Cursors[1] != (screen.Pos{Line: 3, Col: 4}) {
		t.Errorf("cursors = %v", p.Cursors)
	}
	if len(p.Selections) != 2 || p.Selections[0] != (screen.Range{Start: screen.Pos{Line: 1, Col: 4}, End: screen.Pos{Line: 1, Col: 5}}) ||
		p.Selections[1].Start != p.Selections[1].End {
		t.Errorf("selections = %v", p.Selections)
	}
	l0 := p.Lines[0]
	if len(l0.Diag) != 1 || l0.Diag[0].Cont || l0.Diag[0].Start != 4 || l0.Diag[0].Severity != screen.SeverityWarning {
		t.Errorf("diag = %+v", l0.Diag)
	}
	if len(l0.Marks) != 1 || l0.Marks[0].Kind != screen.MarkDiag || l0.Marks[0].Style != screen.StyleDiagWarn {
		t.Errorf("marks = %+v", l0.Marks)
	}
	// the diagnostic goes on to the next line, as an underline only
	if l1 := p.Lines[1]; len(l1.Diag) != 1 || !l1.Diag[0].Cont || len(l1.Marks) != 0 {
		t.Errorf("continued diag = %+v marks %+v", l1.Diag, l1.Marks)
	}
}

// Syntax colors reach the Pane as the theme's names, and the same name gives
// the same color back.
func TestPaneSyntaxStyles(t *testing.T) {
	tb := paneTab("if x")
	kw, str := curTheme.Syntax.Keyword, curTheme.Syntax.String
	tb.hl.colors = [][]string{{kw, kw, "", str}}
	tb.hl.lines = []string{"if x"}
	tb.hl.version = tb.buf.Version
	p := buildTextPane(tb, 20, 1, paneOpts{})
	got := p.Lines[0].Spans
	want := []screen.Span{
		{Text: "if", Style: screen.StyleSyntaxKeyword},
		{Text: " "},
		{Text: "x", Style: screen.StyleSyntaxString},
	}
	if len(got) != len(want) {
		t.Fatalf("spans = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if styleColor(screen.StyleSyntaxKeyword) != kw {
		t.Errorf("keyword color = %q, want %q", styleColor(screen.StyleSyntaxKeyword), kw)
	}
}

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
	rows := renderEditor(tb, 30, 1, editorOpts{})
	if got := strings.TrimRight(string([]rune(ansi.Strip(rows[0]))[gutterWidth(tb.buf):]), " "); got != "世é    x" {
		t.Errorf("drawn = %q", got)
	}
}

// A click maps the screen column to the rune of the clicked line, from the
// characters of that line.
func TestMouseColumnToRune(t *testing.T) {
	line := []rune("a世é\tb")
	for _, c := range []struct{ vis, col int }{
		{0, 0}, {1, 1}, // a | 世 starts at 1
		{2, 1},         // 世 takes the cells 1 and 2; both put the cursor before it
		{3, 2},         // the cell after it puts the cursor after it
		{4, 3},         // e at 3, combining at 4
		{5, 4},         // the tab starts at 5
		{6, 4}, {7, 5}, // inside the tab (cells 5 to 7): the nearer edge
		{8, 5},          // b
		{99, len(line)}, // past the end
	} {
		if got := buffer.ColFromVisual(line, c.vis); got != c.col {
			t.Errorf("ColFromVisual(%d) = %d, want %d", c.vis, got, c.col)
		}
	}
}

func TestMouseClickOnWideLine(t *testing.T) {
	a := newTestApp(t, "a世éb\n")
	l := a.layout()
	gw := gutterWidth(a.curText().buf)
	// 世 occupies the text cells 1 and 2; a click on the cell after it puts the cursor after it
	a.mouseToCursor(l, l.editY-l.top, gw+3, false)
	if got := a.curText().buf.Cur; got != (buffer.Pos{Row: 0, Col: 2}) {
		t.Errorf("cursor = %+v, want col 2", got)
	}
	a.mouseToCursor(l, l.editY-l.top, gw+1, false)
	if got := a.curText().buf.Cur; got != (buffer.Pos{Row: 0, Col: 1}) {
		t.Errorf("cursor = %+v, want col 1", got)
	}
}
