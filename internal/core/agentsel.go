package core

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// Text of the agent panel's log picked with the mouse.

// logRow is one row of the log. cont marks a row that continues the one
// above it, cut by the panel width; space says the cut fell on a space, which
// copying puts back.
type logRow struct {
	line   screen.Line // the row as spans
	Plain  string      // its text, padded to the panel width
	cont   bool
	space  bool
	indent int // columns of decoration that copying leaves out of a cont row
	// from, to, when to > 0, are the columns copied as they are, spaces
	// included: a row of a code line cut by the width
	from, to int
}

// agentView is where the log was drawn last: rows[start:start+n] sit at row
// y0 of the panel.
type agentView struct {
	Rows      []logRow
	Y0, Start int
	n, W      int
}

// agentPos is a cell of the log: row indexes the whole log, so it stays on
// the same text while the view scrolls.
type AgentPos struct{ Row, Col int }

func (p AgentPos) less(q AgentPos) bool {
	return p.Row < q.Row || p.Row == q.Row && p.Col < q.Col
}

// agentSel is the picked text. a is where the drag started and b where it is
// now; both cells are included.
type AgentSel struct {
	On       bool // something is selected and drawn
	Dragging bool
	A, B     AgentPos
}

func (s AgentSel) ordered() (AgentPos, AgentPos) {
	if s.B.less(s.A) {
		return s.B, s.A
	}
	return s.A, s.B
}

// snapCols widens the columns [c0, c1) to whole characters of plain.
func snapCols(plain string, c0, c1 int) (int, int) {
	x := 0
	s0, s1 := c0, c1
	for g := uniseg.NewGraphemes(plain); g.Next(); { // by grapheme cluster, as the row is drawn
		w := ansi.StringWidth(g.Str())
		if x < c0 && x+w > c0 {
			s0 = x
		}
		if x < c1 && x+w > c1 {
			s1 = x + w
		}
		x += w
	}
	return s0, s1
}

// rowCols returns the columns [s0, s1) of log row i that are selected;
// s0 == s1 when none are.
func (s AgentSel) rowCols(i int, plain string, w int) (int, int) {
	if !s.On {
		return 0, 0
	}
	a, b := s.ordered()
	if i < a.Row || i > b.Row {
		return 0, 0
	}
	c0, c1 := 0, w
	if i == a.Row {
		c0 = a.Col
	}
	if i == b.Row {
		c1 = b.Col + 1
	}
	return snapCols(plain, c0, c1)
}

// markRow draws the columns [s0, s1) of the row reversed, keeping its colors.
func MarkRow(row string, w, s0, s1 int) string {
	if s0 >= s1 {
		return row
	}
	mid := ansi.Cut(row, s0, s1)
	for _, reset := range []string{"\x1b[m", "\x1b[0m"} {
		mid = strings.ReplaceAll(mid, reset, reset+"\x1b[7m")
	}
	return ansi.Cut(row, 0, s0) + "\x1b[7m" + mid + "\x1b[27m" + ansi.Cut(row, s1, w)
}

// cutCols returns the characters of plain that cover columns [c0, c1).
func CutCols(plain string, c0, c1 int) string {
	var sb strings.Builder
	x := 0
	for g := uniseg.NewGraphemes(plain); g.Next(); { // by grapheme cluster, as the row is drawn
		w := ansi.StringWidth(g.Str())
		if x < c1 && x+w > c0 {
			sb.WriteString(g.Str())
		}
		x += w
	}
	return sb.String()
}

// text returns the selected text. Rows cut by the width join up again.
func (s AgentSel) Text(rows []logRow, w int) string {
	if !s.On {
		return ""
	}
	a, b := s.ordered()
	var sb strings.Builder
	for i := a.Row; i <= b.Row && i < len(rows); i++ {
		c0, c1 := 0, w
		if i == a.Row {
			c0 = a.Col
		}
		if i == b.Row {
			c1 = b.Col + 1
		}
		seg := strings.TrimRight(CutCols(rows[i].Plain, c0, c1), " ")
		if i > a.Row {
			switch {
			case !rows[i].cont:
				sb.WriteByte('\n')
			case rows[i-1].space:
				sb.WriteByte(' ')
			}
		}
		switch {
		case rows[i].to > 0:
			seg = CutCols(rows[i].Plain, max(c0, rows[i].from), min(c1, rows[i].to))
		case rows[i].cont:
			seg = strings.Trim(CutCols(rows[i].Plain, max(c0, rows[i].indent), c1), " ")
		case c0 == 0:
			seg = strings.TrimPrefix(seg, " ") // the margin of the panel
		}
		sb.WriteString(seg)
	}
	return sb.String()
}

// logPos is the cell of the log under screen cell (x, y), clamped to the rows
// drawn. above and below say the pointer is off them.
func (a *Core) logPos(msg Mouse) (p AgentPos, above, below bool) {
	v := a.Ag.View
	y := msg.Row - v.Y0
	p.Col = Clamp(msg.Col, 0, max(0, v.W-1))
	switch {
	case y < 0:
		above = true
		p.Row = v.Start
	case y >= v.n:
		below = true
		p.Row = v.Start + v.n - 1
	default:
		p.Row = v.Start + y
	}
	p.Row = Clamp(p.Row, 0, max(0, len(v.Rows)-1))
	return
}

// inLog reports whether screen cell (x, y) is on a drawn log row.
func (a *Core) inLog(msg Mouse) bool {
	v := a.Ag.View
	y := msg.Row - v.Y0
	return v.n > 0 && y >= 0 && y < v.n && msg.Col >= 0
}

// agentDrag follows a drag that began on the log, wherever the pointer goes.
// ok is false when the event is not part of it.
func (a *Core) AgentDrag(msg Mouse) ([]Effect, bool) {
	s := &a.Ag.Sel
	switch {
	case msg.Action == MousePress && (msg.Button == MouseWheelUp || msg.Button == MouseWheelDown):
		return nil, false // scrolling keeps the selection
	case msg.Action == MousePress:
		s.Dragging = false // the release was lost
		return nil, false
	}
	p, above, below := a.logPos(msg)
	switch {
	case above:
		a.Ag.Scroll++
		p.Row = Clamp(a.Ag.View.Start-1, 0, max(0, len(a.Ag.View.Rows)-1))
	case below:
		a.Ag.Scroll = max(0, a.Ag.Scroll-1)
		p.Row = Clamp(a.Ag.View.Start+a.Ag.View.n, 0, max(0, len(a.Ag.View.Rows)-1))
	}
	s.B = p
	s.On = s.A != s.B
	if msg.Action == MouseRelease {
		s.Dragging = false
		if txt := s.Text(a.Ag.View.Rows, a.Ag.View.W); txt != "" {
			a.CopyText(txt)
			a.Info(i18n.T("agentsel.copied"), utf8.RuneCountInString(txt))
		}
	}
	return nil, true
}
