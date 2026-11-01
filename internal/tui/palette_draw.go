package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// drawPalette draws a palette Overlay as a box w cells wide. in is the TUI's
// state of the input (what is in view of a long text); the Overlay holds the
// text and the caret.
func drawPalette(ov screen.Overlay, in *core.InputField, w int) []string {
	var out []string
	label := core.PaletteLabel(ov.Title, ov.Field.Prefix)
	out = append(out, stPaletteBox.Render(core.Fit("", w)))
	out = append(out, stPaletteBox.Render(label)+fitStyled(viewField(in, inputStyleOn(cPaletteBg)), w-ansi.StringWidth(label), stPalette))
	for i, it := range ov.Items {
		out = append(out, renderItem(it, w, i == ov.Active))
	}
	if ov.Hint != "" {
		out = append(out, stPaletteDim.Render(core.Fit(ov.Hint, w)))
	}
	out = append(out, stPalette.Render(core.Fit("", w)))
	return out
}

func renderItem(it screen.Item, w int, selected bool) string {
	base, hi := stPalette, stPaletteHi
	if selected {
		base, hi = stPaletteSel, stPaletteHiS
	}
	matched := map[int]bool{}
	for _, m := range it.Matched {
		matched[m] = true
	}
	var sb strings.Builder
	sb.WriteString(base.Render("  "))
	used := 2
	for i, r := range []rune(it.Label) {
		rw := ansi.StringWidth(string(r))
		if used+rw > w-1 {
			break
		}
		if matched[i] {
			sb.WriteString(hi.Render(string(r)))
		} else {
			sb.WriteString(base.Render(string(r)))
		}
		used += rw
	}
	if it.Detail != "" && it.Detail != "." {
		d := "  " + it.Detail
		dw := ansi.StringWidth(d)
		if used+dw < w {
			st := stPaletteDim
			if selected {
				st = bgfg(cListSel, core.CurTheme.SubSel)
			}
			sb.WriteString(st.Render(d))
			used += dw
		}
	}
	if used < w {
		sb.WriteString(base.Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}
