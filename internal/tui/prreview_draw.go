package tui

import "strings"

// cardBox lays out the open card for layout l.
func (a *App) cardBox(l layout) (rows []string, x, y int) {
	c := a.card
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
	c := a.composer
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
	if ts := a.gd.rv.lineThreads(p, dv.rows[idx].new+1); len(ts) > 0 {
		return ts[0]
	}
	return nil
}

// composerCaret is the screen cell of the comment editor's cursor, where an
// IME shows its composition.
func (a *App) composerCaret(l layout) (x, y int, ok bool) {
	rows, bx, by := a.composerBox(l)
	ta := a.composer.ta
	ta.SetVirtualCursor(false)
	c := ta.Cursor()
	if c == nil {
		return 0, 0, false
	}
	x, y = bx+2+c.Position.X, by+1+c.Position.Y
	return x, y, y < by+len(rows) && x < a.w
}
