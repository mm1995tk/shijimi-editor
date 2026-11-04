package core

import (
	"strings"
	"unicode"

	"github.com/mm1995tk/shijimi-editor/core/buffer"
)

// textAreaRows is how many rows of text a TextArea shows at once.
const textAreaRows = 6

// TextArea is the multi-line input of the review comment editor. The core
// edits and wraps it, so every UI gets the same keys and the same text; a UI
// only draws Rows and the cursor cell (CursorCell) in its own way, or draws
// the text itself from Value, Line and Column.
type TextArea struct {
	lines [][]rune
	row   int // cursor line
	col   int // cursor column in runes
	width int // cells per row; 0 until SetWidth
	top   int // first shown wrapped row
	// want is the cell column an up or down move tries to keep.
	want int
}

// NewTextArea is a text area holding initial, with the cursor at its end.
func NewTextArea(initial string) *TextArea {
	t := &TextArea{width: 80}
	t.SetValue(initial)
	return t
}

// Value is the text.
func (t *TextArea) Value() string {
	var sb strings.Builder
	for i, l := range t.lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(l))
	}
	return sb.String()
}

// SetValue replaces the text and puts the cursor at its end.
func (t *TextArea) SetValue(s string) {
	t.lines = [][]rune{{}}
	t.row, t.col, t.top, t.want = 0, 0, 0, -1
	t.InsertString(s)
}

// InsertString inserts s at the cursor. A newline starts a new line.
func (t *TextArea) InsertString(s string) {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	for _, r := range s {
		switch {
		case r == '\n':
			t.newline()
		case r == '\t': // a tab has no width of its own in a row: spaces
			for range buffer.TabWidth {
				t.insertRune(' ')
			}
		case !unicode.IsControl(r):
			t.insertRune(r)
		}
	}
	t.want = -1
	t.fix()
}

func (t *TextArea) insertRune(r rune) {
	l := t.lines[t.row]
	l = append(l, 0)
	copy(l[t.col+1:], l[t.col:])
	l[t.col] = r
	t.lines[t.row] = l
	t.col++
}

func (t *TextArea) newline() {
	l := t.lines[t.row]
	rest := append([]rune(nil), l[t.col:]...)
	t.lines[t.row] = l[:t.col:t.col]
	t.lines = append(t.lines, nil)
	copy(t.lines[t.row+2:], t.lines[t.row+1:])
	t.lines[t.row+1] = rest
	t.row++
	t.col = 0
}

// Line and Column are the cursor's row and column in the text (the column
// counts runes).
func (t *TextArea) Line() int   { return t.row }
func (t *TextArea) Column() int { return t.col }

// SetWidth sets how many cells a row of text may take before it wraps.
func (t *TextArea) SetWidth(w int) {
	if w < 1 {
		w = 1
	}
	if w != t.width {
		t.width = w
		t.fix()
	}
}

// Update takes a Key or a Paste.
func (t *TextArea) Update(in Input) {
	switch in := in.(type) {
	case Paste:
		t.InsertString(in.Content)
	case Key:
		t.key(in)
	}
}

func (t *TextArea) key(k Key) {
	name := k.keystroke()
	if k.Mod&(ModCtrl|ModAlt) == 0 && k.Text != "" {
		t.InsertString(k.Text)
		t.want = -1
		return
	}
	vertical := false
	switch name {
	case "left", "ctrl+b":
		t.left()
	case "right", "ctrl+f":
		t.right()
	case "up", "ctrl+p":
		t.moveRows(-1)
		vertical = true
	case "down", "ctrl+n":
		t.moveRows(1)
		vertical = true
	case "home", "ctrl+a":
		t.col = 0
	case "end", "ctrl+e":
		t.col = len(t.lines[t.row])
	case "alt+left", "ctrl+left", "alt+b":
		t.wordLeft()
	case "alt+right", "ctrl+right", "alt+f":
		t.wordRight()
	case "ctrl+home", "alt+<":
		t.row, t.col = 0, 0
	case "ctrl+end", "alt+>":
		t.row = len(t.lines) - 1
		t.col = len(t.lines[t.row])
	case "backspace", "ctrl+h":
		t.backspace()
	case "delete", "ctrl+d":
		t.deleteRune()
	case "alt+backspace", "ctrl+w":
		from := t.col
		t.wordLeft()
		t.cut(t.row, t.col, from)
	case "alt+delete", "alt+d":
		l := t.lines[t.row]
		to := t.col
		for to < len(l) && unicode.IsSpace(l[to]) {
			to++
		}
		for to < len(l) && !unicode.IsSpace(l[to]) {
			to++
		}
		t.cut(t.row, t.col, to)
	case "ctrl+k":
		t.cut(t.row, t.col, len(t.lines[t.row]))
	case "ctrl+u":
		t.cut(t.row, 0, t.col)
	}
	if !vertical {
		t.want = -1
	}
	t.fix()
}

func (t *TextArea) left() {
	if t.col > 0 {
		t.col--
	} else if t.row > 0 {
		t.row--
		t.col = len(t.lines[t.row])
	}
}

func (t *TextArea) right() {
	if t.col < len(t.lines[t.row]) {
		t.col++
	} else if t.row < len(t.lines)-1 {
		t.row++
		t.col = 0
	}
}

