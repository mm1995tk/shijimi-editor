package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/screen"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

// cardOverlay is the open card as it shows in layout l; ok is false when the
// editor area is too narrow for it.
func (a *App) cardOverlay(l layout) (ov screen.Overlay, ok bool) {
	cols, maxH, ok := core.CardSize(l.rightW, l.editH)
	if !ok {
		return screen.Overlay{}, false
	}
	return a.CardOverlay(a.OpenCard(), cols, maxH), true
}

// fitCard keeps the open card's scroll inside its rows in layout l and sends
// the images in view to the terminal (see cardBox).
func (a *App) fitCard(l layout) {
	if !a.CardOpen() {
		return
	}
	cols, maxH, ok := core.CardSize(l.rightW, l.editH)
	if !ok {
		return
	}
	a.FitCard(cols, maxH)
	ov := a.CardOverlay(a.OpenCard(), cols, maxH)
	a.sendMdImages(ov.Images, ov.View.Cols)
}

// cardBox lays out the open card for layout l.
func (a *App) cardBox(l layout) (rows []string, x, y int) {
	c := a.OpenCard()
	ov, ok := a.cardOverlay(l)
	if !ok {
		return nil, 0, 0
	}
	rows = a.drawCard(ov)
	cx, cy := l.rightX+2, l.editY+1
	if c.At != nil {
		cx, cy = c.At.X, c.At.Y
	} else if t := a.CurText(); t != nil && t.Buf.Path != "" && a.RepoPath(t.Buf.Path) == c.Path {
		cx, cy = a.cursorScreen(t, l)
		if t.Buf.Cur.Row+1 != c.Thread.Line {
			cx, cy = l.rightX+2, l.editY+1
		}
	}
	x, y = popupPos(l, a.w, max(cx-4, l.rightX), cy, rows)
	return rows, x, y
}

// composerWidth is the width of the comment editor's box in layout l, and of
// the text area inside it.
func composerWidth(l layout) (w, iw int) {
	w = min(96, l.rightW-2)
	if w < 24 {
		w = max(10, l.rightW)
	}
	return w, w - 4
}

// composerParts is the open comment editor as it shows in layout l: the box's
// width, the text area as the widget draws it and the rest as an Overlay.
func (a *App) composerParts(l layout) (w int, input []string, ov screen.Overlay) {
	w, iw := composerWidth(l)
	input = composerRows(a.OpenComposer().Ta)
	prevH := max(0, l.editH-(len(input)+2)-1)
	return w, input, a.ComposerOverlay(iw, min(prevH, 12))
}

// fitComposer gives the open comment editor's text area the width it is drawn
// in and sends the images of its preview to the terminal (see composerBox).
func (a *App) fitComposer(l layout) {
	if !a.Composing() {
		return
	}
	_, iw := composerWidth(l)
	a.OpenComposer().Ta.SetWidth(iw)
	if _, _, ov := a.composerParts(l); ov.Compose != nil && !ov.Compose.PreviewLabel.IsZero() {
		a.sendMdImages(ov.Images, ov.View.Cols)
	}
}

// composerBox draws the editor over the bottom of the editor area. The text
// area's width was set by fitComposer.
func (a *App) composerBox(l layout) (rows []string, x, y int) {
	w, input, ov := a.composerParts(l)
	rows = a.drawComposer(ov, input)
	x = l.rightX + max(0, (l.rightW-w)/2)
	y = max(l.editY, l.editY+l.editH-len(rows))
	return rows, x, y
}

// markerAt returns the thread whose marker is at screen cell (x, y), if any.
func (a *App) markerAt(l layout, x, y int) *core.PrThread {
	t := a.CurText()
	if t == nil || !a.PrShown() {
		return nil
	}
	dv := a.DiffFor(t)
	if dv == nil {
		return nil
	}
	g := a.dvGeom(l)
	if x != g.RightX || y < g.Top || y >= g.Top+g.H {
		return nil
	}
	idx := dv.Start(t.Buf.ScrollRow) + y - g.Top
	if idx >= len(dv.Rows) || dv.Rows[idx].New < 0 {
		return nil
	}
	p := a.RepoPath(t.Buf.Path)
	if a.IsStale(p) {
		return nil
	}
	if ts := a.LineThreads(p, dv.Rows[idx].New+1); len(ts) > 0 {
		return ts[0]
	}
	return nil
}

// composerCaret is the screen cell of the comment editor's cursor, where an
// IME shows its composition.
func (a *App) composerCaret(l layout) (x, y int, ok bool) {
	rows, bx, by := a.composerBox(l)
	cx, cy, ok := a.ComposerArea().CursorCell()
	if !ok {
		return 0, 0, false
	}
	x, y = bx+2+cx, by+1+cy
	return x, y, y < by+len(rows) && x < a.w
}

// drawCard draws a review card Overlay as a bordered box ov.View.Cols+4
// cells wide: the content rows, the hint on the bottom border and, when the
// card scrolls, where the rows are among all of them.
func (a *App) drawCard(ov screen.Overlay) []string {
	vw := *ov.View // CardOverlay always sets it
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
	if hw, in := ansi.StringWidth(tr(ov.Hint)), ansi.StringWidth(ind); hw+in+2 <= iw {
		bottom = "─" + tr(ov.Hint) + strings.Repeat("─", iw-hw-in-1) + ind
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
	title := " " + tr(ov.Title) + " "
	top := "╭─" + ansi.Truncate(title, w-4, "…")
	top += strings.Repeat("─", max(0, w-1-ansi.StringWidth(top))) + "╮"
	rows := []string{bd.Render(top)}
	for _, l := range input {
		rows = append(rows, line(l))
	}
	rows = append(rows, bd.Render("├"+core.Fit(tr(ov.Hint), w-2)+"┤"))
	if label := tr(ov.Compose.PreviewLabel); label != "" {
		prev := append([]string{stPanelDim.Render(core.Fit(label, iw))}, a.drawMdRows(ov.Lines, 0, ov.Images, iw)...)
		for _, p := range prev {
			rows = append(rows, bd.Render("│")+stEmpty.Render(" ")+fitStyled(p, iw, stEmpty)+stEmpty.Render(" ")+bd.Render("│"))
		}
	}
	return append(rows, bd.Render("╰"+strings.Repeat("─", w-2)+"╯"))
}

// composerRows is the text of the comment editor's input as the box shows it:
// the rows the core's text area gives, in the colours of the palette, with the
// placeholder in the first row while there is no text.
func composerRows(ta *core.TextArea) []string {
	base := bgfg(cPaletteBg, cEditorFg)
	rows := ta.Rows()
	for i := range rows {
		rows[i] = base.Render(rows[i])
	}
	if ta.Value() == "" {
		rows[0] = bgfg(cPaletteBg, core.CurTheme.Dim).Render(i18n.T("rv.markdownok"))
	}
	return rows
}
