package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// renderSpaceMenu draws the popup over the bottom-right corner of the editor
// area while a leader key is waiting for its follow-up.
func (a *App) renderSpaceMenu(rows []string, l layout) []string {
	return a.renderMenu(rows, l, " Space", a.SpaceMenuItems())
}

// renderMenu draws a menu of leader key items titled title.
func (a *App) renderMenu(rows []string, l layout, title string, items []core.SpaceItem) []string {
	box := drawMenu(core.MenuOverlay(title, items), l.rightW)
	if box == nil {
		return rows
	}
	x := l.rightX + l.rightW - ansi.StringWidth(box[0])
	y := l.editY + l.editH - len(box)
	return core.Overlay(rows, box, x, max(0, y))
}

// drawMenu draws a menu Overlay as a box of cells no wider than maxW, or nil
// when there is nothing to draw.
func drawMenu(ov screen.Overlay, maxW int) []string {
	items := ov.Items
	if len(items) == 0 {
		return nil
	}
	cols := 2
	if len(items) < 6 || maxW < 60 {
		cols = 1
	}
	per := (len(items) + cols - 1) / cols
	cellW := 0
	for _, it := range items {
		cellW = max(cellW, 2+ansi.StringWidth(it.Label))
	}
	w := min(cols*cellW+(cols-1)*2+2, maxW)
	if w < 8 {
		return nil
	}
	inner := w - 2
	box := []string{stPaletteBox.Render(core.Fit(ov.Title, w))}
	for r := range per {
		var sb strings.Builder
		sb.WriteString(stPalette.Render(" "))
		used := 1
		for c := 0; c < cols; c++ {
			i := c*per + r
			if i >= len(items) {
				break
			}
			if c > 0 {
				sb.WriteString(stPalette.Render("  "))
				used += 2
			}
			it := items[i]
			sb.WriteString(stPaletteHi.Render(it.Detail))
			sb.WriteString(stPalette.Render(" " + it.Label))
			used += 2 + ansi.StringWidth(it.Label)
			if pad := cellW - 2 - ansi.StringWidth(it.Label); pad > 0 && c < cols-1 {
				sb.WriteString(stPalette.Render(strings.Repeat(" ", pad)))
				used += pad
			}
		}
		line := sb.String()
		if used > inner+1 {
			line = ansi.Truncate(line, inner+1, "")
			used = inner + 1
		}
		box = append(box, line+stPalette.Render(strings.Repeat(" ", max(0, w-used))))
	}
	return box
}
