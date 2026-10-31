package ui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
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
	plain  string      // its text, padded to the panel width
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
	rows      []logRow
	y0, start int
	n, w      int
}

// agentPos is a cell of the log: row indexes the whole log, so it stays on
// the same text while the view scrolls.
type agentPos struct{ row, col int }

func (p agentPos) less(q agentPos) bool {
	return p.row < q.row || p.row == q.row && p.col < q.col
}

// agentSel is the picked text. a is where the drag started and b where it is
// now; both cells are included.
type agentSel struct {
	on       bool // something is selected and drawn
	dragging bool
	a, b     agentPos
}

func (s agentSel) ordered() (agentPos, agentPos) {
	if s.b.less(s.a) {
		return s.b, s.a
	}
	return s.a, s.b
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
func (s agentSel) rowCols(i int, plain string, w int) (int, int) {
	if !s.on {
		return 0, 0
	}
	a, b := s.ordered()
	if i < a.row || i > b.row {
		return 0, 0
	}
	c0, c1 := 0, w
	if i == a.row {
		c0 = a.col
	}
	if i == b.row {
		c1 = b.col + 1
	}
	return snapCols(plain, c0, c1)
}

// markRow draws the columns [s0, s1) of the row reversed, keeping its colors.
func markRow(row string, w, s0, s1 int) string {
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
func cutCols(plain string, c0, c1 int) string {
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
func (s agentSel) text(rows []logRow, w int) string {
	if !s.on {
		return ""
	}
	a, b := s.ordered()
	var sb strings.Builder
	for i := a.row; i <= b.row && i < len(rows); i++ {
		c0, c1 := 0, w
		if i == a.row {
			c0 = a.col
		}
		if i == b.row {
			c1 = b.col + 1
		}
		seg := strings.TrimRight(cutCols(rows[i].plain, c0, c1), " ")
		if i > a.row {
			switch {
			case !rows[i].cont:
				sb.WriteByte('\n')
			case rows[i-1].space:
				sb.WriteByte(' ')
			}
		}
		switch {
		case rows[i].to > 0:
			seg = cutCols(rows[i].plain, max(c0, rows[i].from), min(c1, rows[i].to))
		case rows[i].cont:
			seg = strings.Trim(cutCols(rows[i].plain, max(c0, rows[i].indent), c1), " ")
		case c0 == 0:
			seg = strings.TrimPrefix(seg, " ") // the margin of the panel
		}
		sb.WriteString(seg)
	}
	return sb.String()
}

// logPos is the cell of the log under screen cell (x, y), clamped to the rows
// drawn. above and below say the pointer is off them.
func (a *App) logPos(l layout, msg Mouse) (p agentPos, above, below bool) {
	v := a.ag.view
	y := msg.Row - v.y0
	p.col = clamp(msg.Col, 0, max(0, v.w-1))
	switch {
	case y < 0:
		above = true
		p.row = v.start
	case y >= v.n:
		below = true
		p.row = v.start + v.n - 1
	default:
		p.row = v.start + y
	}
	p.row = clamp(p.row, 0, max(0, len(v.rows)-1))
	return
}

// inLog reports whether screen cell (x, y) is on a drawn log row.
func (a *App) inLog(l layout, msg Mouse) bool {
	v := a.ag.view
	y := msg.Row - v.y0
	return v.n > 0 && y >= 0 && y < v.n && msg.Col >= 0
}

// agentDrag follows a drag that began on the log, wherever the pointer goes.
// ok is false when the event is not part of it.
func (a *App) agentDrag(l layout, msg Mouse) (tea.Cmd, bool) {
	s := &a.ag.sel
	switch {
	case msg.Action == MousePress && (msg.Button == MouseWheelUp || msg.Button == MouseWheelDown):
		return nil, false // scrolling keeps the selection
	case msg.Action == MousePress:
		s.dragging = false // the release was lost
		return nil, false
	}
	p, above, below := a.logPos(l, msg)
	switch {
	case above:
		a.ag.scroll++
		p.row = clamp(a.ag.view.start-1, 0, max(0, len(a.ag.view.rows)-1))
	case below:
		a.ag.scroll = max(0, a.ag.scroll-1)
		p.row = clamp(a.ag.view.start+a.ag.view.n, 0, max(0, len(a.ag.view.rows)-1))
	}
	s.b = p
	s.on = s.a != s.b
	if msg.Action == MouseRelease {
		s.dragging = false
		if txt := s.text(a.ag.view.rows, a.ag.view.w); txt != "" {
			a.copyText(txt)
			a.info(i18n.T("agentsel.copied"), utf8.RuneCountInString(txt))
		}
	}
	return nil, true
}
