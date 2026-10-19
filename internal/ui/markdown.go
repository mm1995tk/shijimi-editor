package ui

import (
	"path/filepath"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/mattn/go-runewidth"
)

func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// mdStyle is the look of one run of inline text.
type mdStyle struct {
	fg                          string
	bg                          string
	bold, italic, under, strike bool
}

var mdStyleCache = map[mdStyle]lipgloss.Style{}

func (s mdStyle) render(text string) string {
	st, ok := mdStyleCache[s]
	if !ok {
		bg := s.bg
		if bg == "" {
			bg = cEditorBg
		}
		fg := s.fg
		if fg == "" {
			fg = cEditorFg
		}
		st = bgfg(bg, fg).Bold(s.bold).Italic(s.italic).Underline(s.under).Strikethrough(s.strike)
		mdStyleCache[s] = st
	}
	return st.Render(text)
}

type mdCell struct {
	r  rune
	st mdStyle
}

var (
	mdLinkRe    = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s]*)[^)]*\)`)
	mdOrderedRe = regexp.MustCompile(`^(\s*)(\d+)[.)]\s+(.*)$`)
	mdBulletRe  = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	mdFenceRe   = regexp.MustCompile("^\\s*(```+|~~~+)\\s*([^\\s`]*)")
	mdTableSep  = regexp.MustCompile(`^\s*\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?\s*$`)
)

// inlineCells parses inline markdown (code, bold, italic, strike, links)
// into styled runes.
func inlineCells(s string, base mdStyle) []mdCell {
	var out []mdCell
	add := func(text string, st mdStyle) {
		for _, r := range text {
			out = append(out, mdCell{r, st})
		}
	}
	rs := []rune(s)
	for i := 0; i < len(rs); {
		rest := string(rs[i:])
		switch {
		case rs[i] == '\\' && i+1 < len(rs):
			add(string(rs[i+1]), base)
			i += 2
		case rs[i] == '`':
			n := 1
			for i+n < len(rs) && rs[i+n] == '`' {
				n++
			}
			fence := strings.Repeat("`", n)
			if end := strings.Index(rest[n:], fence); end >= 0 {
				st := base
				st.fg, st.bg = "#ce9178", "#2d2d2d"
				add(strings.TrimSpace(rest[n:n+end]), st)
				i += len([]rune(rest[:n+end+n]))
				continue
			}
			add(fence, base)
			i += n
		case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
			if end := strings.Index(rest[2:], rest[:2]); end > 0 {
				st := base
				st.bold = true
				out = append(out, inlineCells(rest[2:2+end], st)...)
				i += len([]rune(rest[:2+end+2]))
				continue
			}
			add(rest[:2], base)
			i += 2
		case strings.HasPrefix(rest, "~~"):
			if end := strings.Index(rest[2:], "~~"); end > 0 {
				st := base
				st.strike = true
				out = append(out, inlineCells(rest[2:2+end], st)...)
				i += len([]rune(rest[:2+end+2]))
				continue
			}
			add("~~", base)
			i += 2
		case (rs[i] == '*' || rs[i] == '_') && i+1 < len(rs) && rs[i+1] != ' ' && (i == 0 || rs[i] == '*' || rs[i-1] == ' '):
			mark := string(rs[i])
			if end := strings.Index(rest[1:], mark); end > 0 {
				st := base
				st.italic = true
				out = append(out, inlineCells(rest[1:1+end], st)...)
				i += len([]rune(rest[:1+end+1]))
				continue
			}
			add(mark, base)
			i++
		case rs[i] == '[':
			if m := mdLinkRe.FindStringSubmatch(rest); m != nil {
				st := base
				st.fg, st.under = "#4fc1ff", true
				out = append(out, inlineCells(m[1], st)...)
				i += len([]rune(m[0]))
				continue
			}
			add("[", base)
			i++
		case rs[i] == '!' && strings.HasPrefix(rest, "!["):
			if m := mdLinkRe.FindStringSubmatch(rest[1:]); m != nil {
				st := base
				st.fg = "#6e6e6e"
				add("🖼 "+m[1], st)
				i += 1 + len([]rune(m[0]))
				continue
			}
			add("!", base)
			i++
		default:
			add(string(rs[i]), base)
			i++
		}
	}
	return out
}

