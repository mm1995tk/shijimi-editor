package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// signaturePos puts the box on the row above the cursor, or below it when the
// cursor is on the top row of the editor.
func signaturePos(l layout, w, cx, cy int, box []string) (x, y int) {
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	x = core.Clamp(cx, l.rightX, max(l.rightX, w-bw))
	y = cy - len(box)
	if y < l.editY {
		y = cy + 1
	}
	return x, y
}

// drawSignature draws a signature Overlay as a box of cells, cut to maxW.
func drawSignature(ov screen.Overlay, maxW int) []string {
	var sb strings.Builder
	sb.WriteString(stPalette.Render(" "))
	for _, sp := range ov.Lines[0].Spans {
		st := stPalette
		if sp.Style == screen.StylePaletteHi {
			st = stPaletteHi
		}
		sb.WriteString(st.Render(sp.Text))
	}
	sb.WriteString(stPalette.Render(" "))
	line := sb.String()
	if ansi.StringWidth(line) > maxW {
		line = core.Fit(line, maxW)
	}
	return []string{line}
}
