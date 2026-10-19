package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/caret"
)

// SetCaret hands the app the output writer, which keeps the terminal's cursor
// hidden and carries sequences such as the clipboard (OSC 52).
func (a *App) SetCaret(w *caret.Writer) { a.caret = w }

// updateCaret records where the user is typing in the frame View is about to
// return. View hands it to bubbletea as the cursor, so an IME shows its
// composition there.
func (a *App) updateCaret(l layout) {
	a.caretAt = nil
	if x, y, ok := a.caretPos(l); ok {
		a.caretAt = tea.NewCursor(x, y)
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
