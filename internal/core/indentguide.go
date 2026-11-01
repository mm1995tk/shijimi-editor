package core

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

// indentUnitWidth is the screen width of one indent level (at least 1).
func indentUnitWidth(b *buffer.Buffer) int {
	w := 0
	for _, r := range b.IndentUnit() {
		w += buffer.RuneWidth(r, w)
	}
	return max(1, w)
}

// leadingWidth is the screen width of the whitespace at the start of line,
// and whether the line has nothing but whitespace.
func leadingWidth(line []rune) (w int, blank bool) {
	for _, r := range line {
		switch r {
		case ' ':
			w++
		case '\t':
			w += buffer.RuneWidth(r, w)
		default:
			return w, false
		}
	}
	return w, true
}

// indentGuideWidth is how many columns of row get indent guides: the width of
// its leading whitespace, or for a blank line the deeper of the nearest
// non-blank lines above and below.
func indentGuideWidth(b *buffer.Buffer, row int) int {
	w, blank := leadingWidth(b.Lines[row])
	if !blank {
		return w
	}
	deepest := 0
	for r := row - 1; r >= 0; r-- {
		if lw, bl := leadingWidth(b.Lines[r]); !bl {
			deepest = lw
			break
		}
	}
	for r := row + 1; r < len(b.Lines); r++ {
		if lw, bl := leadingWidth(b.Lines[r]); !bl {
			deepest = max(deepest, lw)
			break
		}
	}
	return deepest
}
