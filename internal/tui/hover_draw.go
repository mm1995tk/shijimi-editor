package tui

import (
	"fmt"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// renderCells draws cells on bg, padded with spaces to w cells.
func renderCells(cs []core.HoverCell, bg string, w int) string {
	var sb strings.Builder
	width := 0
	for i := 0; i < len(cs); {
		j := i
		var run strings.Builder
		for j < len(cs) && cs[j].St == cs[i].St {
			run.WriteRune(cs[j].R)
			width += ansi.StringWidth(string(cs[j].R))
			j++
		}
		fg := core.StyleColor(cs[i].St)
		if fg == "" {
			fg = cHoverFg
		}
		sb.WriteString(bgfg(bg, fg).Bold(cs[i].St == screen.StyleHoverHead).Render(run.String()))
		i = j
	}
	if width < w {
		sb.WriteString(bgfg(bg, cHoverFg).Render(strings.Repeat(" ", w-width)))
	}
	return sb.String()
}

// drawHover draws a hover Overlay as a box of cells: border, padding, code
// bands and a scroll indicator on the bottom border.
func drawHover(ov screen.Overlay) []string {
	vw := screen.Viewport{Cols: 1}
	if ov.View != nil {
		vw = *ov.View
	}
	width := max(vw.Cols, 1)
	end := vw.Top + len(ov.Lines)
	bd := bgfg(cHoverBg, cHoverBorder)
	iw := width + 2
	out := make([]string, 0, len(ov.Lines)+2)
	top := strings.Repeat("─", iw)
	if n := ansi.StringWidth(ov.CopyLabel); n+2 <= iw {
		top = strings.Repeat("─", iw-n-1) + ov.CopyLabel + "─"
	}
	out = append(out, bd.Render("╭"+top+"╮"))
	for _, l := range ov.Lines {
		bg := cHoverBg
		if l.Band != "" {
			bg = core.StyleColor(l.Band)
		}
		pad := bgfg(bg, cHoverFg).Render(" ")
		var cells []core.HoverCell
		if l.Rule {
			cells = make([]core.HoverCell, width)
			for i := range cells {
				cells[i] = core.HoverCell{R: '─', St: screen.StyleHoverRule}
			}
		}
		for _, sp := range l.Spans {
			for _, r := range sp.Text {
				cells = append(cells, core.HoverCell{R: r, St: sp.Style})
			}
		}
		out = append(out, bd.Render("│")+pad+renderCells(cells, bg, width)+pad+bd.Render("│"))
	}
	ind := ""
	hint := ov.Hint
	if vw.Total > vw.Rows {
		ind = fmt.Sprintf(" %d/%d ", end, vw.Total)
		if vw.Top > 0 {
			ind = "↑" + ind
		}
		if end < vw.Total {
			ind += "↓"
		}
	}
	bottom := strings.Repeat("─", iw)
	switch hn, in := ansi.StringWidth(hint), ansi.StringWidth(ind); {
	case in > 0 && hn+in+3 <= iw:
		bottom = "─" + hint + strings.Repeat("─", iw-hn-in-2) + ind + "─"
	case in == 0 && hn+2 <= iw:
		bottom = "─" + hint + strings.Repeat("─", iw-hn-1)
	case in > 0 && in < iw:
		bottom = strings.Repeat("─", iw-in-1) + ind + "─"
	}
	out = append(out, bd.Render("╰"+bottom+"╯"))
	return out
}
