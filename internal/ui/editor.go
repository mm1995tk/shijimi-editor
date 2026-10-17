package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/config"
	"github.com/mm1995tk/gh-editor/internal/lsp"
)

type tabKind int

const (
	tabText tabKind = iota
	tabImage
	tabReview
	tabBrowser
)

// tab is one item in the editor area: a text buffer, an image preview or a
// diff review.
type tab struct {
	kind tabKind

	buf        *buffer.Buffer
	hl         *highlighter
	lang       *config.Language
	lspVersion int
	autoVer    int  // buffer version an auto-save timer was last scheduled for
	conflict   bool // changed on disk while holding unsaved edits (already reported)

	img *imageView
	rev *review
	dv  *diffView // set while the tab is shown as a diff (git-diff mode)
	web *browserView

	// Markdown preview beside the buffer
	preview   previewMode
	prevCache []string
	prevW     int
	prevVer   int
	prevGen   int

	// mermaid fences found at scanVer (scanned reports whether that is valid)
	scanned    bool
	scanVer    int
	scanBlocks []string
}

func newTab(b *buffer.Buffer) *tab {
	return &tab{kind: tabText, buf: b, hl: newHighlighter(b.Path), lspVersion: -1}
}

func (t *tab) title() string {
	switch t.kind {
	case tabImage:
		return t.img.name()
	case tabReview:
		return t.rev.tabTitle()
	case tabBrowser:
		return t.web.name()
	}
	return t.buf.Name()
}

func (t *tab) dirty() bool { return t.kind == tabText && t.buf.Dirty }

func (t *tab) path() string {
	switch t.kind {
	case tabText:
		return t.buf.Path
	case tabImage:
		return t.img.path
	}
	return ""
}

type cellStyle struct {
	fg, bg string
	cursor string // cursor color, "" = none
	under  string // underline color
}

var cellStyles = map[cellStyle]lipgloss.Style{}

func styleFor(k cellStyle) lipgloss.Style {
	if s, ok := cellStyles[k]; ok {
		return s
	}
	s := bgfg(k.bg, k.fg)
	if k.cursor != "" {
		s = bgfg(k.cursor, curTheme.CursorFg)
	}
	if k.under != "" {
		s = s.Underline(true)
	}
	cellStyles[k] = s
	return s
}

func gutterWidth(b *buffer.Buffer) int {
	return len(fmt.Sprint(len(b.Lines))) + 4
}

// ensureCursorVisible adjusts scrolling so the cursor is on screen.
func ensureCursorVisible(b *buffer.Buffer, w, h int) {
	if h <= 0 {
		return
	}
	textW := max(1, w-gutterWidth(b))
	margin := min(3, (h-1)/2)
	if b.Cur.Row < b.ScrollRow+margin {
		b.ScrollRow = max(0, b.Cur.Row-margin)
	}
	if b.Cur.Row >= b.ScrollRow+h-margin {
		b.ScrollRow = b.Cur.Row - h + margin + 1
	}
	vis := buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)
	if vis < b.ScrollCol {
		b.ScrollCol = max(0, vis-textW/4)
	}
	if vis >= b.ScrollCol+textW {
		b.ScrollCol = vis - textW + 1 + textW/4
	}
}

type editorOpts struct {
	focused      bool
	query        string
	cursorColor  string
	cursorAt     buffer.Pos   // where to draw the cursor
	extraCursors []buffer.Pos // cursors of a multi-cursor edit
	extraSels    [][2]buffer.Pos
	relative     bool
	diags        []lsp.Diagnostic
	rowBg        func(row int) string // background override per buffer line (diff view)
}

var severityColor map[int]string

func diagColor(sev int) string {
	if c, ok := severityColor[sev]; ok {
		return c
	}
	return severityColor[1]
}

