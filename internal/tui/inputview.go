package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

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
func (l *lineInput) view(st inputStyle) string {
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

func (l *lineInput) caretCell(st inputStyle, plain lipgloss.Style, s string) string {
	if l.focused {
		return st.cursor.Inline(true).Render(s)
	}
	return plain.Inline(true).Render(s)
}

// placeholderView draws the hint of an empty field with the caret on its first
// character, padded to the field's width plus the caret's cell.
func (l *lineInput) placeholderView(st inputStyle) string {
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
