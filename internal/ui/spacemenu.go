package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// spaceItem is one row of the Space menu popup. textOnly marks commands that
// do nothing unless the current tab is a text tab.
type spaceItem struct {
	key, label string
	textOnly   bool
}

// spaceItems lists the keys spaceMode understands, in display order.
var spaceItems = []spaceItem{
	{"f", "space.openfile", false},
	{"b", "space.buffers", false},
	{"k", "space.hover", true},
	{"o", "space.openurl", false},
	{"r", "space.rename", true},
	{".", "lspui.codeactions", true},
	{"s", "space.symbols", true},
	{"S", "space.wsymbols", true},
	{"d", "space.diagnostics", false},
	{"e", "app.pane.tree", false},
	{"E", "app.pane.changes", false},
	{"m", "space.preview", false},
	{"g", "space.diffpr", false},
	{"G", "space.diffhead", false},
	{"v", "space.review", false},
	{"w", "space.window", false},
	{"y", "space.copy", true},
	{"p", "space.pasteafter", true},
	{"P", "space.pastebefore", true},
	{"/", "space.grep", false},
	{"c", "space.togglecomment", true},
	{"t", "app.pane.terminal", false},
	{"a", "space.sendagent", false},
	{"i", "app.pane.suggest", false},
	{"?", "space.commands", false},
}

// spaceMenuItems returns the items usable right now.
func (a *App) spaceMenuItems() []spaceItem {
	t := a.cur()
	text := t != nil && t.kind == tabText
	full := t != nil && a.fullPreview(t)
	var out []spaceItem
	for _, it := range spaceItems {
		if it.textOnly && !text {
			continue
		}
		if it.key == "v" && !a.prShown() {
			continue
		}
		if it.key == "E" && !a.gd.on {
			continue
		}
		if full && !strings.Contains("fbkodDeEmgGvwy?/tai", it.key) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// renderSpaceMenu draws the popup over the bottom-right corner of the editor
// area while a leader key is waiting for its follow-up.
func (a *App) renderSpaceMenu(rows []string, l layout) []string {
	return a.renderMenu(rows, l, " Space", a.spaceMenuItems())
}

// menuOverlay is a menu of leader key items titled title, as a screen.Overlay.
// Each Item has the translated label in Label and the key in Detail. It has no
// place in a pane (the drawer puts it in a corner). It changes nothing.
// Drawing it as cells is drawMenu's work.
func menuOverlay(title string, items []spaceItem) screen.Overlay {
	ov := screen.Overlay{Kind: screen.OverlayMenu, Title: title}
	for _, it := range items {
		ov.Items = append(ov.Items, screen.Item{Label: i18n.T(it.label), Detail: it.key})
	}
	return ov
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
	box := []string{stPaletteBox.Render(fit(ov.Title, w))}
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
