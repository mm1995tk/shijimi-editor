package core

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// hoverCell is one rune with the name of its color (a heading's is bold).
type HoverCell struct {
	R  rune
	St screen.Style
}

// hoverLine is one logical line of the popup. code lines get a darker band.
type hoverLine struct {
	cells []HoverCell
	code  bool
	rule  bool // horizontal rule, drawn at the popup width
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// textCells styles one markdown text line: headings, `inline code`, links.
func textCells(l string) [][]HoverCell {
	head := false
	if t := strings.TrimLeft(l, "#"); len(t) < len(l) && strings.HasPrefix(t, " ") {
		l, head = strings.TrimSpace(t), true
	}
	l = mdLink.ReplaceAllString(l, "$1")
	var cs []HoverCell
	inCode := false
	for _, r := range l {
		switch {
		case r == '`':
			inCode = !inCode
		case inCode:
			cs = append(cs, HoverCell{R: r, St: screen.StyleHoverInline})
		case head:
			cs = append(cs, HoverCell{R: r, St: screen.StyleHoverHead})
		default:
			cs = append(cs, HoverCell{R: r})
		}
	}
	return [][]HoverCell{cs}
}

// codeCells syntax-highlights a fenced block, one cell slice per line.
func codeCells(lang string, lines []string) [][]HoverCell {
	out := make([][]HoverCell, len(lines))
	for i, l := range lines {
		for _, r := range l {
			out[i] = append(out[i], HoverCell{R: r})
		}
	}
	var lx chroma.Lexer
	if lang != "" {
		lx = lexers.Get(lang)
	}
	if lx == nil {
		return out
	}
	it, err := chroma.Coalesce(lx).Tokenise(nil, strings.Join(lines, "\n"))
	if err != nil {
		return out
	}
	row, col := 0, 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		var c screen.Style
		if e := ChromaStyle.Get(tok.Type); e.Colour.IsSet() {
			c = syntaxStyleOf(e.Colour.String())
		}
		for _, r := range tok.Value {
			if r == '\n' {
				row++
				col = 0
				continue
			}
			if row < len(out) && col < len(out[row]) {
				out[row][col].St = c
			}
			col++
		}
	}
	return out
}

// parseHover converts hover markdown into logical lines: one per text line and
// one per line of a fenced block, not yet wrapped. Wrapping needs the width of
// the popup, which is the TUI's (wrapHover).
func parseHover(text string) []hoverLine {
	var out []hoverLine
	var fence []string
	fenceLang := ""
	inFence := false
	flush := func() {
		cells := codeCells(fenceLang, fence)
		for _, cs := range cells {
			out = append(out, hoverLine{cells: cs, code: true})
		}
		if len(cells) == 0 {
			out = append(out, hoverLine{code: true})
		}
	}
	for l := range strings.SplitSeq(strings.TrimSpace(strings.ReplaceAll(text, "\r", "")), "\n") {
		l = strings.ReplaceAll(l, "\t", "    ")
		trim := strings.TrimSpace(l)
		if after, ok := strings.CutPrefix(trim, "```"); ok {
			if inFence {
				flush()
				fence = nil
			} else {
				fenceLang = strings.TrimSpace(after)
			}
			inFence = !inFence
			continue
		}
		if inFence {
			fence = append(fence, l)
			continue
		}
		switch {
		case trim == "---" || trim == "***" || trim == "___":
			out = append(out, hoverLine{rule: true})
		default:
			out = append(out, hoverLine{cells: textCells(l)[0]})
		}
	}
	if inFence { // unterminated fence
		flush()
	}
	return out
}
