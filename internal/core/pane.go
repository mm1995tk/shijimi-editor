package core

import (
	"strings"

	"github.com/alecthomas/chroma/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// paneOpts is what App decides about the contents of a text pane, apart from
// the buffer itself. How it is drawn (focus, search highlight, line number
// style, diff colors) stays with editorOpts.
// extraCursor is a cursor of a multi-cursor edit with its selection (an empty pair
// when it selects nothing).
type extraCursor struct {
	cursor buffer.Pos
	sel    [2]buffer.Pos
}

type paneOpts struct {
	cursorAt     buffer.Pos    // where the cursor is drawn
	extras       []extraCursor // cursors of a multi-cursor edit
	indentGuides bool          // fill Line.Guides
	title        *screen.Title // the title row of the pane, nil = just the file name
	diags        []lsp.Diagnostic
}

// buildTextPane makes the screen.Pane of the buffer of t as it is now: the
// lines in view (scrolled by the buffer's ScrollRow, h of them at most) with
// their text split into spans by syntax color, the cursors and selections, the
// diagnostics and the indent guides. It changes nothing. w is the width of
// the pane with the line numbers; the line numbers themselves, the widths of
// characters and the tab stops are the drawing side's work (the mapping from
// a screen column back to a rune is buffer.ColFromVisual).
func buildTextPane(t *tab, w, h int, o paneOpts) screen.Pane {
	b := t.buf
	p := textPaneHead(t, w, h, o)
	colors := t.hl.Colors(b)
	unit := guideUnit(b, o)
	for i := range h {
		row := b.ScrollRow + i
		if row >= len(b.Lines) {
			break
		}
		p.Lines = append(p.Lines, textLine(b, colors, row, o, unit))
	}
	return p
}

// guideUnit is the step of the indent guides of b (1 when there are none).
func guideUnit(b *buffer.Buffer, o paneOpts) int {
	if o.indentGuides {
		return indentUnitWidth(b)
	}
	return 1
}

// textPaneHead is buildTextPane without the lines: the title, the viewport and
// the cursors and selections of the buffer of t.
func textPaneHead(t *tab, w, h int, o paneOpts) screen.Pane {
	b := t.buf
	p := screen.Pane{
		ID:   t.path(),
		Kind: screen.KindText,
		Title: screen.Title{
			Text:  t.title(),
			Dirty: t.dirty(),
		},
		Viewport: screen.Viewport{
			Top: b.ScrollRow, Rows: h, Cols: max(0, w-gutterWidth(b)),
			Left: b.ScrollCol, Total: len(b.Lines),
		},
		CurLine: b.Cur.Row,
	}
	if o.title != nil {
		p.Title = *o.title
	}

	// Cursors[0] is the main one; Selections is parallel to Cursors.
	p.Cursors = append(p.Cursors, toScreenPos(o.cursorAt))
	if s, e, ok := b.Selection(); ok {
		p.Selections = append(p.Selections, screen.Range{Start: toScreenPos(s), End: toScreenPos(e)})
	} else {
		p.Selections = append(p.Selections, screen.Range{Start: p.Cursors[0], End: p.Cursors[0]})
	}
	for _, x := range o.extras {
		sp := toScreenPos(x.cursor)
		r := screen.Range{Start: sp, End: sp}
		if s := x.sel; s[0] != s[1] {
			r = screen.Range{Start: toScreenPos(s[0]), End: toScreenPos(s[1])}
		}
		p.Cursors = append(p.Cursors, sp)
		p.Selections = append(p.Selections, r)
	}

	return p
}

// textLine is the line row of b with its colors (the highlighter's, may be
// nil), diagnostics and indent guides (unit apart, see guideUnit).
func textLine(b *buffer.Buffer, colors [][]string, row int, o paneOpts, unit int) screen.Line {
	line := b.Lines[row]
	var lc []string
	if colors != nil && row < len(colors) {
		lc = colors[row]
	}
	ln := screen.Line{No: row, Spans: spansOf(line, lc)}
	ln.Diag = lineDiagnostics(line, row, o.diags)
	if d, ok := headDiagnostic(ln.Diag); ok {
		ln.Marks = append(ln.Marks, screen.Mark{Kind: screen.MarkDiag, Style: diagStyle(d.Severity)})
	}
	if o.indentGuides {
		ln.Indent = indentGuideWidth(b, row)
		for v := 0; v < ln.Indent; v += unit {
			ln.Guides = append(ln.Guides, v)
		}
	}
	return ln
}

func toScreenPos(p buffer.Pos) screen.Pos { return screen.Pos{Line: p.Row, Col: p.Col} }

func posLess(a, b screen.Pos) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
}

