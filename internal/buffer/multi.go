package buffer

import (
	"errors"
	"regexp"
	"sort"
	"unicode/utf8"
)

// Range is one cursor with its optional selection anchor.
type Range struct {
	Anchor    Pos
	HasAnchor bool
	Cur       Pos
	want      int
}

// Start and End return the ordered bounds of the range.
func (r Range) Start() Pos {
	if r.HasAnchor && r.Cur.Less(r.Anchor) {
		return r.Cur
	}
	if r.HasAnchor {
		return r.Anchor
	}
	return r.Cur
}

func (r Range) End() Pos {
	if r.HasAnchor && r.Anchor.Less(r.Cur) {
		return r.Cur
	}
	return r.Cur
}

// multi holds the secondary cursors. The primary cursor stays in Buffer.Cur
// and Buffer.Anchor, so single-cursor code does not need to know about them.
type multi struct {
	extra []Range

	// state while ForEachCursor runs: every cursor is in all, and the one
	// being edited lives in Cur/Anchor.
	looping    bool
	batchBegun bool
	all        []Range
	active     int
}

// HasExtra reports whether there is more than one cursor.
func (b *Buffer) HasExtra() bool { return len(b.extra) > 0 }

// Extra returns the secondary cursors.
func (b *Buffer) Extra() []Range { return append([]Range(nil), b.extra...) }

// ClearExtra drops every cursor except the primary one.
func (b *Buffer) ClearExtra() { b.extra = nil }

func (b *Buffer) primaryRange() Range {
	r := Range{Cur: b.Cur, want: b.wantVis}
	if b.Anchor != nil {
		r.Anchor, r.HasAnchor = *b.Anchor, true
	}
	return r
}

func (b *Buffer) load(r Range) {
	b.Cur = r.Cur
	b.Anchor = nil
	if r.HasAnchor {
		a := r.Anchor
		b.Anchor = &a
	}
	b.wantVis = r.want
}

// ForEachCursor runs f once per cursor with that cursor loaded as the
// primary one. Edits made by f move the other cursors, and the whole run is
// a single undo step. The primary cursor is visited last.
func (b *Buffer) ForEachCursor(f func()) {
	if len(b.extra) == 0 || b.looping {
		f()
		return
	}
	b.looping, b.batchBegun = true, false
	b.all = append(append([]Range(nil), b.extra...), b.primaryRange())
	prim := len(b.all) - 1
	for i := range b.all {
		b.active = i
		b.load(b.all[i])
		f()
		b.all[i] = b.primaryRange()
	}
	all := b.all
	b.looping, b.all = false, nil
	b.setRanges(all, prim)
}

// shiftOthers moves the cursors that are not being edited.
func (b *Buffer) shiftOthers(f func(Pos) Pos) {
	if !b.looping {
		return
	}
	for i := range b.all {
		if i == b.active {
			continue
		}
		b.all[i].Cur = f(b.all[i].Cur)
		b.all[i].Anchor = f(b.all[i].Anchor)
	}
}

func shiftInsert(p, start, end Pos) Pos {
	if p.Less(start) {
		return p
	}
	if p.Row == start.Row {
		return Pos{end.Row, end.Col + p.Col - start.Col}
	}
	return Pos{p.Row + end.Row - start.Row, p.Col}
}

func shiftDelete(p, s, e Pos) Pos {
	if !s.Less(p) {
		return p
	}
	if p.Less(e) {
		return s
	}
	if p.Row == e.Row {
		return Pos{s.Row, s.Col + p.Col - e.Col}
	}
	return Pos{p.Row - (e.Row - s.Row), p.Col}
}

