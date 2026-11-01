package core

import (
	"slices"
	"unicode"
)

// lineInput is a one-line text field: the text and the caret in it. It
// replaces bubbles' textinput.Model (the find bar,
// the ":" line, the palette and the search panel's fields) and reads Input, so
// it does not depend on a terminal library or on cell widths. How wide it is,
// which part of it is in view and how it is drawn are the TUI's
// (inputview.go); the GUI draws it from the text and the caret.
//
// The keys are textinput's defaults, so they work as they did.
type lineInput struct {
	// Placeholder is shown, with the caret on its first character, while the
	// text is empty.
	Placeholder string
	// limit is the most characters the field holds. 0 means no limit.
	limit int

	value   []rune
	Pos     int // caret, in runes
	focused bool
}

// newLineInput is an empty field that holds at most limit characters (0: any).
func newLineInput(limit int) lineInput { return lineInput{limit: limit} }

func (l *lineInput) Value() string { return string(l.value) }
func (l *lineInput) Position() int { return l.Pos }
func (l *lineInput) Focused() bool { return l.focused }
func (l *lineInput) Focus()        { l.focused = true }
func (l *lineInput) Blur()         { l.focused = false }
func (l *lineInput) CursorStart()  { l.SetCursor(0) }
func (l *lineInput) CursorEnd()    { l.SetCursor(len(l.value)) }

// SetValue replaces the text. The caret stays where it was unless that is past
// the new end (or the field was empty with the caret at the start), where it
// goes to the end.
func (l *lineInput) SetValue(s string) {
	l.setRunes(sanitizeLine([]rune(s)))
}

func (l *lineInput) setRunes(r []rune) {
	empty := len(l.value) == 0
	if l.limit > 0 && len(r) > l.limit {
		r = r[:l.limit]
	}
	l.value = r
	if (l.Pos == 0 && empty) || l.Pos > len(l.value) {
		l.SetCursor(len(l.value))
	}
}

// SetCursor moves the caret, clamped to the text.
func (l *lineInput) SetCursor(pos int) {
	l.Pos = min(len(l.value), max(0, pos))
}

// sanitizeLine turns tabs and line breaks into spaces and drops other control
// characters, so the text stays on one line.
func sanitizeLine(r []rune) []rune {
	out := make([]rune, 0, len(r))
	for _, c := range r {
		switch {
		case c == '�':
		case c == '\r' || c == '\n' || c == '\t':
			out = append(out, ' ')
		case unicode.IsControl(c):
		default:
			out = append(out, c)
		}
	}
	return out
}

// Insert types s at the caret.
func (l *lineInput) Insert(s string) {
	paste := sanitizeLine([]rune(s))
	if l.limit > 0 {
		room := l.limit - len(l.value)
		if room <= 0 {
			return
		}
		paste = paste[:min(room, len(paste))]
	}
	v := slices.Concat(l.value[:l.Pos], paste, l.value[l.Pos:])
	l.Pos += len(paste)
	l.setRunes(v)
}

// Update applies a key or a paste.
func (l *lineInput) Update(in Input) {
	if !l.focused {
		return
	}
	switch in := in.(type) {
	case Key:
		l.key(in)
	case Paste:
		l.Insert(in.Content)
	}
}

func (l *lineInput) key(k Key) {
	switch k.String() {
	case "alt+backspace", "ctrl+w", "ctrl+backspace":
		l.deleteWordBackward()
	case "backspace", "ctrl+h":
		if l.Pos > 0 {
			l.value = slices.Delete(l.value, l.Pos-1, l.Pos)
			l.SetCursor(l.Pos - 1)
		}
	case "alt+left", "ctrl+left", "alt+b":
		l.wordBackward()
	case "left", "ctrl+b":
		if l.Pos > 0 {
			l.SetCursor(l.Pos - 1)
		}
	case "alt+right", "ctrl+right", "alt+f":
		l.wordForward()
	case "right", "ctrl+f":
		if l.Pos < len(l.value) {
			l.SetCursor(l.Pos + 1)
		}
	case "home", "ctrl+a":
		l.CursorStart()
	case "delete", "ctrl+d":
		if l.Pos < len(l.value) {
			l.value = slices.Delete(l.value, l.Pos, l.Pos+1)
		}
	case "end", "ctrl+e":
		l.CursorEnd()
	case "ctrl+k":
		l.value = l.value[:l.Pos]
		l.CursorEnd()
	case "ctrl+u":
		l.value = l.value[l.Pos:]
		l.SetCursor(0)
	case "alt+delete", "alt+d", "ctrl+delete":
		l.deleteWordForward()
	case "ctrl+v", "up", "down", "ctrl+n", "ctrl+p":
		// ctrl+v is the App's (it reads the clipboard); the rest are
		// textinput's suggestion keys, which nothing here uses.
	default:
		l.Insert(k.Text)
	}
}

func (l *lineInput) deleteWordBackward() {
	if l.Pos == 0 || len(l.value) == 0 {
		return
	}
	old := l.Pos
	i := l.Pos - 1
	for i > 0 && unicode.IsSpace(l.value[i]) {
		i--
	}
	for i > 0 {
		if unicode.IsSpace(l.value[i]) {
			i++ // keep the space before the word
			break
		}
		i--
	}
	l.value = slices.Delete(l.value, i, old)
	l.SetCursor(i)
}

func (l *lineInput) deleteWordForward() {
	if l.Pos >= len(l.value) {
		return
	}
	old := l.Pos
	i := l.Pos + 1
	for i < len(l.value) && unicode.IsSpace(l.value[i]) {
		i++
	}
	for i < len(l.value) && !unicode.IsSpace(l.value[i]) {
		i++
	}
	l.value = slices.Delete(l.value, old, i)
	l.SetCursor(old)
}

func (l *lineInput) wordBackward() {
	i := l.Pos
	for i > 0 && unicode.IsSpace(l.value[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(l.value[i-1]) {
		i--
	}
	l.SetCursor(i)
}

func (l *lineInput) wordForward() {
	i := l.Pos
	for i < len(l.value) && unicode.IsSpace(l.value[i]) {
		i++
	}
	for i < len(l.value) && !unicode.IsSpace(l.value[i]) {
		i++
	}
	l.SetCursor(i)
}
