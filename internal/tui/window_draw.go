package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// joinPanes puts the two panes side by side with a line between them.
func joinPanes(left, right []string, leftW, rightW int) []string {
	n := max(len(left), len(right))
	out := make([]string, n)
	sep := stSeparator.Render("│")
	pad := func(s string, w int) string {
		if n := ansi.StringWidth(s); n < w {
			return s + stEmpty.Render(strings.Repeat(" ", w-n))
		}
		return s
	}
	for i := range out {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out[i] = pad(l, leftW) + sep + pad(r, rightW)
	}
	return out
}
