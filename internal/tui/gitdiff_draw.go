package tui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
	"github.com/rivo/uniseg"
)


func (a *App) dvGeom(l layout) core.DiffGeom {
	w := max(0, l.rightW-1) // the last column is the change bar
	g := core.DiffGeom{Top: l.editY + 1, H: max(0, l.editH-1), RightX: l.rightX, RightW: w, BarX: l.rightX + w}
	if a.splitShown() {
		g.LeftW = a.diffLeftWidth(w)
		g.RightX = l.rightX + g.LeftW + 1
		g.RightW = w - g.LeftW - 1
	}
	return g
}

// cursorScreen returns the screen position of the cursor cell.
func (a *App) cursorScreen(t *tab, l layout) (x, y int) { return a.posScreen(t, l, t.buf.Cur) }

// posScreen is the screen cell of the character at p in t's buffer.
func (a *App) posScreen(t *tab, l layout, p buffer.Pos) (x, y int) {
	b := t.buf
	x = l.rightX + gutterWidth(b) + buffer.VisualCol(b.Lines[p.Row], p.Col) - b.ScrollCol
	y = l.editY + p.Row - b.ScrollRow
	if dv := a.diffFor(t); dv != nil {
		g := a.dvGeom(l)
		x += g.RightX - l.rightX
		y = g.Top + dv.newIdx[min(b.Cur.Row, len(dv.newIdx)-1)] - dv.start(b.ScrollRow)
	}
	return x, y
}

// renderDiffText draws the diff view of t in the area l gives the file: the
// header, then the Panes (buildDiffPanes) with the change bar.
func (a *App) renderDiffText(t *tab, dv *diffView, l layout, focused bool) []string {
	g := a.dvGeom(l)
	opts := a.editorOpts(t, focused)
	opts.gutterMark = a.commentMark(t)
	panes := a.buildDiffPanes(t, dv, g, opts.paneOpts)

	hint := i18n.T("gd.splittoggle")
	if dv.single {
		hint = "" // one pane: there is nothing to switch
	}
	out := make([]string, 0, l.editH)
	out = append(out, drawDiffHeader(panes[len(panes)-1].Title, hint, l.rightW))
	return append(out, drawDiff(panes, g, opts)...)
}

// diffMouseToCursor moves the cursor to the buffer line under (x, y) in a
// diff view. Rows that exist only on the base side snap to the next line.
func (a *App) diffMouseToCursor(t *tab, dv *diffView, l layout, row, col int, extend bool) {
	g := a.dvGeom(l)
	b := t.buf
	// g is in screen cells; row and col count from the pane's corner.
	gTop, gLeft := g.Top-l.top, g.RightX-l.rightX
	col = clamp(col, gLeft, gLeft+g.RightW-1)
	if row < gTop {
		b.MoveLines(-1, extend)
	} else if row >= gTop+g.H {
		b.MoveLines(1, extend)
	}
	idx := clamp(dv.start(b.ScrollRow)+clamp(row-gTop, 0, max(0, g.H-1)), 0, len(dv.rows)-1)
	newRow := -1
	for i := idx; i < len(dv.rows) && newRow < 0; i++ {
		newRow = dv.rows[i].new
	}
	for i := idx; i >= 0 && newRow < 0; i-- {
		newRow = dv.rows[i].new
	}
	if newRow < 0 {
		return
	}
	vis := max(0, col-gLeft-gutterWidth(b)) + b.ScrollCol
	b.SetCursor(newRow, vis, extend)
	a.followCursor()
}

// drawChanges draws the list of changed files as h lines of w cells.
func drawChanges(ch screen.Changes, w, h int, focused bool) []string {
	out := make([]string, 0, h)
	chip := ch.Chip
	tw := max(0, w-ansi.StringWidth(chip))
	out = append(out, stSideTitle.Render(fit(ch.Title, tw))+bgfg(cSideBg, cAccent).Bold(true).Render(fit(chip, w-tw)))
	for i := range max(0, h-1) {
		if i >= len(ch.Rows) {
			msg := ""
			if i == 0 {
				msg = ch.Empty
			}
			out = append(out, stSide.Render(fit(msg, w)))
			continue
		}
		row := ch.Rows[i]
		indent := " " + strings.Repeat("  ", row.Depth)
		bg := cSideBg
		if row.Selected {
			bg = cListSelUnf
			if focused {
				bg = cListSel
			}
		}
		if row.Dir {
			mark := "▾ "
			if row.Closed {
				mark = "▸ "
			}
			out = append(out, bgfg(bg, curTheme.SubSel).Render(fit(indent+mark+row.Name, w)))
			continue
		}
		col := styleColor(row.LetterStyle)
		add, del := fmt.Sprintf(" +%d", row.Added), fmt.Sprintf(" -%d ", row.Deleted)
		viewed, notes := " ", ""
		if row.Viewed {
			viewed = "✓"
		}
		if row.Threads > 0 {
			notes = fmt.Sprintf(" ●%d", row.Threads)
		}
		nameW := max(0, w-ansi.StringWidth(indent)-2-ansi.StringWidth(notes+add+del))
		line := bgfg(bg, cAddSign).Render(viewed) + bgfg(bg, cSideFg).Render(indent[1:]) + bgfg(bg, col).Bold(true).Render(row.Letter) +
			bgfg(bg, cSideFg).Render(" "+fit(row.Name, nameW)) + bgfg(bg, curTheme.Renamed).Render(notes) +
			bgfg(bg, cAddSign).Render(add) + bgfg(bg, cDelSign).Render(del)
		out = append(out, fitStyled(line, w, bgfg(bg, cSideFg)))
	}
	return out
}

