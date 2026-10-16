package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

// tab is one open file in the editor area.
type tab struct {
	buf *buffer.Buffer
	hl  *highlighter
}

func newTab(b *buffer.Buffer) *tab {
	return &tab{buf: b, hl: newHighlighter(b.Path)}
}

type cellStyle struct {
	fg, bg string
	cursor bool
}

var cellStyles = map[cellStyle]lipgloss.Style{}

func styleFor(k cellStyle) lipgloss.Style {
	if s, ok := cellStyles[k]; ok {
		return s
	}
	s := bgfg(k.bg, k.fg)
	if k.cursor {
		s = bgfg("#aeafad", "#1e1e1e")
	}
	cellStyles[k] = s
	return s
}

func gutterWidth(b *buffer.Buffer) int {
	return len(fmt.Sprint(len(b.Lines))) + 3
}

// ensureCursorVisible adjusts scrolling so the cursor is on screen.
func ensureCursorVisible(b *buffer.Buffer, w, h int) {
	if h <= 0 {
		return
	}
	textW := max(1, w-gutterWidth(b))
	if b.Cur.Row < b.ScrollRow {
		b.ScrollRow = b.Cur.Row
	}
	if b.Cur.Row >= b.ScrollRow+h {
		b.ScrollRow = b.Cur.Row - h + 1
	}
	vis := buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)
	if vis < b.ScrollCol {
		b.ScrollCol = max(0, vis-textW/4)
	}
	if vis >= b.ScrollCol+textW {
		b.ScrollCol = vis - textW + 1 + textW/4
	}
}

// renderEditor draws the visible part of a buffer as exactly h lines of
// width w.
func renderEditor(t *tab, w, h int, focused bool, query string) []string {
	b := t.buf
	colors := t.hl.Colors(b)
	gw := gutterWidth(b)
	textW := max(0, w-gw)
	selS, selE, hasSel := b.Selection()
	q := []rune(strings.ToLower(query))

	out := make([]string, 0, h)
	for i := 0; i < h; i++ {
		row := b.ScrollRow + i
		if row >= len(b.Lines) {
			out = append(out, stEmpty.Render(strings.Repeat(" ", w)))
			continue
		}
		isCur := row == b.Cur.Row
		lineBg := cEditorBg
		numSt := stLineNo
		if isCur && !hasSel {
			lineBg = cCurLine
			numSt = stLineNoCur
		}
		num := fmt.Sprintf("%*d  ", gw-2, row+1)
		var sb strings.Builder
		sb.WriteString(numSt.Render(num))

		line := b.Lines[row]
		var lc []string
		if colors != nil && row < len(colors) {
			lc = colors[row]
		}
		var matchCols []int
		if len(q) > 0 {
			matchCols = buffer.MatchCols(line, q)
		}
		inMatch := func(col int) bool {
			for _, m := range matchCols {
				if col >= m && col < m+len(q) {
					return true
				}
			}
			return false
		}
		inSel := func(col int) bool {
			if !hasSel {
				return false
			}
			p := buffer.Pos{Row: row, Col: col}
			return !p.Less(selS) && p.Less(selE)
		}

		var seg strings.Builder
		var segKey cellStyle
		segOpen := false
		flush := func() {
			if segOpen {
				sb.WriteString(styleFor(segKey).Render(seg.String()))
				seg.Reset()
				segOpen = false
			}
		}
		put := func(k cellStyle, s string) {
			if segOpen && k != segKey {
				flush()
			}
			segKey = k
			segOpen = true
			seg.WriteString(s)
		}

		vis := 0
		drawn := 0
		cellBg := func(col int) string {
			switch {
			case inSel(col):
				return cSelection
			case inMatch(col):
				return cMatch
			}
			return lineBg
		}
		for col := 0; col <= len(line) && drawn < textW; col++ {
			cursorHere := focused && isCur && col == b.Cur.Col
			var r rune = ' '
			rw := 1
			if col < len(line) {
				r = line[col]
				rw = buffer.RuneWidth(r, vis)
			} else if !cursorHere {
				break
			}
			fg := cEditorFg
			if lc != nil && col < len(lc) && lc[col] != "" {
				fg = lc[col]
			}
			k := cellStyle{fg: fg, bg: cellBg(col), cursor: cursorHere}
			if r == '\t' || col == len(line) {
				for c := 0; c < rw; c++ {
					if vis+c >= b.ScrollCol && drawn < textW {
						kk := k
						kk.cursor = cursorHere && c == 0
						put(kk, " ")
						drawn++
					}
				}
			} else if vis >= b.ScrollCol {
				if drawn+rw > textW {
					break
				}
				put(k, string(r))
				drawn += rw
			} else if vis+rw > b.ScrollCol {
				// wide rune partially scrolled off the left edge
				put(k, strings.Repeat(" ", vis+rw-b.ScrollCol))
				drawn += vis + rw - b.ScrollCol
			}
			vis += rw
		}
		flush()
		if drawn < textW {
			sb.WriteString(styleFor(cellStyle{fg: cEditorFg, bg: lineBg}).Render(strings.Repeat(" ", textW-drawn)))
		}
		out = append(out, sb.String())
	}
	return out
}

func renderWelcome(w, h int) []string {
	msg := []struct {
		text string
		head bool
	}{
		{"gh-editor", true},
		{"", false},
		{"Ctrl+P        ファイルを開く", false},
		{"F1 / Ctrl+K   コマンドパレット", false},
		{"Ctrl+B        サイドバーの表示切替", false},
		{"Ctrl+E        エクスプローラーへ移動", false},
		{"Ctrl+J        ターミナルの表示切替", false},
		{"Ctrl+N        新しいファイル", false},
		{"Ctrl+Q        終了", false},
	}
	out := make([]string, h)
	top := max(0, (h-len(msg))/2)
	for i := range out {
		out[i] = stEmpty.Render(strings.Repeat(" ", w))
	}
	for i, m := range msg {
		if top+i >= h {
			break
		}
		width := lipgloss.Width(m.text)
		pad := max(0, (w-30)/2)
		if m.head {
			pad = max(0, (w-width)/2)
		}
		st := stWelcome
		if m.head {
			st = stWelcomeH
		}
		out[top+i] = st.Render(fit(strings.Repeat(" ", pad)+m.text, w))
	}
	return out
}