// mdWrap breaks cells into lines of at most w columns, preferring spaces.
func mdWrap(cells []mdCell, w int) [][]mdCell {
	if w < 1 {
		w = 1
	}
	var lines [][]mdCell
	var cur []mdCell
	curW := 0
	flush := func() {
		for len(cur) > 0 && cur[len(cur)-1].r == ' ' {
			cur = cur[:len(cur)-1]
		}
		lines = append(lines, cur)
		cur, curW = nil, 0
	}
	for _, c := range cells {
		cw := runewidth.RuneWidth(c.r)
		if c.r == ' ' && curW == 0 && len(lines) > 0 {
			continue
		}
		if curW+cw > w {
			brk := -1
			for i := len(cur) - 1; i > 0; i-- {
				if cur[i].r == ' ' {
					brk = i
					break
				}
			}
			if c.r == ' ' {
				flush()
				continue
			}
			if brk >= 0 {
				rest := append([]mdCell(nil), cur[brk+1:]...)
				cur = cur[:brk]
				flush()
				cur = rest
				curW = 0
				for _, r := range rest {
					curW += runewidth.RuneWidth(r.r)
				}
			} else {
				flush()
			}
		}
		cur = append(cur, c)
		curW += cw
	}
	if len(cur) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

func cellsWidth(cs []mdCell) int {
	n := 0
	for _, c := range cs {
		n += runewidth.RuneWidth(c.r)
	}
	return n
}

func mdRender(cs []mdCell) string {
	var sb strings.Builder
	for i := 0; i < len(cs); {
		j := i
		for j < len(cs) && cs[j].st == cs[i].st {
			j++
		}
		run := make([]rune, 0, j-i)
		for _, c := range cs[i:j] {
			run = append(run, c.r)
		}
		sb.WriteString(cs[i].st.render(string(run)))
		i = j
	}
	return sb.String()
}

func plainText(cs []mdCell) string {
	r := make([]rune, len(cs))
	for i, c := range cs {
		r[i] = c.r
	}
	return string(r)
}

// mdOpts supplies the pieces the renderer cannot produce itself.
type mdOpts struct {
	// mermaid returns the rendered lines (exactly w wide) for a diagram, or
	// nil to show the source as a plain code block.
	mermaid func(code string, w int) []string
}

// renderMarkdown formats src into lines of exactly w columns.
func renderMarkdown(src string, w int, o mdOpts) []string {
	var out []string
	blank := stEmpty.Render(strings.Repeat(" ", w))
	emit := func(prefix string, cells [][]mdCell, contPrefix string) {
		pw := lipgloss.Width(prefix)
		for i, l := range cells {
			p := prefix
			if i > 0 {
				p = contPrefix
			}
			line := p + mdRender(l)
			if rest := w - pw - cellsWidth(l); rest > 0 {
				line += stEmpty.Render(strings.Repeat(" ", rest))
			}
			out = append(out, line)
		}
	}
	gap := func() {
		if len(out) > 0 && out[len(out)-1] != blank {
			out = append(out, blank)
		}
	}

	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var para []string
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		text := strings.Join(para, " ")
		emit("", mdWrap(inlineCells(text, mdStyle{}), w), "")
		para = nil
		gap()
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")

		if m := mdFenceRe.FindStringSubmatch(line); m != nil {
			flushPara()
			fence, lang := m[1], m[2]
			var code []string
			for i++; i < len(lines); i++ {
				if t := strings.TrimSpace(lines[i]); len(t) >= len(fence) && strings.Trim(t, string(fence[0])) == "" {
					break
				}
				code = append(code, lines[i])
			}
			text := strings.Join(code, "\n")
			if strings.EqualFold(lang, "mermaid") && o.mermaid != nil {
				if rendered := o.mermaid(text, w); rendered != nil {
					out = append(out, rendered...)
					gap()
					continue
				}
			}
			out = append(out, mdCodeBlock(code, lang, w)...)
			gap()
			continue
		}

		switch {
		case strings.TrimSpace(line) == "":
			flushPara()
			gap()
			continue
		case isRule(line):
			flushPara()
			out = append(out, bgfg(cEditorBg, cBorder).Render(strings.Repeat("─", w)))
			gap()
			continue
		}

		if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
			flushPara()
			gap()
			level := len(m[1])
			fg := map[int]string{1: "#4fc1ff", 2: "#569cd6"}[level]
			if fg == "" {
				fg = "#9cdcfe"
			}
			cells := mdWrap(inlineCells(m[2], mdStyle{fg: fg, bold: true}), w)
			emit("", cells, "")
			if level <= 2 {
				out = append(out, bgfg(cEditorBg, cBorder).Render(strings.Repeat("─", w)))
			}
			gap()
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			flushPara()
			var q []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				q = append(q, strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"), " "))
			}
			i--
			bar := bgfg(cEditorBg, "#6a9955").Render("▎ ")
			for _, l := range renderMarkdown(strings.Join(q, "\n"), max(1, w-2), o) {
				out = append(out, bar+l)
			}
			gap()
			continue
		}

		if i+1 < len(lines) && strings.Contains(line, "|") && mdTableSep.MatchString(lines[i+1]) && strings.Contains(lines[i+1], "-") {
			flushPara()
			rows := [][]string{splitRow(line)}
			aligns := splitRow(lines[i+1])
			for i += 2; i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != ""; i++ {
				rows = append(rows, splitRow(lines[i]))
			}
			i--
			out = append(out, mdTable(rows, aligns, w)...)
			gap()
			continue
		}

		if lm := listItem(line); lm != nil {
			flushPara()
			depth := lm.indent / 2
			prefix := strings.Repeat("  ", depth) + lm.marker + " "
			// continuation lines belonging to this item
			text := lm.text
			for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && listItem(lines[i+1]) == nil && strings.HasPrefix(lines[i+1], " ") && !mdFenceRe.MatchString(lines[i+1]) {
				i++
				text += " " + strings.TrimSpace(lines[i])
			}
			pw := lipgloss.Width(prefix)
			mk := bgfg(cEditorBg, "#9cdcfe")
			var cells [][]mdCell
			if strings.HasPrefix(text, "[ ] ") || strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ") {
				box := "☐ "
				if text[1] != ' ' {
					box = "☑ "
				}
				text = text[4:]
				prefix += box
				pw += 2
			}
			cells = mdWrap(inlineCells(text, mdStyle{}), max(1, w-pw))
			emit(mk.Render(prefix), cells, stEmpty.Render(strings.Repeat(" ", pw)))
			if i+1 >= len(lines) || listItem(lines[i+1]) == nil {
				gap()
			}
			continue
		}

		para = append(para, strings.TrimSpace(line))
	}
	flushPara()
	for len(out) > 0 && out[len(out)-1] == blank {
		out = out[:len(out)-1]
	}
	return out
}