// renderEditor draws the visible part of a buffer as exactly h lines of
// width w.
func renderEditor(t *tab, w, h int, o editorOpts) []string {
	b := t.buf
	colors := t.hl.Colors(b)
	gw := gutterWidth(b)
	textW := max(0, w-gw)
	selS, selE, hasSel := b.Selection()
	q := []rune(strings.ToLower(o.query))

	// diagnostics by line
	lineDiag := map[int]lsp.Diagnostic{}
	for _, d := range o.diags {
		r := d.Range.Start.Line
		if old, ok := lineDiag[r]; !ok || sevRank(d.Severity) < sevRank(old.Severity) {
			lineDiag[r] = d
		}
	}

	out := make([]string, 0, h)
	for i := 0; i < h; i++ {
		row := b.ScrollRow + i
		if row >= len(b.Lines) {
			out = append(out, stEmpty.Render(strings.Repeat(" ", w)))
			continue
		}
		isCur := row == b.Cur.Row
		lineBg := cEditorBg
		if o.rowBg != nil {
			if c := o.rowBg(row); c != "" {
				lineBg = c
			}
		}
		numSt := stLineNo
		if isCur && !hasSel {
			lineBg = cCurLine
		}
		if isCur {
			numSt = bgfg(lineBg, cLineNoCur)
		}
		n := row + 1
		if o.relative && !isCur {
			n = row - b.Cur.Row
			if n < 0 {
				n = -n
			}
		}
		var sb strings.Builder
		d, hasDiag := lineDiag[row]
		if hasDiag {
			sb.WriteString(bgfg(lineBg, diagColor(d.Severity)).Render("●"))
		} else {
			sb.WriteString(bgfg(lineBg, cLineNo).Render(" "))
		}
		sb.WriteString(numSt.Render(fmt.Sprintf("%*d  ", gw-3, n)))

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
			p := buffer.Pos{Row: row, Col: col}
			if hasSel && !p.Less(selS) && p.Less(selE) {
				return true
			}
			for _, s := range o.extraSels {
				if !p.Less(s[0]) && p.Less(s[1]) {
					return true
				}
			}
			return false
		}
		isExtra := func(col int) bool {
			for _, c := range o.extraCursors {
				if c.Row == row && c.Col == col {
					return true
				}
			}
			return false
		}
		// underline the ranges of diagnostics on this line
		underline := func(col int) string {
			best := ""
			bestRank := 99
			for _, dg := range o.diags {
				if dg.Range.Start.Line > row || dg.Range.End.Line < row {
					continue
				}
				s, e := 0, len(line)+1
				if dg.Range.Start.Line == row {
					s = lsp.FromUTF16(line, dg.Range.Start.Character)
				}
				if dg.Range.End.Line == row {
					e = lsp.FromUTF16(line, dg.Range.End.Character)
				}
				if e == s {
					e = s + 1
				}
				if col >= s && col < e && sevRank(dg.Severity) < bestRank {
					best, bestRank = diagColor(dg.Severity), sevRank(dg.Severity)
				}
			}
			return best
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
			cursorHere := o.focused && (row == o.cursorAt.Row && col == o.cursorAt.Col || isExtra(col))
			var r rune = ' '
			rw := 1
			if col < len(line) {
				r = line[col]
				rw = buffer.RuneWidth(r, vis)
			} else if !cursorHere && !inSel(col) {
				break
			}
			fg := cEditorFg
			if lc != nil && col < len(lc) && lc[col] != "" {
				fg = lc[col]
			}
			k := cellStyle{fg: fg, bg: cellBg(col)}
			if hasDiag && col < len(line) {
				if u := underline(col); u != "" {
					k.under = u
					k.fg = u
					if lc != nil && col < len(lc) && lc[col] != "" {
						k.fg = lc[col]
					}
				}
			}
			if cursorHere {
				k.cursor = o.cursorColor
			}
			if r == '\t' || col == len(line) {
				for c := 0; c < rw; c++ {
					if vis+c >= b.ScrollCol && drawn < textW {
						kk := k
						if c > 0 {
							kk.cursor = ""
						}
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
				put(k, strings.Repeat(" ", vis+rw-b.ScrollCol))
				drawn += vis + rw - b.ScrollCol
			}
			vis += rw
		}
		flush()
		// inline diagnostic message (like Helix's inline diagnostics)
		if hasDiag && drawn+6 < textW {
			msg := "   ● " + firstLine(d.Message)
			msg = fit(msg, textW-drawn)
			sb.WriteString(bgfg(lineBg, diagColor(d.Severity)).Faint(true).Render(msg))
			drawn = textW
		}
		if drawn < textW {
			sb.WriteString(styleFor(cellStyle{fg: cEditorFg, bg: lineBg}).Render(strings.Repeat(" ", textW-drawn)))
		}
		out = append(out, sb.String())
	}
	return out
}

func sevRank(s int) int {
	if s == 0 {
		return 1
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func renderWelcome(w, h int, helix bool) []string {
	type row struct {
		text string
		head bool
	}
	msg := []row{{"gh-editor", true}, {"", false}}
	if helix {
		msg = append(msg,
			row{"Space f       ファイルを開く", false},
			row{"Space ?       コマンドパレット", false},
			row{"Space e       ファイルツリーへ移動", false},
			row{"Space g       プルリクエストをレビュー", false},
			row{"Ctrl+G        git-diff モード（変更を差分で見る）", false},
			row{"Ctrl+J        ターミナル", false},
			row{":q            終了", false},
		)
	} else {
		msg = append(msg,
			row{"Ctrl+P        ファイルを開く", false},
			row{"F1 / Ctrl+K   コマンドパレット", false},
			row{"Ctrl+B        サイドバーの表示切替", false},
			row{"Ctrl+E        ファイルツリーへ移動", false},
			row{"Ctrl+G        git-diff モード（変更を差分で見る）", false},
			row{"Ctrl+J        ターミナル", false},
			row{"Ctrl+Q        終了", false},
		)
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
		pad := max(0, (w-34)/2)
		st := stWelcome
		if m.head {
			pad = max(0, (w-width)/2)
			st = stWelcomeH
		}
		out[top+i] = st.Render(fit(strings.Repeat(" ", pad)+m.text, w))
	}
	return out
}