// spansOf splits line into runs of one color. lc is the per-rune color
// ("" = none) of the highlighter, which becomes the theme's name for that
// color.
func spansOf(line []rune, lc []string) []screen.Span {
	var spans []screen.Span
	start := 0
	for i := 1; i <= len(line); i++ {
		if i < len(line) && styleAt(lc, i) == styleAt(lc, start) {
			continue
		}
		spans = append(spans, screen.Span{Text: string(line[start:i]), Style: styleAt(lc, start)})
		start = i
	}
	return spans
}

func styleAt(lc []string, i int) screen.Style {
	if i < len(lc) {
		return syntaxStyleOf(lc[i])
	}
	return ""
}

// syntaxOrder is the order in which syntax colors are looked up by color
// when several of them have the same one.
var syntaxOrder = []screen.Style{
	screen.StyleSyntaxText, screen.StyleSyntaxKeyword, screen.StyleSyntaxType, screen.StyleSyntaxFunction,
	screen.StyleSyntaxName, screen.StyleSyntaxConst, screen.StyleSyntaxString, screen.StyleSyntaxNumber,
	screen.StyleSyntaxComment, screen.StyleSyntaxNamespace, screen.StyleSyntaxEscape, screen.StyleSyntaxPreproc,
	screen.StyleSyntaxControl,
}

// themeCache holds what is derived from the current theme, rebuilt when
// themeGen changes.
var themeCache struct {
	gen     int
	palette screen.Palette
	byColor map[string]screen.Style // normalized syntax color -> its name
}

func themeTables() {
	if themeCache.palette != nil && themeCache.gen == themeGen {
		return
	}
	themeCache.gen = themeGen
	themeCache.palette = curTheme.Palette()
	themeCache.byColor = map[string]screen.Style{}
	for _, st := range syntaxOrder {
		c := normColor(themeCache.palette[st])
		if _, ok := themeCache.byColor[c]; c != "" && !ok {
			themeCache.byColor[c] = st
		}
	}
}

func normColor(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(chroma.ParseColour(s).String())
}

// syntaxStyleOf is the name of the syntax color hex ("" when it is none of the
// theme's, as for colors lexed under the theme before the last switch).
func syntaxStyleOf(hex string) screen.Style {
	if hex == "" {
		return ""
	}
	themeTables()
	return themeCache.byColor[normColor(hex)]
}

// styleColor is the color the current theme gives to st ("" when it has none).
func styleColor(st screen.Style) string {
	if st == "" {
		return ""
	}
	themeTables()
	return themeCache.palette[st]
}

// diagStyle is the name of the color of a diagnostic of severity sev.
func diagStyle(sev screen.Severity) screen.Style {
	switch sev {
	case screen.SeverityWarning:
		return screen.StyleDiagWarn
	case screen.SeverityInfo:
		return screen.StyleDiagInfo
	case screen.SeverityHint:
		return screen.StyleDiagHint
	}
	return screen.StyleDiagError
}

// lineDiagnostics is the diagnostics that touch row, with their range in
// runes of line. One that starts on an earlier row has Cont set.
func lineDiagnostics(line []rune, row int, diags []lsp.Diagnostic) []screen.Diagnostic {
	var out []screen.Diagnostic
	for _, dg := range diags {
		if dg.Range.Start.Line > row || dg.Range.End.Line < row {
			continue
		}
		s, e := 0, len(line)+1
		if dg.Range.Start.Line == row {
			s = lsp.FromUTF16(line, dg.Range.Start.Character)
		}
		if dg.Range.End.Line == row {
			e = lsp.FromUTF16(line, dg.Range.End.Character)
		}
		if e == s {
			e = s + 1
		}
		out = append(out, screen.Diagnostic{
			Start: s, End: e, Severity: screen.Severity(sevRank(dg.Severity)),
			Message: dg.Message, Cont: dg.Range.Start.Line < row,
		})
	}
	return out
}

// headDiagnostic is the diagnostic that shows at the end of the line and in
// the gutter: the most severe of those starting on the line, the first of
// equals.
func headDiagnostic(ds []screen.Diagnostic) (screen.Diagnostic, bool) {
	var best screen.Diagnostic
	found := false
	for _, d := range ds {
		if d.Cont {
			continue
		}
		if !found || d.Severity < best.Severity {
			best, found = d, true
		}
	}
	return best, found
}
