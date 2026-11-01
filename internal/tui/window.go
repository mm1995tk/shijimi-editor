package tui

import (
	"path/filepath"
	"slices"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// The editor area can be split into two panes side by side (never more, and
// never one above the other). The focused pane shows a.cur(), which is what
// every command acts on; the other pane shows a.other (nil = empty). Both
// panes draw their own title row. The same file is never in both panes: opening
// the file of the other pane moves the focus there instead.

// minSplitW is the narrowest editor area that is split: two panes of at
// least 20 columns and the line between them.
const minSplitW = 41

// splitWanted reports whether the editor area is split now. Diff mode and
// single-file mode use the whole area for one file, and the split comes back
// when they end.
func (a *Core) splitWanted() bool { return a.split && !a.single && !a.gd.on }

// tabIndex is the index of t in a.tabs, or -1.
func (a *Core) tabIndex(t *tab) int {
	if t == nil {
		return -1
	}
	return slices.Index(a.tabs, t)
}

// splitOpen splits the editor area. The file stays in the left pane, and the
// right pane, which gets the focus, shows the last file (ga) or, with none,
// asks for a file.
func (a *Core) splitOpen() []Effect {
	switch {
	case a.single:
		return nil
	case a.split:
		a.infoText(i18n.T("win.already"))
		return nil
	case a.gd.on:
		a.infoText(i18n.T("win.nodiff"))
		return nil
	case a.editorCols() < minSplitW:
		a.infoText(i18n.T("win.narrow"))
		return nil
	}
	cur := a.cur()
	if cur == nil {
		a.infoText(i18n.T("win.nofile"))
		return nil
	}
	cur.transient = false
	prev := a.prevTab
	if a.tabIndex(prev) < 0 || prev == cur {
		prev = nil
	}
	a.split, a.pane = true, 1
	a.other, a.otherPrev = cur, nil
	a.prevTab = nil
	a.active = a.tabIndex(prev)
	a.seenTab = a.cur()
	a.follow = true
	a.mode = modeNormal
	a.comp = nil
	if prev == nil {
		return a.pal.show(a.root, "")
	}
	return a.setFocus(focusEditor)
}

// focusPane moves the focus to the pane on side (0 = left, 1 = right).
func (a *Core) focusPane(side int) []Effect {
	if !a.splitWanted() {
		if a.split {
			a.infoText(i18n.T("win.nodiff"))
		} else {
			a.infoText(i18n.T("win.nosplit"))
		}
		return nil
	}
	if side != a.pane {
		a.swapPanes()
		a.pane = side
	}
	return a.setFocus(focusEditor)
}

// focusOtherPane moves the focus to the other pane.
func (a *Core) focusOtherPane() []Effect { return a.focusPane(1 - a.pane) }

// swapPanes makes the other pane's file the focused one's, and the reverse.
// The pane keeps its own "last file".
func (a *Core) swapPanes() {
	cur := a.cur()
	a.active = a.tabIndex(a.other)
	a.other = cur
	a.prevTab, a.otherPrev = a.otherPrev, a.prevTab
	a.seenTab = a.cur()
	a.follow = true
	a.mode = modeNormal
	a.comp = nil
}

// closeSplit goes back to one pane. keepFocused picks which file stays in it;
// the other file stays open, only not shown.
func (a *Core) closeSplit(keepFocused bool) {
	if !a.split {
		a.infoText(i18n.T("win.nosplit"))
		return
	}
	if !keepFocused && a.other != nil {
		a.swapPanes()
	}
	a.split, a.pane, a.other, a.otherPrev = false, 0, nil, nil
	a.follow = true
}

// showInPane shows tab i in the focused pane. A tab that the other pane shows
// is swapped over instead, so that the focus stays where it is.
func (a *Core) showInPane(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	if a.split && a.tabs[i] == a.other {
		a.other = a.cur()
		a.otherPrev, a.prevTab = a.prevTab, a.otherPrev
	}
	a.active = i
	a.follow = true
	a.mode = modeNormal
	a.comp = nil
}

// normalizeSplit keeps the panes consistent after an event: a closed file
// leaves its pane empty, an empty split is dropped, and a file chosen that the
// other pane shows takes the focus to that pane.
func (a *Core) normalizeSplit() {
	if !a.split {
		a.other, a.otherPrev = nil, nil
		return
	}
	if a.tabIndex(a.other) < 0 {
		a.other = nil
	}
	if cur := a.cur(); cur != nil && cur == a.other {
		a.other = a.seenTab
		if a.tabIndex(a.other) < 0 || a.other == cur {
			a.other = nil
		}
		a.pane = 1 - a.pane
		a.prevTab, a.otherPrev = a.otherPrev, a.prevTab
		a.seenTab = cur
	}
	if a.cur() == nil && a.other == nil {
		a.split, a.pane = false, 0
	}
	if a.other != nil {
		a.other.transient = false
	}
}

// windowKey runs Space w <key> (Helix's window menu).
func (a *Core) windowKey(key string) []Effect {
	if a.single {
		return nil
	}
	switch key {
	case "v", "s":
		return a.splitOpen()
	case "h", "left":
		return a.focusPane(0)
	case "l", "right":
		return a.focusPane(1)
	case "w":
		return a.focusOtherPane()
	case "q":
		a.closeSplit(false)
	case "o":
		a.closeSplit(true)
	}
	return nil
}

// toggleEditorSplit is Ctrl+\: split, or go back to one pane.
func (a *Core) toggleEditorSplit() []Effect {
	if a.split {
		a.closeSplit(true)
		return nil
	}
	return a.splitOpen()
}

// otherLayout is l for the pane that is not focused.
func (l layout) otherLayout() layout {
	l.rightX, l.rightW = l.otherX, l.otherW
	return l
}

// inOtherPane reports whether the cell (x, y) is in the unfocused pane,
// including its title row.
func (l layout) inOtherPane(x, y int) bool {
	return l.split && x >= l.otherX && x < l.otherX+l.otherW && y >= l.top && y < l.panelY
}

// openFileHere is openFile for commands that act on the focused pane (Ctrl+O,
// Ctrl+I): a file the other pane shows is swapped into this one.
func (a *Core) openFileHere(path string) bool {
	abs, _ := filepath.Abs(path)
	if a.split && a.other != nil && a.other.path() == abs {
		a.showInPane(a.tabIndex(a.other))
		return true
	}
	return a.openFile(path)
}

// windowItems is the Space w menu.
var windowItems = []spaceItem{
	{"v", "win.item.split", false},
	{"h", "win.item.left", false},
	{"l", "win.item.right", false},
	{"w", "win.item.other", false},
	{"q", "win.item.close", false},
	{"o", "win.item.only", false},
}
