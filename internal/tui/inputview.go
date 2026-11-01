package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// inputField is a lineInput as the TUI shows it: in a number of cells, with
// the part of the text that is in view once it is wider than that. The text and
// the caret are the lineInput's, in runes; the cells are the TUI's.
type inputField struct {
	lineInput

	width int // cells in view; 0 or less means no scrolling

	// offset and offsetRight are the runes in view, value[offset:offsetRight],
	// once the text is wider than width.
	offset, offsetRight int
}

// newInputField is an empty field that holds at most limit characters (0: any).
func newInputField(limit int) inputField { return inputField{lineInput: newLineInput(limit)} }

func (f *inputField) SetWidth(w int) { f.width = w }

// The edits keep the caret in view.

func (f *inputField) SetValue(s string) { f.lineInput.SetValue(s); f.scroll() }
func (f *inputField) SetCursor(pos int) { f.lineInput.SetCursor(pos); f.scroll() }
func (f *inputField) CursorStart()      { f.SetCursor(0) }
func (f *inputField) CursorEnd()        { f.SetCursor(len(f.value)) }
func (f *inputField) Insert(s string)   { f.lineInput.Insert(s); f.scroll() }
func (f *inputField) Update(in Input)   { f.lineInput.Update(in); f.scroll() }

// Caret is how many cells after the start of the view the caret is.
func (f *inputField) Caret() int {
	f.scroll()
	return uniseg.StringWidth(string(f.value[f.offset:max(f.offset, f.pos)]))
}

// scroll keeps the caret in view when the text is wider than the field.
func (f *inputField) scroll() {
	if f.width <= 0 || uniseg.StringWidth(string(f.value)) <= f.width {
		f.offset = 0
		f.offsetRight = len(f.value)
		return
	}
	f.offsetRight = min(f.offsetRight, len(f.value))
	switch {
	case f.pos < f.offset:
		f.offset = f.pos
		w, i := 0, 0
		runes := f.value[f.offset:]
		for i < len(runes) && w <= f.width {
			w += runewidth.RuneWidth(runes[i])
			if w <= f.width+1 {
				i++
			}
		}
		f.offsetRight = f.offset + i
	case f.pos >= f.offsetRight:
		f.offsetRight = f.pos
		w := 0
		runes := f.value[:f.offsetRight]
		i := len(runes) - 1
		for i > 0 && w < f.width {
			w += runewidth.RuneWidth(runes[i])
			if w <= f.width {
				i--
			}
		}
		f.offset = f.offsetRight - (len(runes) - 1 - i)
	}
}

// inputStyle is how a lineInput is drawn: the text, the hint of an empty field
// and the caret cell. It is built from the current theme each time a field is
// drawn, so a theme change needs no restyling.
type inputStyle struct {
	text, placeholder, cursor lipgloss.Style
}

// inputStyleOn is the style of a field drawn on background bg.
func inputStyleOn(bg string) inputStyle {
	return inputStyle{
		text:        bgfg(bg, cEditorFg),
		placeholder: bgfg(bg, curTheme.Dim),
		// The caret cell is reversed, so the cursor colour as the foreground
		// ends up as the cell background.
		cursor: lipgloss.NewStyle().Foreground(lipgloss.Color(curTheme.Cursor)).Reverse(true),
	}
}

// view draws the field in the cells it was given with SetWidth, and one more
// for the caret at the end. The caret is shown only while the field has focus.
func (l *inputField) view(st inputStyle) string {
	if len(l.value) == 0 && l.Placeholder != "" {
		return l.placeholderView(st)
	}
	value := l.value[l.offset:l.offsetRight]
	pos := max(0, l.pos-l.offset)
	var b strings.Builder
	b.WriteString(st.text.Inline(true).Render(string(value[:pos])))
	caret := " "
	if pos < len(value) {
		caret = string(value[pos])
	}
	b.WriteString(l.caretCell(st, st.text, caret))
	if pos < len(value) {
		b.WriteString(st.text.Inline(true).Render(string(value[pos+1:])))
	}
	if w := uniseg.StringWidth(string(value)); l.width > 0 && w <= l.width {
		// pad to width, plus the caret's cell when it sits on a character
		pad := l.width - w
		if pos < len(value) {
			pad++
		}
		b.WriteString(st.text.Inline(true).Render(strings.Repeat(" ", pad)))
	}
	return b.String()
}

func (l *inputField) caretCell(st inputStyle, plain lipgloss.Style, s string) string {
	if l.focused {
		return st.cursor.Inline(true).Render(s)
	}
	return plain.Inline(true).Render(s)
}

// placeholderView draws the hint of an empty field with the caret on its first
// character, padded to the field's width plus the caret's cell.
func (l *inputField) placeholderView(st inputStyle) string {
	rs := []rune(l.Placeholder)
	rest := string(rs[1:])
	first := string(rs[0])
	if l.width > 0 {
		room := l.width + 1 - ansi.StringWidth(first)
		rest = ansi.Truncate(rest, max(0, room), "")
		rest += strings.Repeat(" ", max(0, room-ansi.StringWidth(rest)))
	}
	return l.caretCell(st, st.placeholder, first) + st.placeholder.Inline(true).Render(rest)
}
