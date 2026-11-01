package core

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// cells.go is the helpers that fit text to a number of terminal cells.

// fit truncates or pads s (which may contain ANSI codes) to exactly w cells.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		s = ansi.Truncate(s, w, "…")
		sw = ansi.StringWidth(s)
	}
	if sw < w {
		s += strings.Repeat(" ", w-sw)
	}
	return s
}

// overlay draws box lines over base lines starting at (x, y).
func Overlay(base []string, box []string, x, y int) []string {
	for i, bl := range box {
		r := y + i
		if r < 0 || r >= len(base) {
			continue
		}
		line := base[r]
		bw := ansi.StringWidth(bl)
		left := ansi.Truncate(line, x, "")
		right := ansi.TruncateLeft(line, x+bw, "")
		base[r] = left + "\x1b[0m" + bl + "\x1b[0m" + right
	}
	return base
}

func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
