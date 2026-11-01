package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// cardBox lays out the open card for layout l.
func (a *App) cardBox(l layout) (rows []string, x, y int) {
	c := a.OpenCard()
	w := min(84, l.rightW-2)
	if w < 20 {
		return nil, 0, 0
	}
	rows = a.drawCard(a.CardOverlay(c, w-4, max(3, l.editH*2/3-2)))
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

// composerBox draws the editor over the bottom of the editor area.
func (a *App) composerBox(l layout) (rows []string, x, y int) {
	c := a.OpenComposer()
	w := min(96, l.rightW-2)
	if w < 24 {
		w = max(10, l.rightW)
	}
	iw := w - 4
	c.Ta.SetWidth(iw)
	input := strings.Split(c.Ta.View(), "\n")
	prevH := max(0, l.editH-(len(input)+2)-1)
	rows = a.drawComposer(a.ComposerOverlay(iw, min(prevH, 12)), input)
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
	rows = append(rows, bd.Render("├"+core.Fit(ov.Hint, w-2)+"┤"))
	if label := ov.Compose.PreviewLabel; label != "" {
		prev := append([]string{stPanelDim.Render(core.Fit(label, iw))}, a.drawMdRows(ov.Lines, 0, ov.Images, iw)...)
		for _, p := range prev {
			rows = append(rows, bd.Render("│")+stEmpty.Render(" ")+fitStyled(p, iw, stEmpty)+stEmpty.Render(" ")+bd.Render("│"))
		}
	}
	return append(rows, bd.Render("╰"+strings.Repeat("─", w-2)+"╯"))
}

// textArea is the review comment editor's input: a bubbles textarea in the
// colours of the palette.
type textArea struct{ ta textarea.Model }

// NewTextArea is a focused text area holding initial (core.FrontEnd).
func (a *App) NewTextArea(initial string) core.TextArea {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetHeight(6)
	styleComposer(&ta)
	ta.Placeholder = i18n.T("rv.markdownok")
	ta.SetValue(initial)
	// Focus hands back the cursor's blink command; its BlinkMsg is never routed to
	// the textarea (see update), so it is dropped.
	ta.Focus()
	return &textArea{ta: ta}
}

// styleComposer gives the comment editor the colours of the palette.
func styleComposer(ta *textarea.Model) {
	st := ta.Styles()
	base := bgfg(cPaletteBg, cEditorFg)
	st.Focused = textarea.StyleState{Base: base, Text: base, CursorLine: base, Placeholder: bgfg(cPaletteBg, core.CurTheme.Dim), EndOfBuffer: base, Prompt: base}
	st.Blurred = st.Focused
	ta.SetStyles(st)
}

func (t *textArea) Value() string         { return t.ta.Value() }
func (t *textArea) SetValue(s string)     { t.ta.SetValue(s) }
func (t *textArea) InsertString(s string) { t.ta.InsertString(s) }
func (t *textArea) Line() int             { return t.ta.Line() }
func (t *textArea) Column() int           { return t.ta.Column() }
func (t *textArea) SetWidth(w int)        { t.ta.SetWidth(w) }
func (t *textArea) View() string          { return t.ta.View() }

// Update passes a key or a paste to the textarea; the blink command it
// returns is not used.
func (t *textArea) Update(in core.Input) { t.ta, _ = t.ta.Update(widgetMsg(in)) }

func (t *textArea) CursorCell() (x, y int, ok bool) {
	ta := t.ta // a copy: the real one keeps its virtual cursor
	ta.SetVirtualCursor(false)
	c := ta.Cursor()
	if c == nil {
		return 0, 0, false
	}
	return c.Position.X, c.Position.Y, true
}
