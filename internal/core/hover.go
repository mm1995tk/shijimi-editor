package core

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// Hover popup colors; set by applyTheme.

// hoverLines converts hover markdown into wrapped lines at most w cells wide.
func hoverLines(text string, w int) []hoverLine { return wrapHover(parseHover(text), w) }

// wrapHover wraps each logical line to w cells. A rule is left as it is: it is
// drawn at the popup width.
func wrapHover(lines []hoverLine, w int) []hoverLine {
	w = max(w, 1)
	var out []hoverLine
	for _, l := range lines {
		if l.rule {
			out = append(out, l)
			continue
		}
		out = append(out, wrapCells([][]hoverCell{l.cells}, w, l.code)...)
	}
	return out
}

// wrapCells wraps each logical line to w cells, preferring to break at spaces
// in text lines.
func wrapCells(lines [][]hoverCell, w int, code bool) []hoverLine {
	var out []hoverLine
	for _, cs := range lines {
		for {
			width, cut, space := 0, len(cs), -1
			for i, c := range cs {
				cw := ansi.StringWidth(string(c.r))
				if width+cw > w {
					cut = i
					break
				}
				width += cw
				if c.r == ' ' {
					space = i
				}
			}
			if cut == len(cs) {
				out = append(out, hoverLine{cells: cs, code: code})
				break
			}
			next := cut
			if !code && space > 0 {
				cut, next = space, space+1
			}
			if cut == 0 { // first rune wider than w: still make progress
				cut, next = 1, 1
			}
			out = append(out, hoverLine{cells: cs[:cut], code: code})
			cs = cs[next:]
		}
	}
	if len(lines) == 0 {
		out = append(out, hoverLine{code: code})
	}
	return out
}

// hoverCopyLabel is the click target drawn on the top border's right end.
func hoverCopyLabel() string { return i18n.T("hover.copybtn") }

func hoverHint() string         { return i18n.T("hover.hint") }
func hoverHintCopyOnly() string { return i18n.T("hover.hintcopy") }

// copyHit reports whether the cell (col, row), relative to the box's top-left
// corner, is on the [Copy] button.
func (v hoverView) copyHit(col, row int) bool {
	iw := v.width + 2
	n := ansi.StringWidth(hoverCopyLabel())
	if row != 0 || n+2 > iw {
		return false
	}
	start := iw - n // box column of the label's first cell (column 0 is the corner)
	return col >= start && col < start+n
}

// hoverView is a laid-out hover popup.
type hoverView struct {
	lines  []hoverLine
	width  int // content width
	height int // visible rows
}

// maxScroll is the largest valid scroll offset.
func (v hoverView) maxScroll() int { return max(0, len(v.lines)-v.height) }

// newHoverView lays text out inside a box of at most boxW x boxH cells
// (borders and padding included).
func newHoverView(text string, boxW, boxH int) hoverView {
	cw := max(boxW-4, 1)
	lines := hoverLines(text, cw)
	w := 0
	for _, l := range lines {
		cells := 0
		for _, c := range l.cells {
			cells += ansi.StringWidth(string(c.r))
		}
		w = max(w, cells)
	}
	return hoverView{lines: lines, width: max(w, 1), height: max(boxH-2, 1)}
}

// overlay is the popup as a screen.Overlay: the rows in view when scrolled by
// scroll (the rest stays in core), with View telling where they are among all
// of them. The caller says which pane it hangs on and from where.
func (v hoverView) overlay(scroll int) screen.Overlay {
	scroll = clamp(scroll, 0, v.maxScroll())
	end := min(scroll+v.height, len(v.lines))
	ov := screen.Overlay{
		Kind:      screen.OverlayHover,
		View:      &screen.Viewport{Top: scroll, Rows: v.height, Cols: v.width, Total: len(v.lines)},
		Hint:      hoverHintCopyOnly(),
		CopyLabel: hoverCopyLabel(),
	}
	if v.maxScroll() > 0 {
		ov.Hint = hoverHint()
	}
	for i, l := range v.lines[scroll:end] {
		sl := screen.Line{No: scroll + i, Rule: l.rule}
		if l.code {
			sl.Band = screen.StyleHoverCodeBg
		}
		for j := 0; j < len(l.cells); {
			k := j
			for k < len(l.cells) && l.cells[k].st == l.cells[j].st {
				k++
			}
			runes := make([]rune, 0, k-j)
			for _, c := range l.cells[j:k] {
				runes = append(runes, c.r)
			}
			sl.Spans = append(sl.Spans, screen.Span{Text: string(runes), Style: l.cells[j].st})
			j = k
		}
		ov.Lines = append(ov.Lines, sl)
	}
	return ov
}

