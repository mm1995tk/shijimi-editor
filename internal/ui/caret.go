package ui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
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
		return (a.w-pw)/2 + ansi.StringWidth(a.pal.label()) + inputCaret(a.pal.input), 2, true
	case a.findOpen:
		return l.rightX + ansi.StringWidth(findLabel) + inputCaret(a.findInput), l.editY - 1, true
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
		return x, y, a.cursorShown(t, l, x, y)
	}
	return 0, 0, false
}

// inputCaret is how many cells after the start of ti's view the typed text
// goes. textinput's own Cursor counts runes, which is wrong for wide
// characters, and how far the view is scrolled is not exported. So a copy
// without colours is drawn, where the cursor is the only reversed text.
func inputCaret(ti textinput.Model) int {
	ti.SetStyles(textinput.Styles{})
	v := ti.View()
	i := strings.Index(v, "\x1b[7m")
	if i < 0 {
		return 0
	}
	return ansi.StringWidth(v[:i])
}
