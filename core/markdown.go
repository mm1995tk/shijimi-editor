package core

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core/screen"
)

func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

var (
	mdLinkRe = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s]*)[^)]*\)`)
)

// inlineSpans parses inline markdown (code, bold, italic, strike, links)
// into runs of styled text.
func inlineSpans(s string, base screen.Span) []screen.Span {
	var out []screen.Span
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			sp := base
			sp.Text = literal.String()
			out = append(out, sp)
			literal.Reset()
		}
	}
	add := func(text string, st screen.Span) {
		flush()
		st.Text = text
		out = append(out, st)
	}
	rs := []rune(s)
	for i := 0; i < len(rs); {
		rest := string(rs[i:])
		switch {
		case rs[i] == '\\' && i+1 < len(rs):
			literal.WriteRune(rs[i+1])
			i += 2
		case rs[i] == '`':
			n := 1
			for i+n < len(rs) && rs[i+n] == '`' {
				n++
			}
			fence := strings.Repeat("`", n)
			if end := strings.Index(rest[n:], fence); end >= 0 {
				st := base
				st.Style, st.Bg = screen.StyleMdCode, screen.StyleMdCodeBg
				add(strings.TrimSpace(rest[n:n+end]), st)
				i += len([]rune(rest[:n+end+n]))
				continue
			}
			literal.WriteString(fence)
			i += n
		case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
			if end := strings.Index(rest[2:], rest[:2]); end > 0 {
				st := base
				st.Bold = true
				flush()
				out = append(out, inlineSpans(rest[2:2+end], st)...)
				i += len([]rune(rest[:2+end+2]))
				continue
			}
			literal.WriteString(rest[:2])
			i += 2
		case strings.HasPrefix(rest, "~~"):
			if end := strings.Index(rest[2:], "~~"); end > 0 {
				st := base
				st.Strike = true
				flush()
				out = append(out, inlineSpans(rest[2:2+end], st)...)
				i += len([]rune(rest[:2+end+2]))
				continue
			}
			add("~~", base)
			i += 2
		case (rs[i] == '*' || rs[i] == '_') && i+1 < len(rs) && rs[i+1] != ' ' && (i == 0 || rs[i] == '*' || rs[i-1] == ' '):
			mark := string(rs[i])
			if end := strings.Index(rest[1:], mark); end > 0 {
				st := base
				st.Italic = true
				flush()
				out = append(out, inlineSpans(rest[1:1+end], st)...)
				i += len([]rune(rest[:1+end+1]))
				continue
			}
			literal.WriteString(mark)
			i++
		case rs[i] == '[':
			if m := mdLinkRe.FindStringSubmatch(rest); m != nil {
				st := base
				st.Style, st.Underline = screen.StyleMdLink, true
				flush()
				out = append(out, inlineSpans(m[1], st)...)
				i += len([]rune(m[0]))
				continue
			}
			literal.WriteString("[")
			i++
		case rs[i] == '!' && strings.HasPrefix(rest, "!["):
			if m := mdLinkRe.FindStringSubmatch(rest[1:]); m != nil {
				st := base
				st.Style = screen.StyleMdImage
				add("🖼 "+m[1], st)
				i += 1 + len([]rune(m[0]))
				continue
			}
			literal.WriteString("!")
			i++
		default:
			literal.WriteRune(rs[i])
			i++
		}
	}
	flush()
	return out
}

func inlineText(s string, base screen.Span) mdText {
	return mdGraphemes(inlineSpans(s, base))
}

// MdDoc is a Markdown document rendered at one width: the lines (what the
// preview Pane carries) and where the images go.
type MdDoc struct {
	Lines  []screen.Line
	Images []screen.PreviewImage
}

// MdMedia is what MdOpts.mermaid and MdOpts.image return. Image is nil when
// Lines are the rows to show as they are. Otherwise Lines are the empty rows
// the image fills, and Image names it.
type MdMedia struct {
	Lines []screen.Line
	Image *screen.ImageRef

	// Tight says the last row of the image is left empty (a small image is
	// centered in its rows), so it counts as the empty line after the block.
	Tight bool
}

// MdOpts supplies the pieces the renderer cannot produce itself.
type MdOpts struct {
	// mermaid returns the rows for a diagram (each at most w wide), or nil
	// to show the source as a plain code block.
	Mermaid func(code string, w int) *MdMedia
	// image returns the rows for a paragraph that is a single image, or nil
	// to show it as text.
	image func(alt, src string, w int) *MdMedia
	// wrapped, when set, is called for each output row that continues a
	// paragraph or list item cut by the width. space says whether the cut
	// fell on a space. indent is the width of the
	// decoration before its text.
	Wrapped func(row int, space bool, indent int)
	// exact, when set, is called for each row of a code line cut by the
	// width: its text is columns [from, to) as they are, spaces included.
	Exact func(row, from, to int)
	// frontMatter shows a leading block fenced by "---" lines as YAML
	// metadata instead of rules around a paragraph.
	FrontMatter bool
}

