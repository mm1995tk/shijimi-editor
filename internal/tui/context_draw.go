package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

func (c *ctxPreview) render(w, maxH int) []string {
	lines := strings.Split(strings.TrimRight(c.text, "\n"), "\n")
	body := max(1, maxH-3)
	out := []string{stPaletteBox.Render(fit(i18n.T("ctx.title"), w))}
	for _, l := range lines[c.scroll:min(len(lines), c.scroll+body)] {
		out = append(out, stPalette.Render(fit(" "+strings.ReplaceAll(l, "\t", "    "), w)))
	}
	if len(lines) > body {
		out = append(out, stPaletteDim.Render(fit(fmt.Sprintf(i18n.T("ctx.lines"), c.scroll+1, min(len(lines), c.scroll+body), len(lines)), w)))
	}
	return out
}

// drawContext puts the preview c over rows on a screen sw x sh.
func drawContext(rows []string, c *ctxPreview, sw, sh int) []string {
	w := clamp(sw*3/5, 30, 100)
	box := c.render(w, max(4, sh-4))
	if ansi.StringWidth(box[0]) > w {
		box[0] = ansi.Truncate(box[0], w, "")
	}
	return overlay(rows, box, (sw-w)/2, 1)
}