type listMatch struct {
	indent int
	marker string
	text   string
}

func listItem(line string) *listMatch {
	if m := mdBulletRe.FindStringSubmatch(line); m != nil {
		return &listMatch{len(m[1]), "•", m[2]}
	}
	if m := mdOrderedRe.FindStringSubmatch(line); m != nil {
		return &listMatch{len(m[1]), m[2] + ".", m[3]}
	}
	return nil
}

func mdCodeBlock(code []string, lang string, w int) []string {
	var lex chroma.Lexer
	if lang != "" {
		lex = lexers.Get(lang)
	}
	if lex != nil {
		lex = chroma.Coalesce(lex)
	}
	bg := cSideBg
	edge := styleFor(cellStyle{fg: cSideBg, bg: bg})
	var out []string
	if lang != "" {
		out = append(out, bgfg(bg, cLineNo).Render(fit(" "+lang, w)))
	} else {
		out = append(out, edge.Render(strings.Repeat(" ", w)))
	}
	for _, l := range code {
		out = append(out, edge.Render(" ")+fitEmpty(highlightText(l, lex, bg, w-2), w-2)+edge.Render(" "))
	}
	out = append(out, edge.Render(strings.Repeat(" ", w)))
	return out
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func mdTable(rows [][]string, aligns []string, w int) []string {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	cells := make([][][]mdCell, len(rows))
	widths := make([]int, cols)
	for ri, r := range rows {
		cells[ri] = make([][]mdCell, cols)
		for ci := 0; ci < cols; ci++ {
			txt := ""
			if ci < len(r) {
				txt = r[ci]
			}
			st := mdStyle{}
			if ri == 0 {
				st.bold, st.fg = true, "#9cdcfe"
			}
			cells[ri][ci] = inlineCells(txt, st)
			widths[ci] = max(widths[ci], cellsWidth(cells[ri][ci]))
		}
	}
	// shrink the widest columns until the table fits
	avail := w - (3*cols + 1)
	for sum(widths) > avail && avail >= cols {
		wi := 0
		for i, v := range widths {
			if v > widths[wi] {
				wi = i
			}
		}
		widths[wi]--
	}
	bd := bgfg(cEditorBg, cBorder)
	rule := func(l, m, r string) string {
		parts := make([]string, cols)
		for i, cw := range widths {
			parts[i] = strings.Repeat("─", cw+2)
		}
		return fitEmpty(bd.Render(l+strings.Join(parts, m)+r), w)
	}
	var out []string
	out = append(out, rule("┌", "┬", "┐"))
	for ri := range rows {
		// wrap each cell to its column; the row is as tall as its tallest cell
		wrapped := make([][][]mdCell, cols)
		height := 1
		for ci := 0; ci < cols; ci++ {
			wrapped[ci] = mdWrap(cells[ri][ci], widths[ci])
			height = max(height, len(wrapped[ci]))
		}
		for li := 0; li < height; li++ {
			var sb strings.Builder
			sb.WriteString(bd.Render("│"))
			for ci := 0; ci < cols; ci++ {
				var cs []mdCell
				if li < len(wrapped[ci]) {
					cs = wrapped[ci][li]
				}
				// a wide rune can still overflow a 1-column cell
				for cellsWidth(cs) > widths[ci] && len(cs) > 0 {
					cs = cs[:len(cs)-1]
				}
				padN := widths[ci] - cellsWidth(cs)
				left := 0
				if ci < len(aligns) {
					a := strings.TrimSpace(aligns[ci])
					switch {
					case strings.HasPrefix(a, ":") && strings.HasSuffix(a, ":"):
						left = padN / 2
					case strings.HasSuffix(a, ":"):
						left = padN
					}
				}
				sb.WriteString(stEmpty.Render(" " + strings.Repeat(" ", left)))
				sb.WriteString(mdRender(cs))
				sb.WriteString(stEmpty.Render(strings.Repeat(" ", padN-left) + " "))
				sb.WriteString(bd.Render("│"))
			}
			out = append(out, fitEmpty(sb.String(), w))
		}
		if ri == 0 {
			out = append(out, rule("├", "┼", "┤"))
		}
	}
	out = append(out, rule("└", "┴", "┘"))
	return out
}

func sum(v []int) int {
	n := 0
	for _, x := range v {
		n += x
	}
	return n
}

// fitEmpty is fit, but pads with the editor background.
func fitEmpty(s string, w int) string {
	if sw := lipgloss.Width(s); sw < w {
		return s + stEmpty.Render(strings.Repeat(" ", w-sw))
	}
	return fit(s, w)
}

// isRule reports whether line is a thematic break (---, ***, ___).
func isRule(line string) bool {
	s := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	if len(s) < 3 || strings.Trim(s, string(s[0])) != "" {
		return false
	}
	return s[0] == '-' || s[0] == '*' || s[0] == '_'
}
