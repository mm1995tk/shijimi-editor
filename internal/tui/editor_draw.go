package tui

import (
	"fmt"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

type cellStyle struct {
	fg, bg string
	cursor string // cursor color, "" = none
	under  string // underline color
}

var cellStyles = map[cellStyle]lipgloss.Style{}

func styleFor(k cellStyle) lipgloss.Style {
	if s, ok := cellStyles[k]; ok {
		return s
	}
	s := bgfg(k.bg, k.fg)
	if k.cursor != "" {
		s = bgfg(k.cursor, core.CurTheme.CursorFg)
	}
	if k.under != "" {
		s = s.Underline(true)
	}
	cellStyles[k] = s
	return s
}

// drawTextPane draws the Pane of a text file as exactly h lines of width w.
// It reads only p and o: the cell width of characters, tab stops, the line
// numbers and the colors of the theme are decided here.
func drawTextPane(p screen.Pane, w, h int, o core.EditorOpts) []string {
	ld := newLineDrawer(p, w, o)
	out := make([]string, 0, h)
	for i := range h {
		if i >= len(p.Lines) {
			out = append(out, stEmpty.Render(strings.Repeat(" ", w)))
			continue
		}
		out = append(out, ld.draw(p.Lines[i]))
	}
	return out
}

// lineDrawer draws the lines of one text pane (a Pane of Kind text, or the
// changed side of a diff) as cells of the width the pane has.
type lineDrawer struct {
	p         screen.Pane
	o         core.EditorOpts
	gw, textW int
	scrollCol int
	q         []rune
	hasSel    bool
}

func newLineDrawer(p screen.Pane, w int, o core.EditorOpts) lineDrawer {
	gw := core.GutterWidthFor(p.Viewport.Total)
	return lineDrawer{
		p: p, o: o, gw: gw, textW: max(0, w-gw), scrollCol: p.Viewport.Left,
		q:      []rune(strings.ToLower(o.Query)),
		hasSel: len(p.Selections) > 0 && p.Selections[0].Start != p.Selections[0].End,
	}
}

// draw draws the line pl, which is one of ld.p.Lines.
func (ld lineDrawer) draw(pl screen.Line) string {
	p, o, gw, textW, scrollCol, q, hasSel := ld.p, ld.o, ld.gw, ld.textW, ld.scrollCol, ld.q, ld.hasSel
	row := pl.No
	isCur := row == p.CurLine
	lineBg := cEditorBg
	if isCur && !hasSel {
		lineBg = cCurLine
	}
	// a diff row keeps its own background under the cursor, so an added
	// line stays green when the cursor is on it
	if p.Kind == screen.KindDiff && changedLine(pl) {
		lineBg = cAddBg
	}
	numSt := stLineNo
	if isCur {
		numSt = bgfg(lineBg, cLineNoCur)
	}
	n := row + 1
	if o.Relative && !isCur {
		n = row - p.CurLine
		if n < 0 {
			n = -n
		}
	}
	var sb strings.Builder
	var hd *screen.Diagnostic // the diagnostic shown at the end of the line
	if d, ok := core.HeadDiagnostic(pl.Diag); ok {
		hd = &d
	}
	mark := ""
	if o.GutterMark != nil {
		mark = o.GutterMark(row)
	}
	diagMark := ""
	for _, m := range pl.Marks {
		if m.Kind == screen.MarkDiag {
			diagMark = core.StyleColor(m.Style)
		}
	}
	if mark != "" {
		sb.WriteString(bgfg(lineBg, mark).Render("●"))
	} else if diagMark != "" {
		sb.WriteString(bgfg(lineBg, diagMark).Render("●"))
	} else {
		sb.WriteString(bgfg(lineBg, cLineNo).Render(" "))
	}
	sb.WriteString(numSt.Render(fmt.Sprintf("%*d  ", gw-3, n)))

	// the text of the line and the color of each rune ("" = none)
	var line []rune
	var lc []string
	for _, sp := range pl.Spans {
		c := core.StyleColor(sp.Style)
		for _, r := range sp.Text {
			line = append(line, r)
			lc = append(lc, c)
		}
	}
	var matchCols []int
	if len(q) > 0 {
		matchCols = buffer.MatchCols(line, q)
	}
	inMatch := func(col int) bool {
		for _, m := range matchCols {
			if col >= m && col < m+len(q) {
				return true
			}
		}
		return false
	}
	inSel := func(col int) bool {
		at := screen.Pos{Line: row, Col: col}
		for _, s := range p.Selections {
			if !core.PosLess(at, s.Start) && core.PosLess(at, s.End) {
				return true
			}
		}
		return false
	}
	isCursor := func(col int) bool {
		for _, c := range p.Cursors {
			if c.Line == row && c.Col == col {
				return true
			}
		}
		return false
	}
	// underline the ranges of diagnostics on this line
	underline := func(col int) string {
		best := ""
		bestRank := 99
		for _, dg := range pl.Diag {
			if col >= dg.Start && col < dg.End && int(dg.Severity) < bestRank {
				best, bestRank = core.StyleColor(core.DiagStyle(dg.Severity)), int(dg.Severity)
			}
		}
		return best
	}

	var seg strings.Builder
	var segKey cellStyle
	segOpen := false
	flush := func() {
		if segOpen {
			sb.WriteString(styleFor(segKey).Render(seg.String()))
			seg.Reset()
			segOpen = false
		}
	}
	put := func(k cellStyle, s string) {
		if segOpen && k != segKey {
			flush()
		}
		segKey = k
		segOpen = true
		seg.WriteString(s)
	}

	vis := 0
	drawn := 0
	guideW := pl.Indent
	isGuideCol := func(v int) bool { return slices.Contains(pl.Guides, v) }
	cellBg := func(col int) string {
		switch {
		case inSel(col):
			return cSelection
		case inMatch(col):
			return cMatch
		}
		for _, m := range pl.Words {
			if col >= m.Start && col < m.End {
				return cAddWordBg
			}
		}
		return lineBg
	}
	for col := 0; col <= len(line) && drawn < textW; col++ {
		cursorHere := o.Focused && isCursor(col)
		var r rune = ' '
		rw := 1
		if col < len(line) {
			r = line[col]
			rw = buffer.RuneWidth(r, vis)
		} else if !cursorHere && !inSel(col) {
			break
		}
		fg := cEditorFg
		if col < len(lc) && lc[col] != "" {
			fg = lc[col]
		}
		k := cellStyle{fg: fg, bg: cellBg(col)}
		if hd != nil && col < len(line) { // a row no diagnostic starts on is not underlined (only Cont ones touch it)
			if u := underline(col); u != "" {
				k.under = u
				k.fg = u
				if col < len(lc) && lc[col] != "" {
					k.fg = lc[col]
				}
			}
		}
		isGuide := col < len(line) && (r == ' ' || r == '\t') && vis < guideW && isGuideCol(vis)
		if isGuide {
			k.fg = cIndentGuide
		}
		if cursorHere {
			k.cursor = o.CursorColor
		}
		if r == '\t' || col == len(line) {
			for c := 0; c < rw; c++ {
				if vis+c >= scrollCol && drawn < textW {
					kk := k
					if c > 0 {
						kk.cursor = ""
					}
					if c == 0 && isGuide {
						put(kk, "│")
					} else {
						put(kk, " ")
					}
					drawn++
				}
			}
		} else if vis >= scrollCol {
			if drawn+rw > textW {
				break
			}
			if isGuide {
				r = '│'
			}
			put(k, string(r))
			drawn += rw
		} else if vis+rw > scrollCol {
			put(k, strings.Repeat(" ", vis+rw-scrollCol))
			drawn += vis + rw - scrollCol
		}
		vis += rw
	}
	// guides of a blank line extend past its end
	for v := vis; v < guideW && drawn < textW; v++ {
		if v < scrollCol {
			continue
		}
		ch := " "
		if isGuideCol(v) {
			ch = "│"
		}
		put(cellStyle{fg: cIndentGuide, bg: cellBg(len(line))}, ch)
		drawn++
	}
	flush()
	// inline diagnostic message (like Helix's inline diagnostics)
	if hd != nil && drawn+6 < textW {
		msg := "   ● " + core.FirstLine(hd.Message)
		msg = core.Fit(msg, textW-drawn)
		sb.WriteString(bgfg(lineBg, core.StyleColor(core.DiagStyle(hd.Severity))).Faint(true).Render(msg))
		drawn = textW
	}
	if drawn < textW {
		sb.WriteString(styleFor(cellStyle{fg: cEditorFg, bg: lineBg}).Render(strings.Repeat(" ", textW-drawn)))
	}
	return sb.String()
}

func renderWelcome(w, h int, helix bool) []string {
	type row struct {
		text string
		head bool
	}
	msg := []row{{"shijimi", true}, {"", false}}
	if helix {
		msg = append(msg,
			row{i18n.T("help.vs.openfile"), false},
			row{i18n.T("help.vs.palette"), false},
			row{i18n.T("help.vs.tree"), false},
			row{i18n.T("help.vs.diffpr"), false},
			row{i18n.T("help.vs.diffhead"), false},
			row{i18n.T("help.vs.terminal"), false},
			row{i18n.T("help.vs.quit"), false},
		)
	} else {
		msg = append(msg,
			row{i18n.T("help.vsc.openfile"), false},
			row{i18n.T("help.vsc.palette"), false},
			row{i18n.T("help.vsc.sidebar"), false},
			row{i18n.T("help.vsc.tree"), false},
			row{i18n.T("help.vs.diffhead"), false},
			row{i18n.T("help.vs.terminal"), false},
			row{i18n.T("help.vsc.quit"), false},
		)
	}
	out := make([]string, h)
	top := max(0, (h-len(msg))/2)
	for i := range out {
		out[i] = stEmpty.Render(strings.Repeat(" ", w))
	}
	for i, m := range msg {
		if top+i >= h {
			break
		}
		width := lipgloss.Width(m.text)
		pad := max(0, (w-34)/2)
		st := stWelcome
		if m.head {
			pad = max(0, (w-width)/2)
			st = stWelcomeH
		}
		out[top+i] = st.Render(core.Fit(strings.Repeat(" ", pad)+m.text, w))
	}
	return out
}

// renderEditor draws the visible part of a buffer as exactly h lines of
// width w: the Pane of the buffer (buildTextPane), drawn as cells.
func renderEditor(t *core.Tab, w, h int, o core.EditorOpts) []string {
	return drawTextPane(core.BuildTextPane(t, w, h, o.PaneOpts), w, h, o)
}
