package core

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// hscrollWheel is how many columns one horizontal wheel notch scrolls.
const HscrollWheel = 6

// scrollCols moves the view d columns sideways without moving the cursor.
// It stops at the left edge and where the end of the longest line shows.
func (a *Core) ScrollCols(t *Tab, d int) {
	b := t.Buf
	textW := max(1, a.TextCols(t)-GutterWidth(b))
	widest := 0
	for _, ln := range b.Lines {
		widest = max(widest, buffer.VisualCol(ln, len(ln)))
	}
	if dv := a.DiffFor(t); dv != nil { // the base side scrolls with it
		if dv.split && !dv.Single { // the narrower base side decides where the end shows
			textW = max(1, min(textW, a.DiffBaseCols()-GutterWidthFor(len(dv.old))))
		}
		for _, ln := range dv.old {
			widest = max(widest, ansi.StringWidth(strings.ReplaceAll(ln, "\t", "    ")))
		}
	}
	b.ScrollCol = Clamp(b.ScrollCol+d, 0, max(0, widest-textW))
	a.follow = false // the view stays where it is, off the cursor if need be
}

// scrollHalfScreen scrolls dir (-1 left, 1 right) by half the text width,
// the sideways counterpart of Ctrl+U / Ctrl+D.
func (a *Core) scrollHalfScreen(t *Tab, dir int) {
	textW := max(1, a.TextCols(t)-GutterWidth(t.Buf))
	a.ScrollCols(t, dir*max(1, textW/2))
}
