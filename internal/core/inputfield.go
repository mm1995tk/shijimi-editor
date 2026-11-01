package core

import (
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

