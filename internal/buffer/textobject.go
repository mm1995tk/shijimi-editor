package buffer

import (
	"strings"
	"unicode"
)

// SelectTextObject selects the Helix text object named by kind around the
// cursor: "w" word, "W" WORD, "p" paragraph, a bracket (either side of the
// pair) or a quote. With around it includes the surrounding whitespace,
// blank lines or delimiters (Helix "ma"); otherwise only the inside ("mi").
// It reports false, leaving the selection unchanged, when there is no such
// object at the cursor. The selection replaces the current one.
func (b *Buffer) SelectTextObject(kind rune, around bool) bool {
	var s, e Pos
	var ok bool
	switch kind {
	case 'w', 'W':
		s, e, ok = b.wordObject(kind == 'W', around)
	case 'p':
		s, e, ok = b.paragraphObject(around)
	case '\'', '"', '`':
		s, e, ok = b.quoteObject(kind, around)
	default:
		s, e, ok = b.bracketObject(kind, around)
	}
	if !ok {
		return false
	}
	b.lineSel = nil
	b.Anchor = &Pos{s.Row, s.Col}
	b.Cur = e
	b.lastEdit = editOther
	b.syncWant()
	return true
}

// objectClass groups characters the way Helix's word objects do: 0 is
// whitespace, and for a short word letters/digits/_ and punctuation are
// different classes. For a long WORD everything but whitespace is one class.
func objectClass(r rune, long bool) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case long || isWord(r):
		return 1
	}
	return 2
}

func (b *Buffer) wordObject(long, around bool) (Pos, Pos, bool) {
	l := b.Lines[b.Cur.Row]
	c := b.Cur.Col
	if c >= len(l) {
		return Pos{}, Pos{}, false
	}
	cls := objectClass(l[c], long)
	run := func(from int, class int) (int, int) {
		s, e := from, from
		for s > 0 && objectClass(l[s-1], long) == class {
			s--
		}
		for e < len(l) && objectClass(l[e], long) == class {
			e++
		}
		return s, e
	}
	s, e := run(c, cls)
	if around {
		if cls == 0 {
			// whitespace plus the word that follows it
			if e < len(l) {
				_, e = run(e, objectClass(l[e], long))
			}
		} else if e < len(l) && objectClass(l[e], long) == 0 {
			_, e = run(e, 0)
		} else if s > 0 {
			s, _ = run(s-1, 0) // no trailing space: take the leading space
		}
	}
	return Pos{b.Cur.Row, s}, Pos{b.Cur.Row, e}, true
}

func (b *Buffer) blankRow(row int) bool {
	return strings.TrimSpace(string(b.Lines[row])) == ""
}

func (b *Buffer) paragraphObject(around bool) (Pos, Pos, bool) {
	row := b.Cur.Row
	blank := b.blankRow(row)
	s, e := row, row
	for s > 0 && b.blankRow(s-1) == blank {
		s--
	}
	for e+1 < len(b.Lines) && b.blankRow(e+1) == blank {
		e++
	}
	if around {
		if blank {
			// blank lines plus the paragraph that follows them
			if e+1 < len(b.Lines) {
				e++
				for e+1 < len(b.Lines) && !b.blankRow(e+1) {
					e++
				}
			}
		} else if e+1 < len(b.Lines) {
			e++ // the next row is blank
			for e+1 < len(b.Lines) && b.blankRow(e+1) {
				e++
			}
		} else {
			for s > 0 && b.blankRow(s-1) {
				s--
			}
		}
	}
	return Pos{s, 0}, b.lineEndPos(e), true
}

func (b *Buffer) quoteObject(q rune, around bool) (Pos, Pos, bool) {
	l := b.Lines[b.Cur.Row]
	open := -1
	for i := 0; i < len(l); i++ {
		switch {
		case l[i] == '\\':
			i++
		case l[i] == q && open < 0:
			open = i
		case l[i] == q:
			if b.Cur.Col >= open && b.Cur.Col <= i {
				s, e := open+1, i
				if around {
					s, e = open, i+1
				}
				return Pos{b.Cur.Row, s}, Pos{b.Cur.Row, e}, true
			}
			open = -1
		}
	}
	return Pos{}, Pos{}, false
}

func (b *Buffer) bracketObject(k rune, around bool) (Pos, Pos, bool) {
	var open, close rune
	switch k {
	case '(', ')':
		open, close = '(', ')'
	case '[', ']':
		open, close = '[', ']'
	case '{', '}':
		open, close = '{', '}'
	case '<', '>':
		open, close = '<', '>'
	default:
		return Pos{}, Pos{}, false
	}
	at := func(p Pos) rune { return b.Lines[p.Row][p.Col] }
	prev := func(p Pos) (Pos, bool) {
		p.Col--
		for p.Col < 0 {
			p.Row--
			if p.Row < 0 {
				return p, false
			}
			p.Col = len(b.Lines[p.Row]) - 1
		}
		return p, true
	}
	next := func(p Pos) (Pos, bool) {
		p.Col++
		for p.Col >= len(b.Lines[p.Row]) {
			p.Row++
			p.Col = 0
			if p.Row >= len(b.Lines) {
				return p, false
			}
		}
		return p, true
	}
	// find the opening bracket: under the cursor, or the nearest unmatched
	// one before it
	p := b.Cur
	var os Pos
	found := false
	if p.Col < len(b.Lines[p.Row]) && at(p) == open {
		os, found = p, true
	} else {
		depth := 0
		for q, ok := prev(p); ok; q, ok = prev(q) {
			switch at(q) {
			case close:
				depth++
			case open:
				if depth == 0 {
					os, found = q, true
				} else {
					depth--
				}
			}
			if found {
				break
			}
		}
	}
	if !found {
		return Pos{}, Pos{}, false
	}
	depth := 0
	for q, ok := next(os); ok; q, ok = next(q) {
		switch at(q) {
		case open:
			depth++
		case close:
			if depth > 0 {
				depth--
				continue
			}
			// the pair must contain the cursor (it may end at the cursor)
			if q.Less(b.Cur) {
				return Pos{}, Pos{}, false
			}
			if around {
				return os, Pos{q.Row, q.Col + 1}, true
			}
			in := Pos{os.Row, os.Col + 1}
			if in == q {
				return Pos{}, Pos{}, false
			}
			return in, q, true
		}
	}
	return Pos{}, Pos{}, false
}
