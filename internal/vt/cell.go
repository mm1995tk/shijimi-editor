// Package vt is a small terminal emulator: it turns the bytes a program writes
// to a PTY into a grid of cells with a scrollback, and encodes mouse input for
// programs that ask for it. Control sequences are tokenized by
// charmbracelet/x/ansi; everything else lives here.
package vt

// Color is a terminal color. The zero value is the default foreground or
// background.
type Color uint32

const (
	colorIndexed Color = 1 << 24
	colorRGB     Color = 2 << 24
	colorKind          = Color(0xff) << 24
)

// Indexed returns the color at palette index i (0-255).
func Indexed(i int) Color { return colorIndexed | Color(i&0xff) }

// RGB returns a 24-bit color.
func RGB(r, g, b uint8) Color {
	return colorRGB | Color(r)<<16 | Color(g)<<8 | Color(b)
}

// IsDefault reports whether c is the default color.
func (c Color) IsDefault() bool { return c == 0 }

// Index returns the palette index when c is an indexed color.
func (c Color) Index() (int, bool) {
	if c&colorKind != colorIndexed {
		return 0, false
	}
	return int(c & 0xff), true
}

// Components returns the red, green and blue values when c is a 24-bit color.
func (c Color) Components() (r, g, b uint8, ok bool) {
	if c&colorKind != colorRGB {
		return 0, 0, 0, false
	}
	return uint8(c >> 16), uint8(c >> 8), uint8(c), true
}

// Attr is a set of text attributes.
type Attr uint8

const (
	AttrBold Attr = 1 << iota
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrReverse
	AttrStrike
)

// Cell is one character cell. A double-width character occupies two cells: the
// first has Width 2 and holds the character, the second has Width 0.
type Cell struct {
	Rune   rune   // 0 for an empty cell
	Comb   string // combining marks that follow Rune
	Width  int8   // 1, 2, or 0 for the right half of a wide character
	FG, BG Color
	Attr   Attr
}

// Text returns the characters in the cell as a string. An empty cell and the
// right half of a wide character give "".
func (c Cell) Text() string {
	if c.Rune == 0 || c.Width == 0 {
		return ""
	}
	return string(c.Rune) + c.Comb
}

func blank(bg Color) Cell { return Cell{Width: 1, BG: bg} }

func blankLine(n int, bg Color) []Cell {
	l := make([]Cell, n)
	for i := range l {
		l[i] = blank(bg)
	}
	return l
}
