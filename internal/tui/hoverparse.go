package tui

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// hoverCell is one rune with the name of its color (a heading's is bold).
type hoverCell struct {
	r  rune
	st screen.Style
}

// hoverLine is one logical line of the popup. code lines get a darker band.
type hoverLine struct {
	cells []hoverCell
	code  bool
	rule  bool // horizontal rule, drawn at the popup width
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// textCells styles one markdown text line: headings, `inline code`, links.
func textCells(l string) [][]hoverCell {
	head := false
	if t := strings.TrimLeft(l, "#"); len(t) < len(l) && strings.HasPrefix(t, " ") {
		l, head = strings.TrimSpace(t), true
	}
	l = mdLink.ReplaceAllString(l, "$1")
	var cs []hoverCell
	inCode := false
	for _, r := range l {
		switch {
		case r == '`':
			inCode = !inCode
		case inCode:
			cs = append(cs, hoverCell{r: r, st: screen.StyleHoverInline})
		case head:
			cs = append(cs, hoverCell{r: r, st: screen.StyleHoverHead})
		default:
			cs = append(cs, hoverCell{r: r})
		}
	}
	return [][]hoverCell{cs}
}

// codeCells syntax-highlights a fenced block, one cell slice per line.
func codeCells(lang string, lines []string) [][]hoverCell {
	out := make([][]hoverCell, len(lines))
	for i, l := range lines {
		for _, r := range l {
			out[i] = append(out[i], hoverCell{r: r})
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
		if e := chromaStyle.Get(tok.Type); e.Colour.IsSet() {
			c = syntaxStyleOf(e.Colour.String())
		}
		for _, r := range tok.Value {
			if r == '\n' {
				row++
				col = 0
				continue
			}
			if row < len(out) && col < len(out[row]) {
				out[row][col].st = c
			}
			col++
		}
	}
	return out
}
