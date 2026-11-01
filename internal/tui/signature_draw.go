package tui

import "github.com/charmbracelet/x/ansi"

// signaturePos puts the box on the row above the cursor, or below it when the
// cursor is on the top row of the editor.
func signaturePos(l layout, w, cx, cy int, box []string) (x, y int) {
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	x = clamp(cx, l.rightX, max(l.rightX, w-bw))
	y = cy - len(box)
	if y < l.editY {
		y = cy + 1
	}
	return x, y
}
