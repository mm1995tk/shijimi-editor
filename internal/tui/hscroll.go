package tui

import (
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
	textW := max(1, a.textCols(t)-gutterWidth(b))
	widest := 0
	for _, ln := range b.Lines {
		widest = max(widest, buffer.VisualCol(ln, len(ln)))
	}
	if dv := a.diffFor(t); dv != nil { // the base side scrolls with it
		if dv.split && !dv.single { // the narrower base side decides where the end shows
			textW = max(1, min(textW, a.diffBaseCols()-gutterWidthFor(len(dv.old))))
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
	textW := max(1, a.textCols(t)-gutterWidth(t.buf))
	a.scrollCols(t, dir*max(1, textW/2))
}