// changeBar draws the bar of a diff Pane h cells high: a mark per change,
// and a lighter background over the rows on screen.
func changeBar(d *screen.DiffInfo, h int) []string {
	kinds := barKinds(d.Rows, h, d.Hunks)
	out := make([]string, h)
	for i, k := range kinds {
		lo, hi := barRange(i, h, d.Rows)
		bg := cEditorBg
		if hi > d.Top && lo < d.Top+h {
			bg = cBarView
		}
		switch k {
		case screen.MarkAdded:
			out[i] = bgfg(bg, cAddSign).Render("▐")
		case screen.MarkDeleted:
			out[i] = bgfg(bg, cDelSign).Render("▐")
		case screen.MarkModified:
			out[i] = bgfg(bg, cModSign).Render("▐")
		default:
			out[i] = bgfg(bg, cEditorFg).Render(" ")
		}
	}
	return out
}

// drawDiffHeader draws the header row of a diff: the file name with what was
// added and deleted, the review note and the hint, w cells wide.
func drawDiffHeader(title screen.Title, hint string, w int) string {
	name := " " + title.Text
	stats := fmt.Sprintf("  +%d", title.Added)
	dels := fmt.Sprintf(" -%d", title.Deleted)
	noteFg := curTheme.Renamed
	if title.NoteWarn {
		noteFg = curTheme.DiagWarn
	}
	hw := 0
	if hint != "" {
		hw = ansi.StringWidth(hint) + 1
	}
	nw := max(0, min(ansi.StringWidth(name), w-hw-ansi.StringWidth(stats+dels)))
	head := bgfg(cTabBarBg, curTheme.Strong).Bold(true).Render(ansi.Truncate(name, nw, "…")) +
		bgfg(cTabBarBg, cAddSign).Render(stats) + bgfg(cTabBarBg, cDelSign).Render(dels) +
		bgfg(cTabBarBg, noteFg).Render(title.Note)
	return fitStyled(head, max(0, w-hw), bgfg(cTabBarBg, curTheme.Strong)) +
		bgfg(cTabBarBg, curTheme.Dim).Render(fit(" "+hint, hw))
}

// drawDiff draws the rows of the Panes of a diff (the last one is the current
// side and has the change bar) as g.H lines.
func drawDiff(panes []screen.Pane, g core.DiffGeom, o editorOpts) []string {
	cur := panes[len(panes)-1]
	two := len(panes) == 2
	bar := changeBar(cur.Diff, g.H)
	ld := newLineDrawer(cur, g.RightW, o)
	width := g.RightW
	if two {
		width += g.LeftW + 1
	}
	out := make([]string, 0, g.H)
	for i := range g.H {
		if i >= len(cur.Lines) {
			out = append(out, stEmpty.Render(fit("", width))+bar[i])
			continue
		}
		pl := cur.Lines[i]
		var line string
		if pl.Blank || isDeletedLine(pl) { // the deleted lines of a unified diff
			line = drawOldLine(cur, pl, g.RightW)
		} else {
			line = ld.draw(pl)
		}
		if two {
			line = drawOldLine(panes[0], panes[0].Lines[i], g.LeftW) + stSeparator.Render("│") + line
		}
		out = append(out, line+bar[i])
	}
	return out
}

// drawOldLine draws pl, a line of the base (or the space where the base has
// none), w cells wide.
func drawOldLine(p screen.Pane, pl screen.Line, w int) string {
	if pl.Blank {
		return bgfg(cSideBg, cLineNo).Render(fit("", w))
	}
	gw := gutterWidthFor(p.Viewport.Total)
	bg, sc, sign := cEditorBg, cLineNo, " "
	if isDeletedLine(pl) {
		bg, sc, sign = cDelBg, cDelSign, "-"
	}
	gutter := bgfg(bg, sc).Render(sign) + bgfg(bg, cLineNo).Render(fmt.Sprintf("%*d  ", gw-3, pl.No+1))
	return gutter + drawSpans(pl.Spans, bg, pl.Words, cDelWordBg, w-gw, p.Viewport.Left)
}

