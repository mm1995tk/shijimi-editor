package tui

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
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
	fg, bg                      screen.Style // empty = the editor colors
	bold, italic, under, strike bool
}

type mdCell struct {
	r  rune
	st mdStyle
}

var (
	mdLinkRe    = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s]*)[^)]*\)`)
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
				st.fg, st.bg = screen.StyleMdCode, screen.StyleMdCodeBg
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
				st.fg, st.under = screen.StyleMdLink, true
				out = append(out, inlineCells(m[1], st)...)
				i += len([]rune(m[0]))
				continue
			}
			add("[", base)
			i++
		case rs[i] == '!' && strings.HasPrefix(rest, "!["):
			if m := mdLinkRe.FindStringSubmatch(rest[1:]); m != nil {
				st := base
				st.fg = screen.StyleMdImage
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
	lines, _ := mdWrapSep(cells, w)
	return lines
}

// mdWrapSep is mdWrap that also reports, for each line, whether a space was
// dropped at the break after it (false for a word cut in the middle).
func mdWrapSep(cells []mdCell, w int) ([][]mdCell, []bool) {
	if w < 1 {
		w = 1
	}
	var lines [][]mdCell
	var seps []bool
	var cur []mdCell
	curW := 0
	flush := func(space bool) {
		for len(cur) > 0 && cur[len(cur)-1].r == ' ' {
			cur = cur[:len(cur)-1]
		}
		lines = append(lines, cur)
		seps = append(seps, space)
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
				flush(true)
				continue
			}
			if brk >= 0 {
				rest := append([]mdCell(nil), cur[brk+1:]...)
				cur = cur[:brk]
				flush(true)
				cur = rest
				curW = 0
				for _, r := range rest {
					curW += runewidth.RuneWidth(r.r)
				}
			} else {
				flush(false)
			}
		}
		cur = append(cur, c)
		curW += cw
	}
	if len(cur) > 0 || len(lines) == 0 {
		flush(false)
	}
	return lines, seps
}

func cellsWidth(cs []mdCell) int {
	n := 0
	for _, c := range cs {
		n += runewidth.RuneWidth(c.r)
	}
	return n
}

// mdSpans turns styled runes into spans, one per run of the same style.
func mdSpans(cs []mdCell) []screen.Span {
	var out []screen.Span
	for i := 0; i < len(cs); {
		j := i
		for j < len(cs) && cs[j].st == cs[i].st {
			j++
		}
		run := make([]rune, 0, j-i)
		for _, c := range cs[i:j] {
			run = append(run, c.r)
		}
		st := cs[i].st
		out = append(out, screen.Span{
			Text: string(run), Style: st.fg, Bg: st.bg,
			Bold: st.bold, Italic: st.italic, Underline: st.under, Strike: st.strike,
		})
		i = j
	}
	return out
}

func plainText(cs []mdCell) string {
	r := make([]rune, len(cs))
	for i, c := range cs {
		r[i] = c.r
	}
	return string(r)
}

// mdDoc is a Markdown document rendered at one width: the lines (what the
// preview Pane carries) and where the images go.
type mdDoc struct {
	Lines  []screen.Line
	Images []screen.PreviewImage
}

// mdMedia is what mdOpts.mermaid and mdOpts.image return. Image is nil when
// Lines are the rows to show as they are. Otherwise Lines are the empty rows
// the image fills, and Image names it.
type mdMedia struct {
	Lines []screen.Line
	Image *screen.ImageRef

	// Tight says the last row of the image is left empty (a small image is
	// centered in its rows), so it counts as the empty line after the block.
	Tight bool
}

// mdOpts supplies the pieces the renderer cannot produce itself.
type mdOpts struct {
	// mermaid returns the rows for a diagram (each at most w wide), or nil
	// to show the source as a plain code block.
	mermaid func(code string, w int) *mdMedia
	// image returns the rows for a paragraph that is a single image, or nil
	// to show it as text.
	image func(alt, src string, w int) *mdMedia
	// wrapped, when set, is called for each output row that continues a
	// paragraph or list item cut by the width. space says whether the cut
	// fell on a space. indent is the width of the
	// decoration before its text.
	wrapped func(row int, space bool, indent int)
	// exact, when set, is called for each row of a code line cut by the
	// width: its text is columns [from, to) as they are, spaces included.
	exact func(row, from, to int)
	// frontMatter shows a leading block fenced by "---" lines as YAML
	// metadata instead of rules around a paragraph.
	frontMatter bool
}

// plainSpan is text in the editor colors.
func plainSpan(text string) screen.Span { return screen.Span{Text: text} }

// mdRow is a line of the given spans.
func mdRow(spans ...screen.Span) screen.Line {
	if spans == nil {
		spans = []screen.Span{}
	}
	return screen.Line{Spans: spans}
}

// spansWidth is the width in cells of spans.
func spansWidth(spans []screen.Span) int {
	n := 0
	for _, sp := range spans {
		n += ansi.StringWidth(sp.Text)
	}
	return n
}

// withText is sp with other text.
func withText(sp screen.Span, text string) screen.Span {
	sp.Text = text
	return sp
}

// cutSpans keeps the cells [skip, skip+w) of spans, measured as the terminal
// draws them (a tab is four cells, a wide rune cut by the left edge leaves
// blanks).
func cutSpans(spans []screen.Span, skip, w int) []screen.Span {
	if w <= 0 {
		return nil
	}
	var out []screen.Span
	used, col := 0, 0 // cells kept, cells passed
	add := func(s string, sp screen.Span) {
		if n := len(out); n > 0 {
			last := withText(out[n-1], "")
			if last == withText(sp, "") {
				out[n-1].Text += s
				return
			}
		}
		out = append(out, withText(sp, s))
	}
	for _, sp := range spans {
		for g := uniseg.NewGraphemes(sp.Text); g.Next(); {
			ch := g.Str()
			if ch == "\t" {
				ch = "    "
			}
			cw := ansi.StringWidth(ch)
			if cw == 0 && skip > 0 && used == 0 { // a combining mark of the rune left of the view
				continue
			}
			if col < skip { // left of the view; a wide rune cut by the edge leaves blanks
				col += cw
				if col <= skip {
					continue
				}
				ch = strings.Repeat(" ", col-skip)
				cw = col - skip
			}
			if used+cw > w {
				return out
			}
			add(ch, sp)
			used += cw
		}
		if used >= w {
			break
		}
	}
	return out
}

// fitSpans makes spans exactly w cells wide: a longer line ends with "…", a
// shorter one is padded in the editor colors.
func fitSpans(spans []screen.Span, w int) []screen.Span {
	if w <= 0 {
		return nil
	}
	sw := spansWidth(spans)
	if sw > w {
		spans = cutSpans(spans, 0, w-1)
		tail := plainSpan("…")
		if n := len(spans); n > 0 {
			tail = withText(spans[n-1], "…")
		}
		spans = append(spans, tail)
		sw = spansWidth(spans)
	}
	if sw < w {
		spans = append(spans, plainSpan(strings.Repeat(" ", w-sw)))
	}
	return spans
}

// renderMarkdown formats src into lines for a pane w columns wide. The lines
// are not padded to w: the drawing side fills the rest with the editor
// background. The structure of src is parseMarkdown's; the widths are here.
func renderMarkdown(src string, w int, o mdOpts) mdDoc {
	return formatMarkdown(parseMarkdown(src, o.frontMatter), w, o)
}

// formatMarkdown lays the blocks of a document out for a pane w columns wide.
func formatMarkdown(blocks []mdBlock, w int, o mdOpts) mdDoc {
	var out []screen.Line
	var images []screen.PreviewImage
	gapAt := -1     // the index in out of the empty line that separates blocks
	var seps []bool // of the lines wrap returned last
	wrap := func(cells []mdCell, w int) [][]mdCell {
		var lines [][]mdCell
		lines, seps = mdWrapSep(cells, w)
		return lines
	}
	emit := func(prefix screen.Span, cells [][]mdCell, contPrefix screen.Span) {
		pw := ansi.StringWidth(prefix.Text)
		for i, l := range cells {
			p := prefix
			if i > 0 {
				p = contPrefix
				if o.wrapped != nil && i-1 < len(seps) {
					o.wrapped(len(out), seps[i-1], pw)
				}
			}
			var spans []screen.Span
			if p.Text != "" {
				spans = append(spans, p)
			}
			out = append(out, mdRow(append(spans, mdSpans(l)...)...))
		}
	}
	gap := func() {
		if len(out) > 0 && gapAt != len(out)-1 {
			out = append(out, mdRow())
			gapAt = len(out) - 1
		}
	}
	media := func(m *mdMedia) {
		if m.Image != nil {
			images = append(images, screen.PreviewImage{Row: len(out), Rows: len(m.Lines), Image: *m.Image})
		}
		out = append(out, m.Lines...)
		if m.Tight {
			gapAt = len(out) - 1
		}
	}
	rule := func() {
		out = append(out, mdRow(screen.Span{Text: strings.Repeat("─", w), Style: screen.StyleBorder}))
	}

	codeBlock := func(code []string, lang string) {
		rows, pieces := mdCodeRows(code, lang, w)
		for row, p := range pieces {
			if p.cont && o.wrapped != nil {
				o.wrapped(len(out)+row, false, 1)
			}
			if o.exact != nil {
				o.exact(len(out)+row, 1, 1+p.width)
			}
		}
		out = append(out, rows...)
	}

	for _, b := range blocks {
		switch b.Kind {
		case mdGap:
			gap()

		case mdPara:
			if alt, src, ok := soleImage(b.Text); ok && o.image != nil {
				if rendered := o.image(alt, src, w); rendered != nil {
					media(rendered)
					gap()
					continue
				}
			}
			emit(screen.Span{}, wrap(inlineCells(b.Text, mdStyle{}), w), screen.Span{})
			gap()

		case mdFence:
			if strings.EqualFold(b.Lang, "mermaid") && o.mermaid != nil {
				if rendered := o.mermaid(strings.Join(b.Code, "\n"), w); rendered != nil {
					media(rendered)
					gap()
					continue
				}
			}
			codeBlock(b.Code, b.Lang)
			gap()

		case mdRule:
			rule()
			gap()

		case mdHeading:
			gap()
			fg := map[int]screen.Style{1: screen.StyleMdH1, 2: screen.StyleMdH2}[b.Level]
			if fg == "" {
				fg = screen.StyleMdH3
			}
			cells := wrap(inlineCells(b.Text, mdStyle{fg: fg, bold: true}), w)
			emit(screen.Span{}, cells, screen.Span{})
			if b.Level <= 2 {
				rule()
			}
			gap()

		case mdQuote:
			bar := screen.Span{Text: "▎ ", Style: screen.StyleMdQuote}
			qo, base := o, len(out)
			qo.frontMatter = false
			if o.wrapped != nil {
				qo.wrapped = func(row int, space bool, indent int) { o.wrapped(base+row, space, indent+2) }
			}
			if o.exact != nil {
				qo.exact = func(row, from, to int) { o.exact(base+row, from+2, to+2) }
			}
			inner := formatMarkdown(b.Inner, max(1, w-2), qo)
			for _, l := range inner.Lines {
				l.Spans = append([]screen.Span{bar}, l.Spans...)
				out = append(out, l)
			}
			for _, im := range inner.Images {
				im.Row += base
				im.Col += 2
				images = append(images, im)
			}
			gap()

		case mdTableBlock:
			out = append(out, mdTable(b.Rows, b.Aligns, w)...)
			gap()

		case mdList:
			pw := ansi.StringWidth(b.Prefix)
			cells := wrap(inlineCells(b.Text, mdStyle{}), max(1, w-pw))
			emit(screen.Span{Text: b.Prefix, Style: screen.StyleMdListMark}, cells, plainSpan(strings.Repeat(" ", pw)))
			if b.GapAfter {
				gap()
			}
		}
	}
	for len(out) > 0 && gapAt == len(out)-1 {
		out = out[:len(out)-1]
		gapAt = -1
	}
	return mdDoc{Lines: out, Images: images}
}

// soleImage reports whether a paragraph is one image and nothing else, and
// returns its alt text and source.
func soleImage(text string) (alt, src string, ok bool) {
	if !strings.HasPrefix(text, "![") {
		return "", "", false
	}
	m := mdLinkRe.FindStringSubmatch(text[1:])
	if m == nil || len(m[0]) != len(text)-1 {
		return "", "", false
	}
	return m[1], m[2], true
}

func mdCodeBlock(code []string, lang string, w int) []screen.Line {
	rows, _ := mdCodeRows(code, lang, w)
	return rows
}

// codePiece is one row of a code line cut by the width of its block.
type codePiece struct {
	cont  bool // continues the row above
	width int  // columns of text after the left edge
}

// mdCodeRows makes a code block, wrapping lines longer than the block.
// pieces holds the rows of the lines that were cut.
func mdCodeRows(code []string, lang string, w int) (rows []screen.Line, pieces map[int]codePiece) {
	var lex chroma.Lexer
	if lang != "" {
		lex = lexers.Get(lang)
	}
	if lex != nil {
		lex = chroma.Coalesce(lex)
	}
	// the edge cells and the text are on the side background; the gap right
	// of a short line is on the editor background
	const bg = screen.StyleSideBg
	edge := screen.Span{Text: " ", Style: bg, Bg: bg}
	blank := mdRow(screen.Span{Text: strings.Repeat(" ", max(0, w)), Style: bg, Bg: bg})
	var out []screen.Line
	if lang != "" {
		out = append(out, mdRow(screen.Span{Text: fit(" "+lang, w), Style: screen.StyleLineNo, Bg: bg}))
	} else {
		out = append(out, blank)
	}
	pieces = map[int]codePiece{}
	tw := w - 2
	for _, l := range code {
		// cut the line into pieces of at most tw columns, measured as
		// the spans are drawn; each piece is cut out of the whole line,
		// skipping the columns before it
		var widths []int
		piece := 0
		for g := uniseg.NewGraphemes(l); g.Next(); {
			cells := []int{ansi.StringWidth(g.Str())}
			if g.Str() == "\t" {
				cells = []int{1, 1, 1, 1} // a tab may be cut like the spaces it is drawn as
			}
			for _, cw := range cells {
				if tw > 0 && piece > 0 && piece+cw > tw {
					widths, piece = append(widths, piece), 0
				}
				piece += cw
			}
		}
		widths = append(widths, piece)
		col := 0
		for i, pw := range widths {
			if len(widths) > 1 {
				pieces[len(out)] = codePiece{cont: i > 0, width: min(pw, tw)} // a rune wider than the block is drawn as "…"
			}
			out = append(out, codeRow(lexSpans(l, lex), edge, col, pw, tw))
			col += pw
		}
	}
	out = append(out, blank)
	return out, pieces
}

// codeRow is the row of a code block for columns [col, col+pw) of the line
// whose spans are given: between the edge cells, tw cells wide.
func codeRow(spans []screen.Span, edge screen.Span, col, pw, tw int) screen.Line {
	if tw <= 0 {
		return mdRow(edge, edge)
	}
	text := cutSpans(spans, col, pw)
	if pw > tw { // a rune wider than the block
		text = cutSpans(text, 0, tw-1)
		text = append(text, plainSpan("…"))
		pw = tw
	}
	for i := range text {
		text[i].Bg = edge.Bg
	}
	row := append([]screen.Span{edge}, text...)
	if n := pw - spansWidth(text); n > 0 {
		row = append(row, screen.Span{Text: strings.Repeat(" ", n), Bg: edge.Bg})
	}
	if n := tw - pw; n > 0 {
		row = append(row, plainSpan(strings.Repeat(" ", n)))
	}
	return mdRow(append(row, edge)...)
}

func mdTable(rows [][]string, aligns []string, w int) []screen.Line {
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
				st.bold, st.fg = true, screen.StyleMdH3
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
	border := func(s string) screen.Span { return screen.Span{Text: s, Style: screen.StyleBorder} }
	rule := func(l, m, r string) screen.Line {
		parts := make([]string, cols)
		for i, cw := range widths {
			parts[i] = strings.Repeat("─", cw+2)
		}
		return mdRow(fitSpans([]screen.Span{border(l + strings.Join(parts, m) + r)}, w)...)
	}
	var out []screen.Line
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
			spans := []screen.Span{border("│")}
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
				spans = append(spans, plainSpan(" "+strings.Repeat(" ", left)))
				spans = append(spans, mdSpans(cs)...)
				spans = append(spans, plainSpan(strings.Repeat(" ", padN-left)+" "), border("│"))
			}
			out = append(out, mdRow(fitSpans(spans, w)...))
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

