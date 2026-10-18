package buffer

// Mark is a position that follows edits made to the buffer. A mark with
// Right set moves to the end of text inserted exactly at it; otherwise it
// stays in front of that text.
type Mark struct {
	Pos   Pos
	Right bool
}

// AddMark registers m so its Pos follows later edits.
func (b *Buffer) AddMark(m *Mark) { b.marks = append(b.marks, m) }

// ClearMarks forgets every registered mark.
func (b *Buffer) ClearMarks() { b.marks = nil }

func (b *Buffer) shiftMarks(f func(Pos) Pos, atStart func(Pos) bool) {
	for _, m := range b.marks {
		if !m.Right && atStart(m.Pos) {
			continue
		}
		m.Pos = f(m.Pos)
	}
}

// SelectRanges makes the given [anchor, cursor] pairs the cursor set. The
// first pair becomes the primary cursor. An empty pair (anchor == cursor)
// is a plain cursor without a selection.
func (b *Buffer) SelectRanges(pairs [][2]Pos) {
	if len(pairs) == 0 {
		return
	}
	rs := make([]Range, len(pairs))
	for i, p := range pairs {
		rs[i] = Range{Anchor: p[0], HasAnchor: p[0] != p[1], Cur: p[1]}
	}
	b.setRanges(rs, 0)
	b.syncWant()
}

// HasMarks reports whether any mark is still registered. Undo and redo drop
// them, because they replace the text the marks pointed into.
func (b *Buffer) HasMarks() bool { return len(b.marks) > 0 }
