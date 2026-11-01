package tui

import (
	"strings"

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
func (a *Core) spaceMenuItems() []spaceItem {
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
