package ui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

type tabKind int

const (
	tabText tabKind = iota
	tabImage
)

// tab is one item in the editor area: a text buffer or an image preview.
type tab struct {
	kind tabKind

	buf        *buffer.Buffer
	hl         *highlighter
	lang       *config.Language
	lspVersion int
	autoVer    int  // buffer version an auto-save timer was last scheduled for
	conflict   bool // changed on disk while holding unsaved edits (already reported)

	// a transient tab (VS Code: preview) gives way to the next one until kept
	transient    bool
	transientVer int // buffer version when opened; an edit keeps the tab

	img *imageView
	dv  *diffView // set while the tab is shown as a diff (diff mode)

	// Markdown preview beside the buffer
	preview   previewMode
	prevCache []string
	prevW     int
	prevVer   int
	prevGen   int

	// mermaid fences and image sources found at scanVer (scanned reports
	// whether that is valid)
	scanned    bool
	scanVer    int
	scanBlocks []string
	scanImages []string
}

func newTab(b *buffer.Buffer) *tab {
	return &tab{kind: tabText, buf: b, hl: newHighlighter(b.Path), lspVersion: -1}
}

func (t *tab) title() string {
	switch t.kind {
	case tabImage:
		return t.img.name()
	}
	return t.buf.Name()
}

func (t *tab) dirty() bool { return t.kind == tabText && t.buf.Dirty }

func (t *tab) path() string {
	switch t.kind {
	case tabText:
		return t.buf.Path
	case tabImage:
		return t.img.path
	}
	return ""
}

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
		s = bgfg(k.cursor, curTheme.CursorFg)
	}
	if k.under != "" {
		s = s.Underline(true)
	}
	cellStyles[k] = s
	return s
}

// gutterWidthFor is the width of the line numbers column for a file of total lines.
func gutterWidthFor(total int) int { return len(fmt.Sprint(total)) + 4 }

func gutterWidth(b *buffer.Buffer) int { return gutterWidthFor(len(b.Lines)) }

// ensureCursorVisible adjusts scrolling so the cursor is on screen.
func ensureCursorVisible(b *buffer.Buffer, w, h int) {
	if h <= 0 {
		return
	}
	textW := max(1, w-gutterWidth(b))
	margin := min(3, (h-1)/2)
	if b.Cur.Row < b.ScrollRow+margin {
		b.ScrollRow = max(0, b.Cur.Row-margin)
	}
	if b.Cur.Row >= b.ScrollRow+h-margin {
		b.ScrollRow = b.Cur.Row - h + margin + 1
	}
	vis := buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)
	if vis < b.ScrollCol {
		b.ScrollCol = max(0, vis-textW/4)
	}
	if vis >= b.ScrollCol+textW {
		b.ScrollCol = vis - textW + 1 + textW/4
	}
}

// editorOpts is what App decides about one text pane: its contents (paneOpts,
// for buildTextPane) and how it is drawn (the rest, for drawPane).
type editorOpts struct {
	paneOpts
	focused     bool
	query       string
	cursorColor string
	relative    bool
	gutterMark  func(row int) string // color of a ● before the line number (review comments), "" = none
}

// diagColor is the color of a diagnostic of severity sev.
func diagColor(sev int) string {
	return styleColor(diagStyle(screen.Severity(sevRank(sev))))
}

// renderEditor draws the visible part of a buffer as exactly h lines of
// width w: the Pane of the buffer (buildTextPane), drawn as cells.
func renderEditor(t *tab, w, h int, o editorOpts) []string {
	return drawTextPane(buildTextPane(t, w, h, o.paneOpts), w, h, o)
}

// drawTextPane draws the Pane of a text file as exactly h lines of width w.
// It reads only p and o: the cell width of characters, tab stops, the line
// numbers and the colors of the theme are decided here.
func drawTextPane(p screen.Pane, w, h int, o editorOpts) []string {
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
	o         editorOpts
	gw, textW int
	scrollCol int
	q         []rune
	hasSel    bool
}

func newLineDrawer(p screen.Pane, w int, o editorOpts) lineDrawer {
	gw := gutterWidthFor(p.Viewport.Total)
	return lineDrawer{
		p: p, o: o, gw: gw, textW: max(0, w-gw), scrollCol: p.Viewport.Left,
		q:      []rune(strings.ToLower(o.query)),
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
	if o.relative && !isCur {
		n = row - p.CurLine
		if n < 0 {
			n = -n
		}
	}
	var sb strings.Builder
	var hd *screen.Diagnostic // the diagnostic shown at the end of the line
	if d, ok := headDiagnostic(pl.Diag); ok {
		hd = &d
	}
	mark := ""
	if o.gutterMark != nil {
		mark = o.gutterMark(row)
	}
	diagMark := ""
	for _, m := range pl.Marks {
		if m.Kind == screen.MarkDiag {
			diagMark = styleColor(m.Style)
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
		c := styleColor(sp.Style)
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
			if !posLess(at, s.Start) && posLess(at, s.End) {
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
				best, bestRank = styleColor(diagStyle(dg.Severity)), int(dg.Severity)
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
		cursorHere := o.focused && isCursor(col)
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
			k.cursor = o.cursorColor
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
		msg := "   ● " + firstLine(hd.Message)
		msg = fit(msg, textW-drawn)
		sb.WriteString(bgfg(lineBg, styleColor(diagStyle(hd.Severity))).Faint(true).Render(msg))
		drawn = textW
	}
	if drawn < textW {
		sb.WriteString(styleFor(cellStyle{fg: cEditorFg, bg: lineBg}).Render(strings.Repeat(" ", textW-drawn)))
	}
	return sb.String()
}

func sevRank(s int) int {
	if s == 0 {
		return 1
	}
	return s
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
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
		out[top+i] = st.Render(fit(strings.Repeat(" ", pad)+m.text, w))
	}
	return out
}

// reload re-reads the file from disk. Reload bumps the buffer version, but it
// is not a user edit, so a transient tab stays transient.
func (t *tab) reload() error {
	if err := t.buf.Reload(); err != nil {
		return err
	}
	t.transientVer = t.buf.Version
	return nil
}
