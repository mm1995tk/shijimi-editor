package ui

import (
	"sort"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// snipStop is one tab stop of a parsed LSP snippet. start and end are rune
// offsets into the expanded text.
type snipStop struct{ n, start, end int }

type snipParser struct {
	in    []rune
	i     int
	out   []rune
	stops []snipStop
}

// parseSnippet expands an LSP snippet ($1, ${1:foo}, ${1|a,b|}, $0) into
// plain text and the tab stops inside it. Variables expand to their default
// text, or to nothing.
func parseSnippet(s string) (string, []snipStop) {
	p := &snipParser{in: []rune(s)}
	p.seq(false)
	return string(p.out), p.stops
}

func (p *snipParser) seq(nested bool) {
	for p.i < len(p.in) {
		c := p.in[p.i]
		switch {
		case c == '\\' && p.i+1 < len(p.in):
			p.out = append(p.out, p.in[p.i+1])
			p.i += 2
		case c == '}' && nested:
			return
		case c == '$':
			p.dollar()
		default:
			p.out = append(p.out, c)
			p.i++
		}
	}
}

func (p *snipParser) digits() (int, bool) {
	n, ok := 0, false
	for p.i < len(p.in) && p.in[p.i] >= '0' && p.in[p.i] <= '9' {
		n, ok = n*10+int(p.in[p.i]-'0'), true
		p.i++
	}
	return n, ok
}

func (p *snipParser) ident() {
	for p.i < len(p.in) && (p.in[p.i] == '_' || unicode.IsLetter(p.in[p.i]) || unicode.IsDigit(p.in[p.i])) {
		p.i++
	}
}

func (p *snipParser) closeBrace() {
	if p.i < len(p.in) && p.in[p.i] == '}' {
		p.i++
	}
}

// dollar parses the construct that starts at the '$' at p.i.
func (p *snipParser) dollar() {
	p.i++
	if p.i >= len(p.in) {
		p.out = append(p.out, '$')
		return
	}
	pos := len(p.out)
	if n, ok := p.digits(); ok {
		p.stops = append(p.stops, snipStop{n, pos, pos})
		return
	}
	if p.in[p.i] != '{' {
		if c := p.in[p.i]; c == '_' || unicode.IsLetter(c) {
			p.ident() // $name: a variable, expands to nothing
			return
		}
		p.out = append(p.out, '$')
		return
	}
	p.i++
	n, num := p.digits()
	if !num {
		p.ident()
		if p.i < len(p.in) && p.in[p.i] == ':' {
			p.i++
			p.seq(true)
		}
		p.closeBrace()
		return
	}
	switch {
	case p.i < len(p.in) && p.in[p.i] == ':':
		p.i++
		p.seq(true)
		p.closeBrace()
	case p.i < len(p.in) && p.in[p.i] == '|':
		p.i++
		done := false
		for p.i < len(p.in) && !(p.in[p.i] == '|' && p.i+1 < len(p.in) && p.in[p.i+1] == '}') {
			c := p.in[p.i]
			switch {
			case c == '\\' && p.i+1 < len(p.in):
				if !done {
					p.out = append(p.out, p.in[p.i+1])
				}
				p.i += 2
				continue
			case c == ',':
				done = true
			case !done:
				p.out = append(p.out, c)
			}
			p.i++
		}
		if p.i < len(p.in) {
			p.i += 2
		}
	default:
		p.closeBrace()
	}
	p.stops = append(p.stops, snipStop{n, pos, len(p.out)})
}

// snippetSession is a snippet being filled in: Tab and Shift+Tab move
// between its tab stops. A stop's start and end marks follow the edits made
// while it is selected.
type snippetSession struct {
	tab    *tab
	groups [][]snipSpan // stops that share a number, in visiting order
	cur    int
}

type snipSpan struct{ start, end *buffer.Mark }

func (s *snippetSession) span0() snipSpan { return s.groups[s.cur][0] }

// startSnippet replaces the text from start to the cursor with the snippet
// and begins a session when it has tab stops to visit.
func (a *App) startSnippet(t *tab, start buffer.Pos, text string) {
	a.endSnippet()
	b := t.buf
	body, stops := parseSnippet(text)
	b.SelectRange(start, b.Cur)
	b.InsertBlock(body)
	runes := []rune(body)
	at := func(off int) buffer.Pos {
		p := start
		for _, r := range runes[:off] {
			if r == '\n' {
				p = buffer.Pos{Row: p.Row + 1}
			} else {
				p.Col++
			}
		}
		return p
	}
	byN := map[int][]snipStop{}
	var nums []int
	for _, st := range stops {
		if _, ok := byN[st.n]; !ok && st.n > 0 {
			nums = append(nums, st.n)
		}
		byN[st.n] = append(byN[st.n], st)
	}
	sort.Ints(nums)
	end := []snipStop{{0, len(runes), len(runes)}}
	if z, ok := byN[0]; ok {
		end = z
	}
	if len(nums) == 0 {
		b.SelectRanges([][2]buffer.Pos{{at(end[0].start), at(end[0].end)}})
		return
	}
	byN[0] = end
	s := &snippetSession{tab: t}
	for _, n := range append(nums, 0) {
		var spans []snipSpan
		for _, st := range byN[n] {
			sp := snipSpan{&buffer.Mark{Pos: at(st.start)}, &buffer.Mark{Pos: at(st.end), Right: true}}
			b.AddMark(sp.start)
			b.AddMark(sp.end)
			spans = append(spans, sp)
		}
		s.groups = append(s.groups, spans)
	}
	a.snip = s
	a.selectStop(0)
}

// selectStop selects every placeholder of stop i; the last stop ends the
// session.
func (a *App) selectStop(i int) {
	s := a.snip
	s.cur = i
	// Text typed at a boundary belongs to the selected stop. Which side a
	// mark of another stop falls on is decided by where it sits in the
	// document, not by the order the stops are visited in.
	cur := s.groups[i]
	for g, spans := range s.groups {
		for _, sp := range spans {
			if g == i {
				sp.start.Right, sp.end.Right = false, true
				continue
			}
			baseStart, baseEnd := nearest(cur, sp.start.Pos), nearest(cur, sp.end.Pos)
			if sp.start.Pos == baseStart && sp.end.Pos == baseStart {
				// an empty stop at the insertion point: keep the visiting order
				sp.start.Right, sp.end.Right = g > i, g > i
				continue
			}
			sp.start.Right = !sp.start.Pos.Less(baseStart)
			sp.end.Right = baseEnd.Less(sp.end.Pos)
		}
	}
	var pairs [][2]buffer.Pos
	for _, sp := range s.groups[i] {
		pairs = append(pairs, [2]buffer.Pos{sp.start.Pos, sp.end.Pos})
	}
	s.tab.buf.SelectRanges(pairs)
	a.follow = true
	if i == len(s.groups)-1 {
		a.endSnippet()
	}
}

func (a *App) endSnippet() {
	if a.snip != nil {
		a.snip.tab.buf.ClearMarks()
		a.snip = nil
	}
}

// snippetActive reports whether the session still owns the cursor; it ends
// the session when the cursor left the selected stop.
func (a *App) snippetActive() bool {
	s := a.snip
	if s == nil {
		return false
	}
	t := a.curText()
	if t != s.tab || !t.buf.HasMarks() || a.focus != focusEditor || a.helix && a.mode != modeInsert {
		a.endSnippet()
		return false
	}
	sp := s.span0()
	if c := t.buf.Cur; c.Less(sp.start.Pos) || sp.end.Pos.Less(c) {
		a.endSnippet()
		return false
	}
	return true
}

// handleSnippetKey moves between tab stops. handled is false for every key
// the session does not take.
func (a *App) handleSnippetKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if !a.snippetActive() {
		return nil, false
	}
	switch msg.String() {
	case "tab":
		a.selectStop(a.snip.cur + 1)
		return nil, true
	case "shift+tab":
		a.selectStop(max(0, a.snip.cur-1))
		return nil, true
	case "esc":
		a.endSnippet()
	}
	return nil, false
}

// nearest returns the start of the span in spans that is closest in front of
// p (or at it), or the first span's start when none is.
func nearest(spans []snipSpan, p buffer.Pos) buffer.Pos {
	base := spans[0].start.Pos
	for _, c := range spans {
		if !p.Less(c.start.Pos) {
			base = c.start.Pos
		}
	}
	return base
}
