package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// cardBox lays out the open card for layout l.
func (a *App) cardBox(l layout) (rows []string, x, y int) {
	c := a.openCard()
	w := min(84, l.rightW-2)
	if w < 20 {
		return nil, 0, 0
	}
	rows = a.drawCard(a.cardOverlay(c, w-4, max(3, l.editH*2/3-2)))
	cx, cy := l.rightX+2, l.editY+1
	if c.at != nil {
		cx, cy = c.at.x, c.at.y
	} else if t := a.curText(); t != nil && t.buf.Path != "" && a.repoPath(t.buf.Path) == c.path {
		cx, cy = a.cursorScreen(t, l)
		if t.buf.Cur.Row+1 != c.thread.Line {
			cx, cy = l.rightX+2, l.editY+1
		}
	}
	x, y = popupPos(l, a.w, max(cx-4, l.rightX), cy, rows)
	return rows, x, y
}

// composerBox draws the editor over the bottom of the editor area.
func (a *App) composerBox(l layout) (rows []string, x, y int) {
	c := a.openComposer()
	w := min(96, l.rightW-2)
	if w < 24 {
		w = max(10, l.rightW)
	}
	iw := w - 4
	c.ta.SetWidth(iw)
	input := strings.Split(c.ta.View(), "\n")
	prevH := max(0, l.editH-(len(input)+2)-1)
	rows = a.drawComposer(a.composerOverlay(iw, min(prevH, 12)), input)
	x = l.rightX + max(0, (l.rightW-w)/2)
	y = max(l.editY, l.editY+l.editH-len(rows))
	return rows, x, y
}

// markerAt returns the thread whose marker is at screen cell (x, y), if any.
func (a *App) markerAt(l layout, x, y int) *prThread {
	t := a.curText()
	if t == nil || !a.prShown() {
		return nil
	}
	dv := a.diffFor(t)
	if dv == nil {
		return nil
	}
	g := a.dvGeom(l)
	if x != g.rightX || y < g.top || y >= g.top+g.h {
		return nil
	}
	idx := dv.start(t.buf.ScrollRow) + y - g.top
	if idx >= len(dv.rows) || dv.rows[idx].new < 0 {
		return nil
	}
	p := a.repoPath(t.buf.Path)
	if a.isStale(p) {
		return nil
	}
	if ts := a.lineThreads(p, dv.rows[idx].new+1); len(ts) > 0 {
		return ts[0]
	}
	return nil
}

// composerCaret is the screen cell of the comment editor's cursor, where an
// IME shows its composition.
func (a *App) composerCaret(l layout) (x, y int, ok bool) {
	rows, bx, by := a.composerBox(l)
	ta := a.composerArea()
	ta.SetVirtualCursor(false)
	c := ta.Cursor()
	if c == nil {
		return 0, 0, false
	}
	x, y = bx+2+c.Position.X, by+1+c.Position.Y
	return x, y, y < by+len(rows) && x < a.w
}

// drawCard draws a review card Overlay as a bordered box ov.View.Cols+4
// cells wide: the content rows, the hint on the bottom border and, when the
// card scrolls, where the rows are among all of them.
func (a *App) drawCard(ov screen.Overlay) []string {
	vw := *ov.View // cardOverlay always sets it
	w := vw.Cols + 4
	end := vw.Top + len(ov.Lines)
	bd := bgfg(cEditorBg, cHoverBorder)
	iw := w - 2
	out := []string{bd.Render("╭" + strings.Repeat("─", iw) + "╮")}
	pad := stEmpty.Render(" ")
	for _, l := range a.drawMdRows(ov.Lines, vw.Top, ov.Images, vw.Cols) {
		out = append(out, bd.Render("│")+pad+l+pad+bd.Render("│"))
	}
	ind := ""
	if vw.Total > vw.Rows {
		ind = fmt.Sprintf(" %d/%d ", end, vw.Total)
	}
	bottom := strings.Repeat("─", iw)
	if hw, in := ansi.StringWidth(ov.Hint), ansi.StringWidth(ind); hw+in+2 <= iw {
		bottom = "─" + ov.Hint + strings.Repeat("─", iw-hw-in-1) + ind
	}
	return append(out, bd.Render("╰"+bottom+"╯"))
}

// drawComposer draws a comment editor Overlay as a bordered box ov.View.Cols+4
// cells wide. input is the text field as the widget draws it (its scrolling
// and cursor), which stays until the widget is replaced by our own type.
func (a *App) drawComposer(ov screen.Overlay, input []string) []string {
	iw := ov.View.Cols
	w := iw + 4
	bd := bgfg(cPaletteBg, cHoverBorder)
	in := bgfg(cPaletteBg, cEditorFg)
	line := func(s string) string {
		return bd.Render("│") + in.Render(" ") + fitStyled(s, iw, in) + in.Render(" ") + bd.Render("│")
	}
	title := " " + ov.Title + " "
	top := "╭─" + ansi.Truncate(title, w-4, "…")
	top += strings.Repeat("─", max(0, w-1-ansi.StringWidth(top))) + "╮"
	rows := []string{bd.Render(top)}
	for _, l := range input {
		rows = append(rows, line(l))
	}
	rows = append(rows, bd.Render("├"+fit(ov.Hint, w-2)+"┤"))
	if label := ov.Compose.PreviewLabel; label != "" {
		prev := append([]string{stPanelDim.Render(fit(label, iw))}, a.drawMdRows(ov.Lines, 0, ov.Images, iw)...)
		for _, p := range prev {
			rows = append(rows, bd.Render("│")+stEmpty.Render(" ")+fitStyled(p, iw, stEmpty)+stEmpty.Render(" ")+bd.Render("│"))
		}
	}
	return append(rows, bd.Render("╰"+strings.Repeat("─", w-2)+"╯"))
}

// styleComposer gives the comment editor the colours of the palette.
func styleComposer(ta *textarea.Model) {
	st := ta.Styles()
	base := bgfg(cPaletteBg, cEditorFg)
	st.Focused = textarea.StyleState{Base: base, Text: base, CursorLine: base, Placeholder: bgfg(cPaletteBg, curTheme.Dim), EndOfBuffer: base, Prompt: base}
	st.Blurred = st.Focused
	ta.SetStyles(st)
}
