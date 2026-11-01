package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// renderContext is the box of the preview c, w cells wide, showing body rows
// of its text (Core.FitContext keeps c.Scroll within them).
func renderContext(c *core.CtxPreview, w, body int) []string {
	lines := strings.Split(strings.TrimRight(c.Text, "\n"), "\n")
	out := []string{stPaletteBox.Render(core.Fit(i18n.T("ctx.title"), w))}
	for _, l := range lines[c.Scroll:min(len(lines), c.Scroll+body)] {
		out = append(out, stPalette.Render(core.Fit(" "+strings.ReplaceAll(l, "\t", "    "), w)))
	}
	if len(lines) > body {
		out = append(out, stPaletteDim.Render(core.Fit(fmt.Sprintf(i18n.T("ctx.lines"), c.Scroll+1, min(len(lines), c.Scroll+body), len(lines)), w)))
	}
	return out
}

// drawContext puts the preview c over rows on a screen sw x sh.
func drawContext(rows []string, c *core.CtxPreview, sw, sh int) []string {
	w := core.Clamp(sw*3/5, 30, 100)
	box := renderContext(c, w, core.ContextBodyRows(sh))
	if ansi.StringWidth(box[0]) > w {
		box[0] = ansi.Truncate(box[0], w, "")
	}
	return core.Overlay(rows, box, (sw-w)/2, 1)
}
