package ui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/dageki/internal/caret"
)

// SetCaret hands the app the output writer that parks the terminal's cursor
// where the user is typing, so an IME shows its composition there.
func (a *App) SetCaret(w *caret.Writer) { a.caret = w }

// updateCaret tells the caret writer where the user is typing in the frame
// View is about to return.
func (a *App) updateCaret(l layout) {
	if a.caret == nil {
		return
	}
	if x, y, ok := a.caretPos(l); ok {
		a.caret.Set(x, y)
	} else {
		a.caret.Clear()
	}
}

// caretPos is the screen cell where typed text goes, if any.
func (a *App) caretPos(l layout) (x, y int, ok bool) {
	switch {
	case a.cmdOpen:
		v := []rune(a.cmdInput.Value())
		pos := clamp(a.cmdInput.Position(), 0, len(v))
		x = ansi.StringWidth(a.cmdKind) + ansi.StringWidth(string(v[:pos]))
		return min(x, a.w-1), a.h - 1, true
	case a.overlayOpen() || a.findOpen:
		return 0, 0, false
	case a.focus == focusTerminal && a.showTerm:
		tab := a.term.cur()
		if tab == nil {
			return 0, 0, false
		}
		sn := &tab.snap
		if sn.Offset != 0 || sn.CursorY < 0 || sn.CursorY >= len(sn.Cells) {
			return 0, 0, false
		}
		// The panel's first two rows are its border and tab bar.
		x, y = l.rightX+sn.CursorX, l.panelY+2+sn.CursorY
		return x, y, x < a.w && y < l.statusY
	case a.focus == focusEditor:
		t := a.curText()
		if t == nil || a.fullPreview(t) {
			return 0, 0, false
		}
		x, y = a.cursorScreen(t, l)
		return x, y, x >= l.rightX && x < a.w && y >= l.editY && y < l.editY+l.editH
	}
	return 0, 0, false
}
