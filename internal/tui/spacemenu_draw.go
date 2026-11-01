package tui

import "github.com/charmbracelet/x/ansi"

// renderSpaceMenu draws the popup over the bottom-right corner of the editor
// area while a leader key is waiting for its follow-up.
func (a *App) renderSpaceMenu(rows []string, l layout) []string {
	return a.renderMenu(rows, l, " Space", a.spaceMenuItems())
}

// renderMenu draws a menu of leader key items titled title.
func (a *App) renderMenu(rows []string, l layout, title string, items []spaceItem) []string {
	box := drawMenu(menuOverlay(title, items), l.rightW)
	if box == nil {
		return rows
	}
	x := l.rightX + l.rightW - ansi.StringWidth(box[0])
	y := l.editY + l.editH - len(box)
	return overlay(rows, box, x, max(0, y))
}
