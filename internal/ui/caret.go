package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/caret"
)

// SetCaret hands the app the output writer, which keeps the terminal's cursor
// hidden and carries sequences such as the clipboard (OSC 52).
func (a *App) SetCaret(w *caret.Writer) { a.caret = w }

// SetCellSize hands the app a function that reports the terminal cell size
// in pixels (0, 0 when unknown). Images are sized with it.
func (a *App) SetCellSize(f func() (w, h int)) {
	a.cellSize = f
	updateCellSize(f())
}

// caretCursor is where the user is typing in the frame View is about to
// return, or nil. View hands it to bubbletea as the cursor, so an IME shows
// its composition there.
func (a *App) caretCursor(l layout) *tea.Cursor {
	if x, y, ok := a.caretPos(l); ok {
		return tea.NewCursor(x, y)
	}
	return nil
}

// caretPos is the screen cell where typed text goes, if any.
func (a *App) caretPos(l layout) (x, y int, ok bool) {
	switch {
	case a.composer != nil:
		return a.composerCaret(l)
	case a.cmdOpen:
		v := []rune(a.cmdInput.Value())
		pos := clamp(a.cmdInput.Position(), 0, len(v))
		x = ansi.StringWidth(a.cmdKind) + ansi.StringWidth(string(v[:pos]))
		return min(x, a.w-1), a.h - 1, true
	case a.search.open:
		return a.searchCaret()
	case a.pal.open:
		pw := clamp(a.w*3/5, 30, 100)
		return (a.w-pw)/2 + ansi.StringWidth(a.pal.label()) + a.pal.input.Caret(), 2, true
	case a.findOpen:
		return l.rightX + ansi.StringWidth(findLabel()) + a.findInput.Caret(), l.editY - 1, true
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
		x, y = l.fullX+sn.CursorX, l.panelY+2+sn.CursorY
		return x, y, x < a.w && y < l.statusY
	case a.focus == focusEditor:
		t := a.curText()
		if t == nil || a.fullPreview(t) {
			return 0, 0, false
		}
		x, y = a.cursorScreen(t, l)
		return x, y, a.cursorShown(t, l, x, y)
	}
	return 0, 0, false
}