// PlainSpan is text in the editor colors.
func PlainSpan(text string) screen.Span { return screen.Span{Text: text} }

// MdRow is a line of the given spans.
func MdRow(spans ...screen.Span) screen.Line {
	if spans == nil {
		spans = []screen.Span{}
	}
	return screen.Line{Spans: spans}
}

// fitSpans makes spans exactly w cells wide: a longer line ends with "…", a
// shorter one is padded in the editor colors.
func fitSpans(spans []screen.Span, w int) []screen.Span {
	if w <= 0 {
		return nil
	}
	text := mdGraphemes(spans)
	sw := text.width()
	if sw > w {
		text = text.prefix(w - 1)
		spans = text.spans()
		tail := PlainSpan("…")
		if n := len(spans); n > 0 {
			tail = spans[n-1]
			tail.Text = "…"
		}
		spans = append(spans, tail)
		sw = text.width() + 1
	} else {
		spans = text.spans()
	}
	if sw < w {
		spans = append(spans, PlainSpan(strings.Repeat(" ", w-sw)))
	}
	return spans
}

// RenderMarkdown formats src into lines for a pane w columns wide. The lines
// are not padded to w: the drawing side fills the rest with the editor
// background. The structure of src is ParseMarkdown's; the widths are here.
func RenderMarkdown(src string, w int, o MdOpts) MdDoc {
	return formatMarkdown(ParseMarkdown(src, o.FrontMatter), w, o)
}

