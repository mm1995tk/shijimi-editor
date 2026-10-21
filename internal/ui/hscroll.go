package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// hscrollWheel is how many columns one horizontal wheel notch scrolls.
const hscrollWheel = 6

// scrollCols moves the view d columns sideways without moving the cursor.
// It stops at the left edge and where the end of the longest line shows.
func (a *App) scrollCols(t *tab, d int) {
	b := t.buf
	textW := max(1, a.editW(t, a.layout())-gutterWidth(b))
	widest := 0
	for _, ln := range b.Lines {
		widest = max(widest, buffer.VisualCol(ln, len(ln)))
	}
	if dv := a.diffFor(t); dv != nil { // the base side scrolls with it
		if dv.split && !dv.single { // the narrower base side decides where the end shows
			textW = max(1, min(textW, a.dvGeom(a.layout()).leftW-(len(fmt.Sprint(len(dv.old)))+4)))
		}
		for _, ln := range dv.old {
			widest = max(widest, ansi.StringWidth(strings.ReplaceAll(ln, "\t", "    ")))
		}
	}
	b.ScrollCol = clamp(b.ScrollCol+d, 0, max(0, widest-textW))
	a.follow = false // the view stays where it is, off the cursor if need be
}

// scrollHalfScreen scrolls dir (-1 left, 1 right) by half the text width,
// the sideways counterpart of Ctrl+U / Ctrl+D.
func (a *App) scrollHalfScreen(t *tab, dir int) {
	textW := max(1, a.editW(t, a.layout())-gutterWidth(t.buf))
	a.scrollCols(t, dir*max(1, textW/2))
}

// inEditorText reports whether (x, y) is over the text of the current text
// tab, where the sideways wheel scrolls it.
func (a *App) inEditorText(l layout, x, y int) bool {
	t := a.cur()
	if t == nil || t.kind != tabText || a.fullPreview(t) || a.showTerm && y >= l.panelY {
		return false
	}
	return y >= l.editY && y < l.editY+l.editH && x >= l.rightX
}

// cursorShown reports whether the cursor drawn at (x, y) is inside the text
// of its pane: sideways scrolling can take it over the line numbers, the
// base side of a split diff or off the edge.
func (a *App) cursorShown(t *tab, l layout, x, y int) bool {
	left, w, top, h := l.rightX, a.editW(t, l), l.editY, l.editH
	if dv := a.diffFor(t); dv != nil {
		g := a.dvGeom(l)
		left, top, h = g.rightX, g.top, g.h
	}
	return x >= left+gutterWidth(t.buf) && x < left+w && x < a.w && y >= top && y < top+h
}
