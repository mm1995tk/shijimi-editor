package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"github.com/mm1995tk/shijimi-editor/internal/core"
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
		placeholder: bgfg(bg, core.CurTheme.Dim),
		// The caret cell is reversed, so the cursor colour as the foreground
		// ends up as the cell background.
		cursor: lipgloss.NewStyle().Foreground(lipgloss.Color(core.CurTheme.Cursor)).Reverse(true),
	}
}

// view draws the field in the cells it was given with SetWidth, and one more
// for the caret at the end. The caret is shown only while the field has focus.
func viewField(l *core.InputField, st inputStyle) string {
	if len(l.Runes()) == 0 && l.Placeholder != "" {
		return placeholderView(l, st)
	}
	from, to := l.Window()
	value := l.Runes()[from:to]
	pos := max(0, l.Pos-from)
	var b strings.Builder
	b.WriteString(st.text.Inline(true).Render(string(value[:pos])))
	caret := " "
	if pos < len(value) {
		caret = string(value[pos])
	}
	b.WriteString(caretCell(l, st, st.text, caret))
	if pos < len(value) {
		b.WriteString(st.text.Inline(true).Render(string(value[pos+1:])))
	}
	if w := uniseg.StringWidth(string(value)); l.Width > 0 && w <= l.Width {
		// pad to width, plus the caret's cell when it sits on a character
		pad := l.Width - w
		if pos < len(value) {
			pad++
		}
		b.WriteString(st.text.Inline(true).Render(strings.Repeat(" ", pad)))
	}
	return b.String()
}

func caretCell(l *core.InputField, st inputStyle, plain lipgloss.Style, s string) string {
	if l.Focused() {
		return st.cursor.Inline(true).Render(s)
	}
	return plain.Inline(true).Render(s)
}

// placeholderView draws the hint of an empty field with the caret on its first
// character, padded to the field's width plus the caret's cell.
func placeholderView(l *core.InputField, st inputStyle) string {
	rs := []rune(l.Placeholder)
	rest := string(rs[1:])
	first := string(rs[0])
	if l.Width > 0 {
		room := l.Width + 1 - ansi.StringWidth(first)
		rest = ansi.Truncate(rest, max(0, room), "")
		rest += strings.Repeat(" ", max(0, room-ansi.StringWidth(rest)))
	}
	return caretCell(l, st, st.placeholder, first) + st.placeholder.Inline(true).Render(rest)
}