// formatMarkdown lays the blocks of a document out for a pane w columns wide.
func formatMarkdown(blocks []MdBlock, w int, o MdOpts) MdDoc {
	var out []screen.Line
	var images []screen.PreviewImage
	gapAt := -1 // the index in out of the empty line that separates blocks
	emit := func(prefix screen.Span, text mdText) {
		pw := ansi.StringWidth(prefix.Text)
		lines := text.wrap(w-pw, true)
		for i, l := range lines {
			p := prefix
			if i > 0 {
				p = PlainSpan(strings.Repeat(" ", pw))
				if o.Wrapped != nil {
					o.Wrapped(len(out), lines[i-1].space, pw)
				}
			}
			var spans []screen.Span
			if p.Text != "" {
				spans = append(spans, p)
			}
			out = append(out, MdRow(append(spans, l.text.spans()...)...))
		}
	}
	gap := func() {
		if len(out) > 0 && gapAt != len(out)-1 {
			out = append(out, MdRow())
			gapAt = len(out) - 1
		}
	}
	media := func(m *MdMedia) {
		if m.Image != nil {
			images = append(images, screen.PreviewImage{Row: len(out), Rows: len(m.Lines), Image: *m.Image})
		}
		out = append(out, m.Lines...)
		if m.Tight {
			gapAt = len(out) - 1
		}
	}
	rule := func() {
		out = append(out, MdRow(screen.Span{Text: strings.Repeat("─", w), Style: screen.StyleBorder}))
	}

	codeBlock := func(code []string, lang string) {
		rows, pieces := mdCodeRows(code, lang, w)
		for row, p := range pieces {
			if p.cont && o.Wrapped != nil {
				o.Wrapped(len(out)+row, false, 1)
			}
			if o.Exact != nil {
				o.Exact(len(out)+row, 1, 1+p.width)
			}
		}
		out = append(out, rows...)
	}

	for _, b := range blocks {
		switch b.Kind {
		case MdGap:
			gap()

		case MdPara:
			if alt, src, ok := soleImage(b.Text); ok && o.image != nil {
				if rendered := o.image(alt, src, w); rendered != nil {
					media(rendered)
					gap()
					continue
				}
			}
			emit(screen.Span{}, inlineText(b.Text, screen.Span{}))
			gap()

		case MdFence:
			if strings.EqualFold(b.Lang, "mermaid") && o.Mermaid != nil {
				if rendered := o.Mermaid(strings.Join(b.Code, "\n"), w); rendered != nil {
					media(rendered)
					gap()
					continue
				}
			}
			codeBlock(b.Code, b.Lang)
			gap()

		case MdRule:
			rule()
			gap()

		case MdHeading:
			gap()
			fg := map[int]screen.Style{1: screen.StyleMdH1, 2: screen.StyleMdH2}[b.Level]
			if fg == "" {
				fg = screen.StyleMdH3
			}
			emit(screen.Span{}, inlineText(b.Text, screen.Span{Style: fg, Bold: true}))
			if b.Level <= 2 {
				rule()
			}
			gap()

		case MdQuote:
			bar := screen.Span{Text: "▎ ", Style: screen.StyleMdQuote}
			qo, base := o, len(out)
			qo.FrontMatter = false
			if o.Wrapped != nil {
				qo.Wrapped = func(row int, space bool, indent int) { o.Wrapped(base+row, space, indent+2) }
			}
			if o.Exact != nil {
				qo.Exact = func(row, from, to int) { o.Exact(base+row, from+2, to+2) }
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

		case MdTableBlock:
			out = append(out, mdTable(b.Rows, b.Aligns, w)...)
			gap()

		case MdList:
			emit(screen.Span{Text: b.Prefix, Style: screen.StyleMdListMark}, inlineText(b.Text, screen.Span{}))
			if b.GapAfter {
				gap()
			}
		}
	}
	for len(out) > 0 && gapAt == len(out)-1 {
		out = out[:len(out)-1]
		gapAt = -1
	}
	return MdDoc{Lines: out, Images: images}
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
	blank := MdRow(screen.Span{Text: strings.Repeat(" ", max(0, w)), Style: bg, Bg: bg})
	var out []screen.Line
	if lang != "" {
		out = append(out, MdRow(screen.Span{Text: Fit(" "+lang, w), Style: screen.StyleLineNo, Bg: bg}))
	} else {
		out = append(out, blank)
	}
	pieces = map[int]codePiece{}
	tw := w - 2
	for _, l := range code {
		if tw <= 0 {
			out = append(out, MdRow(edge, edge))
			continue
		}
		lines := mdGraphemes(LexSpans(l, lex)).wrap(tw, false)
		for i, line := range lines {
			pw := line.text.width()
			spans := line.text.spans()
			if pw > tw { // a single grapheme wider than the block
				spans = []screen.Span{PlainSpan("…")}
				pw = 1
			}
			if len(lines) > 1 {
				pieces[len(out)] = codePiece{cont: i > 0, width: pw}
			}
			for j := range spans {
				spans[j].Bg = bg
			}
			row := append([]screen.Span{edge}, spans...)
			if pw < tw {
				row = append(row, PlainSpan(strings.Repeat(" ", tw-pw)))
			}
			out = append(out, MdRow(append(row, edge)...))
		}
	}
	out = append(out, blank)
	return out, pieces
}

func mdTable(rows [][]string, aligns []string, w int) []screen.Line {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	cells := make([][]mdText, len(rows))
	widths := make([]int, cols)
	for ri, r := range rows {
		cells[ri] = make([]mdText, cols)
		for ci := 0; ci < cols; ci++ {
			txt := ""
			if ci < len(r) {
				txt = r[ci]
			}
			st := screen.Span{}
			if ri == 0 {
				st.Bold, st.Style = true, screen.StyleMdH3
			}
			cells[ri][ci] = inlineText(txt, st)
			widths[ci] = max(widths[ci], cells[ri][ci].width())
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
		return MdRow(fitSpans([]screen.Span{border(l + strings.Join(parts, m) + r)}, w)...)
	}
	var out []screen.Line
	out = append(out, rule("┌", "┬", "┐"))
	for ri := range rows {
		// wrap each cell to its column; the row is as tall as its tallest cell
		wrapped := make([][]mdWrappedLine, cols)
		height := 1
		for ci := 0; ci < cols; ci++ {
			wrapped[ci] = cells[ri][ci].wrap(widths[ci], true)
			height = max(height, len(wrapped[ci]))
		}
		for li := 0; li < height; li++ {
			spans := []screen.Span{border("│")}
			for ci := 0; ci < cols; ci++ {
				var cs mdText
				if li < len(wrapped[ci]) {
					cs = wrapped[ci][li].text
				}
				// A grapheme wider than the column is left blank.
				cs = cs.prefix(widths[ci])
				padN := widths[ci] - cs.width()
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
				spans = append(spans, PlainSpan(" "+strings.Repeat(" ", left)))
				spans = append(spans, cs.spans()...)
				spans = append(spans, PlainSpan(strings.Repeat(" ", padN-left)+" "), border("│"))
			}
			out = append(out, MdRow(fitSpans(spans, w)...))
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