func (t *TextArea) wordLeft() {
	for t.col == 0 && t.row > 0 {
		t.row--
		t.col = len(t.lines[t.row])
	}
	l := t.lines[t.row]
	for t.col > 0 && unicode.IsSpace(l[t.col-1]) {
		t.col--
	}
	for t.col > 0 && !unicode.IsSpace(l[t.col-1]) {
		t.col--
	}
}

func (t *TextArea) wordRight() {
	for t.col == len(t.lines[t.row]) && t.row < len(t.lines)-1 {
		t.row++
		t.col = 0
	}
	l := t.lines[t.row]
	for t.col < len(l) && unicode.IsSpace(l[t.col]) {
		t.col++
	}
	for t.col < len(l) && !unicode.IsSpace(l[t.col]) {
		t.col++
	}
}

// cut removes columns [from, to) of line row and puts the cursor at from.
func (t *TextArea) cut(row, from, to int) {
	if to <= from {
		return
	}
	l := t.lines[row]
	t.lines[row] = append(l[:from:from], l[to:]...)
	t.row, t.col = row, from
}

func (t *TextArea) backspace() {
	if t.col > 0 {
		t.cut(t.row, t.col-1, t.col)
		return
	}
	if t.row == 0 {
		return
	}
	prev := t.lines[t.row-1]
	t.col = len(prev)
	t.lines[t.row-1] = append(prev[:len(prev):len(prev)], t.lines[t.row]...)
	t.lines = append(t.lines[:t.row], t.lines[t.row+1:]...)
	t.row--
}

func (t *TextArea) deleteRune() {
	if t.col < len(t.lines[t.row]) {
		t.cut(t.row, t.col, t.col+1)
		return
	}
	if t.row == len(t.lines)-1 {
		return
	}
	t.lines[t.row] = append(t.lines[t.row][:t.col:t.col], t.lines[t.row+1]...)
	t.lines = append(t.lines[:t.row+1], t.lines[t.row+2:]...)
}

// seg is a wrapped row: columns [from, to) of line.
type seg struct{ line, from, to int }

// segs wraps the text to the width. A line whose last row is full gets an
// empty one after it, where the cursor can stand at its end.
func (t *TextArea) segs() []seg {
	var out []seg
	for i, l := range t.lines {
		from, w := 0, 0
		for j, r := range l {
			rw := buffer.RuneWidth(r, 0)
			if w+rw > t.width && j > from {
				out = append(out, seg{i, from, j})
				from, w = j, 0
			}
			w += rw
		}
		out = append(out, seg{i, from, len(l)})
		if w >= t.width {
			out = append(out, seg{i, len(l), len(l)})
		}
	}
	return out
}

func (t *TextArea) cellWidth(line, from, to int) int {
	w := 0
	for _, r := range t.lines[line][from:to] {
		w += buffer.RuneWidth(r, 0)
	}
	return w
}

// cursorSeg is the index of the wrapped row the cursor is on.
func (t *TextArea) cursorSeg(ss []seg) int {
	for i, s := range ss {
		if s.line != t.row || t.col < s.from {
			continue
		}
		// the end of a full row belongs to the empty row after it
		if t.col < s.to || (t.col == s.to && (i+1 == len(ss) || ss[i+1].line != t.row || ss[i+1].from != s.to || s.from == s.to)) {
			return i
		}
	}
	return 0
}

// moveRows moves the cursor d wrapped rows, keeping its cell column.
func (t *TextArea) moveRows(d int) {
	ss := t.segs()
	i := t.cursorSeg(ss)
	if t.want < 0 {
		t.want = t.cellWidth(ss[i].line, ss[i].from, t.col)
	}
	j := i + d
	if j < 0 || j >= len(ss) {
		return
	}
	s := ss[j]
	t.row, t.col = s.line, s.from
	w := 0
	for t.col < s.to {
		rw := buffer.RuneWidth(t.lines[s.line][t.col], 0)
		if w+rw > t.want {
			break
		}
		w += rw
		t.col++
	}
}

// fix keeps the cursor in the text and in view.
func (t *TextArea) fix() {
	t.row = Clamp(t.row, 0, len(t.lines)-1)
	t.col = Clamp(t.col, 0, len(t.lines[t.row]))
	ss := t.segs()
	i := t.cursorSeg(ss)
	if i < t.top {
		t.top = i
	}
	if i >= t.top+textAreaRows {
		t.top = i - textAreaRows + 1
	}
	t.top = Clamp(t.top, 0, max(0, len(ss)-textAreaRows))
}

// Rows is the text as it shows: textAreaRows wrapped rows from the first one
// in view, the last ones empty when the text is shorter.
func (t *TextArea) Rows() []string {
	ss := t.segs()
	out := make([]string, textAreaRows)
	for i := range out {
		if k := t.top + i; k < len(ss) {
			s := ss[k]
			out[i] = string(t.lines[s.line][s.from:s.to])
		}
	}
	return out
}

// CursorCell is the cursor's cell inside Rows.
func (t *TextArea) CursorCell() (x, y int, ok bool) {
	ss := t.segs()
	i := t.cursorSeg(ss)
	if i < t.top || i >= t.top+textAreaRows {
		return 0, 0, false
	}
	return t.cellWidth(ss[i].line, ss[i].from, t.col), i - t.top, true
}