// setRanges makes ranges the cursor set: it sorts them, merges overlapping
// ones and loads the one that contains ranges[prim] as the primary cursor.
func (b *Buffer) setRanges(ranges []Range, prim int) {
	type item struct {
		Range
		prim bool
	}
	items := make([]item, len(ranges))
	for i, r := range ranges {
		r.Cur, r.Anchor = b.ClampPos(r.Cur), b.ClampPos(r.Anchor)
		items[i] = item{r, i == prim}
	}
	sort.SliceStable(items, func(i, j int) bool {
		si, sj := items[i].Start(), items[j].Start()
		if si != sj {
			return si.Less(sj)
		}
		return items[i].End().Less(items[j].End())
	})
	var out []item
	for _, it := range items {
		if n := len(out); n > 0 {
			last := &out[n-1]
			if it.Start().Less(last.End()) || (it.Start() == last.Start() && it.End() == last.End()) {
				if it.End().Less(last.End()) || it.End() == last.End() {
					// contained: keep the earlier one
				} else {
					last.Anchor, last.HasAnchor, last.Cur = last.Start(), true, it.End()
				}
				last.prim = last.prim || it.prim
				continue
			}
		}
		out = append(out, it)
	}
	b.extra = nil
	for _, it := range out {
		if it.prim {
			b.load(it.Range)
		} else {
			b.extra = append(b.extra, it.Range)
		}
	}
}

// CopySelection copies every cursor to the line below (dir = 1) or above
// (dir = -1) and makes the copy of the primary cursor the new primary one.
func (b *Buffer) CopySelection(dir int) {
	all := append(append([]Range(nil), b.extra...), b.primaryRange())
	prim := len(all) - 1
	out := append([]Range(nil), all...)
	primOut := prim
	for i, r := range all {
		nr, ok := b.shiftRows(r, dir)
		if !ok {
			continue
		}
		out = append(out, nr)
		if i == prim {
			primOut = len(out) - 1
		}
	}
	b.setRanges(out, primOut)
}

func (b *Buffer) shiftRows(r Range, dir int) (Range, bool) {
	move := func(p Pos) (Pos, bool) {
		row := p.Row + dir
		if row < 0 || row >= len(b.Lines) {
			return p, false
		}
		vis := VisualCol(b.Lines[p.Row], p.Col)
		return Pos{row, ColFromVisual(b.Lines[row], vis)}, true
	}
	cur, ok := move(r.Cur)
	if !ok {
		return r, false
	}
	nr := Range{Cur: cur, want: r.want}
	if r.HasAnchor {
		a, ok := move(r.Anchor)
		if !ok {
			return r, false
		}
		nr.Anchor, nr.HasAnchor = a, true
	}
	return nr, true
}

var errNoMatch = errors.New("一致する箇所がありません")

// SelectMatches replaces every selection (or the character under a cursor
// that has none) with a cursor for each non-empty match of re inside it.
// Nothing changes when there is no match.
func (b *Buffer) SelectMatches(re *regexp.Regexp) error {
	all := append(append([]Range(nil), b.extra...), b.primaryRange())
	prim := len(all) - 1
	var out []Range
	primOut := -1
	for i, r := range all {
		s, e := r.Start(), r.End()
		if !s.Less(e) {
			s = r.Cur
			e = s
			if e.Col < len(b.Lines[e.Row]) {
				e.Col++
			} else if e.Row+1 < len(b.Lines) {
				e = Pos{e.Row + 1, 0}
			}
		}
		text := b.textRange(s, e)
		for _, m := range re.FindAllStringIndex(text, -1) {
			if m[0] == m[1] {
				continue
			}
			from := s.advance(text, utf8.RuneCountInString(text[:m[0]]))
			to := s.advance(text, utf8.RuneCountInString(text[:m[1]]))
			if i == prim && primOut < 0 {
				primOut = len(out)
			}
			out = append(out, Range{Anchor: from, HasAnchor: true, Cur: to})
		}
	}
	if len(out) == 0 {
		return errNoMatch
	}
	if primOut < 0 {
		primOut = 0
	}
	b.setRanges(out, primOut)
	b.syncWant()
	return nil
}

// advance returns the position n runes into text, which starts at p.
func (p Pos) advance(text string, n int) Pos {
	for _, r := range text {
		if n == 0 {
			break
		}
		n--
		if r == '\n' {
			p = Pos{p.Row + 1, 0}
		} else {
			p.Col++
		}
	}
	return p
}
