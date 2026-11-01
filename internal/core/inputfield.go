package core

import (
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// inputField is a lineInput as the TUI shows it: in a number of cells, with
// the part of the text that is in view once it is wider than that. The text and
// the caret are the lineInput's, in runes; the cells are the TUI's.
type InputField struct {
	lineInput

	Width int // cells in view; 0 or less means no scrolling

	// offset and offsetRight are the runes in view, value[offset:offsetRight],
	// once the text is wider than width.
	offset, offsetRight int
}

// newInputField is an empty field that holds at most limit characters (0: any).
func NewInputField(limit int) InputField { return InputField{lineInput: newLineInput(limit)} }

func (f *InputField) SetWidth(w int) { f.Width = w }

// The edits keep the caret in view.

func (f *InputField) SetValue(s string) { f.lineInput.SetValue(s); f.scroll() }
func (f *InputField) SetCursor(pos int) { f.lineInput.SetCursor(pos); f.scroll() }
func (f *InputField) CursorStart()      { f.SetCursor(0) }
func (f *InputField) CursorEnd()        { f.SetCursor(len(f.value)) }
func (f *InputField) Insert(s string)   { f.lineInput.Insert(s); f.scroll() }
func (f *InputField) Update(in Input)   { f.lineInput.Update(in); f.scroll() }

// Caret is how many cells after the start of the view the caret is.
func (f *InputField) Caret() int {
	f.scroll()
	return uniseg.StringWidth(string(f.value[f.offset:max(f.offset, f.Pos)]))
}

// scroll keeps the caret in view when the text is wider than the field.
func (f *InputField) scroll() {
	if f.Width <= 0 || uniseg.StringWidth(string(f.value)) <= f.Width {
		f.offset = 0
		f.offsetRight = len(f.value)
		return
	}
	f.offsetRight = min(f.offsetRight, len(f.value))
	switch {
	case f.Pos < f.offset:
		f.offset = f.Pos
		w, i := 0, 0
		runes := f.value[f.offset:]
		for i < len(runes) && w <= f.Width {
			w += runewidth.RuneWidth(runes[i])
			if w <= f.Width+1 {
				i++
			}
		}
		f.offsetRight = f.offset + i
	case f.Pos >= f.offsetRight:
		f.offsetRight = f.Pos
		w := 0
		runes := f.value[:f.offsetRight]
		i := len(runes) - 1
		for i > 0 && w < f.Width {
			w += runewidth.RuneWidth(runes[i])
			if w <= f.Width {
				i--
			}
		}
		f.offset = f.offsetRight - (len(runes) - 1 - i)
	}
}

// Runes is the text of the field, in runes.
func (f *InputField) Runes() []rune { return f.value }

// Window is the part of Runes in view, Runes()[from:to], once the text is
// wider than Width.
func (f *InputField) Window() (from, to int) { return f.offset, f.offsetRight }