// drawCompareBar draws the line above everything in diff mode: what is
// compared with what. pr is the pull request whose base is compared with, nil
// for HEAD; branch is the one checked out.
func drawCompareBar(branch string, pr *prInfo, w int) string {
	bg := cTabBarBg
	dim := bgfg(bg, curTheme.Dim)
	text := bgfg(bg, cEditorFg)
	strong := bgfg(bg, curTheme.Strong).Bold(true)
	var sb strings.Builder
	sb.WriteString(text.Render(" "))
	tree := i18n.T("gd.worktree")
	if branch != "" {
		tree = fmt.Sprintf(i18n.T("gd.worktreebranch"), branch)
	}
	if pr != nil {
		sb.WriteString(bgfg(curTheme.Renamed, curTheme.CursorFg).Bold(true).Render(fmt.Sprintf(" PR #%d ", pr.Number)))
		sb.WriteString(strong.Render(" " + pr.Title))
		state, col := pr.State, cAddSign
		switch {
		case pr.IsDraft && pr.State == "OPEN":
			state, col = "DRAFT", curTheme.Dim
		case pr.State == "MERGED":
			col = curTheme.Renamed
		case pr.State == "CLOSED":
			col = cDelSign
		}
		sb.WriteString(text.Render("  ") + bgfg(bg, col).Bold(true).Render(state))
		if pr.Author.Login != "" {
			sb.WriteString(dim.Render("  @" + pr.Author.Login))
		}
		sb.WriteString(dim.Render(i18n.T("gd.comparing")) + strong.Render(pr.BaseRefName) + text.Render(i18n.T("gd.prbase")+tree))
	} else {
		sb.WriteString(bgfg(cAccent, curTheme.AccentFg).Bold(true).Render(" DIFF "))
		sb.WriteString(dim.Render(i18n.T("gd.comparing")) + strong.Render("HEAD") + text.Render(" ↔ "+tree))
	}
	return fitStyled(sb.String(), w, text)
}

// drawDiffHelp draws the keys of diff mode (diffHelp) on the line below it.
func drawDiffHelp(help string, w int) string {
	return bgfg(cSideBg, curTheme.Dim).Render(fit(help, w))
}

// changedLine reports whether pl is a line of a change on the current side of
// a diff (drawn on the added background).
func changedLine(pl screen.Line) bool {
	for _, m := range pl.Marks {
		if m.Kind == screen.MarkAdded || m.Kind == screen.MarkModified {
			return true
		}
	}
	return false
}

func isDeletedLine(pl screen.Line) bool {
	for _, m := range pl.Marks {
		if m.Kind == screen.MarkDeleted {
			return true
		}
	}
	return false
}

// drawSpans draws spans on bg, w cells wide. The runes inside words get
// wordBg instead. The first skip cells of the line are left out (a sideways
// scroll).
func drawSpans(spans []screen.Span, bg string, words []screen.Cols, wordBg string, w, skip int) string {
	if w <= 0 {
		return ""
	}
	var sb strings.Builder
	used, idx, col := 0, 0, 0 // cells drawn, rune index into the line, cells passed
	var run strings.Builder
	var runKey cellStyle
	flush := func() {
		if run.Len() > 0 {
			sb.WriteString(styleFor(runKey).Render(run.String()))
			run.Reset()
		}
	}
	// emit draws s in fg and reports whether there is room for more.
	emit := func(s, fg string) bool {
		// by grapheme cluster, measured as the terminal draws it
		for g := uniseg.NewGraphemes(s); g.Next(); {
			cell := bg
			for _, wd := range words {
				if idx >= wd.Start && idx < wd.End {
					cell = wordBg
					break
				}
			}
			idx += len(g.Runes())
			ch := g.Str()
			if ch == "\t" {
				ch = "    "
			}
			cw := ansi.StringWidth(ch)
			if cw == 0 && skip > 0 && used == 0 { // a combining mark of the rune left of the view
				continue
			}
			if col < skip { // left of the view; a wide rune cut by the edge leaves blanks
				col += cw
				if col <= skip {
					continue
				}
				ch = strings.Repeat(" ", col-skip)
				cw = col - skip
			}
			if used+cw > w {
				return false
			}
			if k := (cellStyle{fg: fg, bg: cell}); k != runKey {
				flush()
				runKey = k
			}
			run.WriteString(ch)
			used += cw
		}
		return used < w
	}
	for _, sp := range spans {
		fg := styleColor(sp.Style)
		if fg == "" {
			fg = cEditorFg
		}
		if !emit(sp.Text, fg) {
			break
		}
	}
	flush()
	if used < w {
		sb.WriteString(styleFor(cellStyle{fg: cEditorFg, bg: bg}).Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}

// highlightText draws text with syntax colors on bg, w cells wide (lexSpans
// drawn by drawSpans). The runes inside spans get markBg instead.
func highlightText(text string, lex chroma.Lexer, bg string, spans []diff.Span, markBg string, w, skip int) string {
	return drawSpans(lexSpans(text, lex), bg, colsOf(spans), markBg, w, skip)
}
