package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// dvGeom is where the parts of a diff view sit on screen.
type dvGeom struct {
	top, h         int // first content row and number of content rows
	leftW          int // width of the base pane (0 when unified)
	rightX, rightW int // the editable pane
	barX           int // the change bar at the right edge
}

func (a *App) dvGeom(l layout) dvGeom {
	w := max(0, l.rightW-1) // the last column is the change bar
	g := dvGeom{top: l.editY + 1, h: max(0, l.editH-1), rightX: l.rightX, rightW: w, barX: l.rightX + w}
	if a.splitShown() {
		g.leftW = a.diffLeftWidth(w)
		g.rightX = l.rightX + g.leftW + 1
		g.rightW = w - g.leftW - 1
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
		x += g.rightX - l.rightX
		y = g.top + dv.newIdx[min(b.Cur.Row, len(dv.newIdx)-1)] - dv.start(b.ScrollRow)
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
	gTop, gLeft := g.top-l.top, g.rightX-l.rightX
	col = clamp(col, gLeft, gLeft+g.rightW-1)
	if row < gTop {
		b.MoveLines(-1, extend)
	} else if row >= gTop+g.h {
		b.MoveLines(1, extend)
	}
	idx := clamp(dv.start(b.ScrollRow)+clamp(row-gTop, 0, max(0, g.h-1)), 0, len(dv.rows)-1)
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
	a.follow = true
}
