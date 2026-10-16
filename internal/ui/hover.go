package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
)

// Hover popup colors. Kept separate so a theme can override them later.
const (
	cHoverBg     = cPaletteBg
	cHoverFg     = cEditorFg
	cHoverCodeBg = "#151515"
	cHoverBorder = cAccent
	cHoverInline = "#ce9178"
	cHoverHead   = "#569cd6"
	cHoverRule   = "#5a5a5a"
)

// hoverCell is one rune with its foreground color and weight.
type hoverCell struct {
	r    rune
	fg   string
	bold bool
}

// hoverLine is one logical line of the popup. code lines get a darker band.
type hoverLine struct {
	cells []hoverCell
	code  bool
}

var mdLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// hoverLines converts hover markdown into wrapped lines at most w cells wide.
func hoverLines(text string, w int) []hoverLine {
	w = max(w, 1)
	var out []hoverLine
	var fence []string
	fenceLang := ""
	inFence := false
	for _, l := range strings.Split(strings.TrimSpace(strings.ReplaceAll(text, "\r", "")), "\n") {
		l = strings.ReplaceAll(l, "\t", "    ")
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "```") {
			if inFence {
				out = append(out, wrapCells(codeCells(fenceLang, fence), w, true)...)
				fence = nil
			} else {
				fenceLang = strings.TrimSpace(strings.TrimPrefix(trim, "```"))
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
			cs := make([]hoverCell, w)
			for i := range cs {
				cs[i] = hoverCell{r: '─', fg: cHoverRule}
			}
			out = append(out, hoverLine{cells: cs})
		default:
			out = append(out, wrapCells(textCells(l), w, false)...)
		}
	}
	if inFence { // unterminated fence
		out = append(out, wrapCells(codeCells(fenceLang, fence), w, true)...)
	}
	return out
}

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
			cs = append(cs, hoverCell{r: r, fg: cHoverInline})
		case head:
			cs = append(cs, hoverCell{r: r, fg: cHoverHead, bold: true})
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
		c := ""
		if e := chromaStyle.Get(tok.Type); e.Colour.IsSet() {
			c = e.Colour.String()
		}
		for _, r := range tok.Value {
			if r == '\n' {
				row++
				col = 0
				continue
			}
			if row < len(out) && col < len(out[row]) {
				out[row][col].fg = c
			}
			col++
		}
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
			out = append(out, hoverLine{cells: cs[:cut], code: code})
			cs = cs[next:]
		}
	}
	if len(lines) == 0 {
		out = append(out, hoverLine{code: code})
	}
	return out
}

// renderCells draws cells on bg, padded with spaces to w cells.
func renderCells(cs []hoverCell, bg string, w int) string {
	var sb strings.Builder
	width := 0
	for i := 0; i < len(cs); {
		j := i
		var run strings.Builder
		for j < len(cs) && cs[j].fg == cs[i].fg && cs[j].bold == cs[i].bold {
			run.WriteRune(cs[j].r)
			width += ansi.StringWidth(string(cs[j].r))
			j++
		}
		fg := cs[i].fg
		if fg == "" {
			fg = cHoverFg
		}
		sb.WriteString(bgfg(bg, fg).Bold(cs[i].bold).Render(run.String()))
		i = j
	}
	if width < w {
		sb.WriteString(bgfg(bg, cHoverFg).Render(strings.Repeat(" ", w-width)))
	}
	return sb.String()
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

// render draws the box scrolled by scroll rows: border, padding, code bands
// and a scroll indicator on the bottom border.
func (v hoverView) render(scroll int) []string {
	scroll = clamp(scroll, 0, v.maxScroll())
	end := min(scroll+v.height, len(v.lines))
	vis := v.lines[scroll:end]
	bd := bgfg(cHoverBg, cHoverBorder)
	iw := v.width + 2
	out := make([]string, 0, len(vis)+2)
	out = append(out, bd.Render("╭"+strings.Repeat("─", iw)+"╮"))
	for _, l := range vis {
		bg := cHoverBg
		if l.code {
			bg = cHoverCodeBg
		}
		pad := bgfg(bg, cHoverFg).Render(" ")
		out = append(out, bd.Render("│")+pad+renderCells(l.cells, bg, v.width)+pad+bd.Render("│"))
	}
	bottom := strings.Repeat("─", iw)
	if v.maxScroll() > 0 {
		ind := fmt.Sprintf(" %d/%d ", end, len(v.lines))
		if scroll > 0 {
			ind = "↑" + ind
		}
		if end < len(v.lines) {
			ind += "↓"
		}
		if n := ansi.StringWidth(ind); n < iw {
			bottom = strings.Repeat("─", iw-n-1) + ind + "─"
		}
	}
	out = append(out, bd.Render("╰"+bottom+"╯"))
	return out
}
