package core

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

// MinSplitW is the narrowest editor area that is split: two panes of at
// least 20 columns and the line between them.
const MinSplitW = 41

// SplitWanted reports whether the editor area is split now. Diff mode and
// single-file mode use the whole area for one file, and the split comes back
// when they end.
func (a *Core) SplitWanted() bool { return a.Split && !a.single && !a.Gd.On }

// tabIndex is the index of t in a.tabs, or -1.
func (a *Core) tabIndex(t *Tab) int {
	if t == nil {
		return -1
	}
	return slices.Index(a.Tabs, t)
}

// SplitOpen splits the editor area. The file stays in the left pane, and the
// right pane, which gets the focus, shows the last file (ga) or, with none,
// asks for a file.
func (a *Core) SplitOpen() []Effect {
	switch {
	case a.single:
		return nil
	case a.Split:
		a.InfoText(i18n.T("win.already"))
		return nil
	case a.Gd.On:
		a.InfoText(i18n.T("win.nodiff"))
		return nil
	case a.EditorCols() < MinSplitW:
		a.InfoText(i18n.T("win.narrow"))
		return nil
	}
	cur := a.Cur()
	if cur == nil {
		a.InfoText(i18n.T("win.nofile"))
		return nil
	}
	cur.Transient = false
	prev := a.PrevTab
	if a.tabIndex(prev) < 0 || prev == cur {
		prev = nil
	}
	a.Split, a.Pane = true, 1
	a.Other, a.otherPrev = cur, nil
	a.PrevTab = nil
	a.Active = a.tabIndex(prev)
	a.seenTab = a.Cur()
	a.Follow = true
	a.Mode = ModeNormal
	a.Comp = nil
	if prev == nil {
		return a.Pal.Show(a.Root, "")
	}
	return a.SetFocus(FocusEditor)
}

// FocusPane moves the focus to the pane on side (0 = left, 1 = right).
func (a *Core) FocusPane(side int) []Effect {
	if !a.SplitWanted() {
		if a.Split {
			a.InfoText(i18n.T("win.nodiff"))
		} else {
			a.InfoText(i18n.T("win.nosplit"))
		}
		return nil
	}
	if side != a.Pane {
		a.swapPanes()
		a.Pane = side
	}
	return a.SetFocus(FocusEditor)
}

// focusOtherPane moves the focus to the other pane.
func (a *Core) focusOtherPane() []Effect { return a.FocusPane(1 - a.Pane) }

// swapPanes makes the other pane's file the focused one's, and the reverse.
// The pane keeps its own "last file".
func (a *Core) swapPanes() {
	cur := a.Cur()
	a.Active = a.tabIndex(a.Other)
	a.Other = cur
	a.PrevTab, a.otherPrev = a.otherPrev, a.PrevTab
	a.seenTab = a.Cur()
	a.Follow = true
	a.Mode = ModeNormal
	a.Comp = nil
}

// closeSplit goes back to one pane. keepFocused picks which file stays in it;
// the other file stays open, only not shown.
func (a *Core) closeSplit(keepFocused bool) {
	if !a.Split {
		a.InfoText(i18n.T("win.nosplit"))
		return
	}
	if !keepFocused && a.Other != nil {
		a.swapPanes()
	}
	a.Split, a.Pane, a.Other, a.otherPrev = false, 0, nil, nil
	a.Follow = true
}

// showInPane shows tab i in the focused pane. A tab that the other pane shows
// is swapped over instead, so that the focus stays where it is.
func (a *Core) showInPane(i int) {
	if i < 0 || i >= len(a.Tabs) {
		return
	}
	if a.Split && a.Tabs[i] == a.Other {
		a.Other = a.Cur()
		a.otherPrev, a.PrevTab = a.PrevTab, a.otherPrev
	}
	a.Active = i
	a.Follow = true
	a.Mode = ModeNormal
	a.Comp = nil
}

// NormalizeSplit keeps the panes consistent after an event: a closed file
// leaves its pane empty, an empty split is dropped, and a file chosen that the
// other pane shows takes the focus to that pane.
func (a *Core) NormalizeSplit() {
	if !a.Split {
		a.Other, a.otherPrev = nil, nil
		return
	}
	if a.tabIndex(a.Other) < 0 {
		a.Other = nil
	}
	if cur := a.Cur(); cur != nil && cur == a.Other {
		a.Other = a.seenTab
		if a.tabIndex(a.Other) < 0 || a.Other == cur {
			a.Other = nil
		}
		a.Pane = 1 - a.Pane
		a.PrevTab, a.otherPrev = a.otherPrev, a.PrevTab
		a.seenTab = cur
	}
	if a.Cur() == nil && a.Other == nil {
		a.Split, a.Pane = false, 0
	}
	if a.Other != nil {
		a.Other.Transient = false
	}
}

// windowKey runs Space w <key> (Helix's window menu).
func (a *Core) windowKey(key string) []Effect {
	if a.single {
		return nil
	}
	switch key {
	case "v", "s":
		return a.SplitOpen()
	case "h", "left":
		return a.FocusPane(0)
	case "l", "right":
		return a.FocusPane(1)
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
	if a.Split {
		a.closeSplit(true)
		return nil
	}
	return a.SplitOpen()
}

// openFileHere is OpenFile for commands that act on the focused pane (Ctrl+O,
// Ctrl+I): a file the other pane shows is swapped into this one.
func (a *Core) openFileHere(path string) bool {
	abs, _ := filepath.Abs(path)
	if a.Split && a.Other != nil && a.Other.Path() == abs {
		a.showInPane(a.tabIndex(a.Other))
		return true
	}
	return a.OpenFile(path)
}

// WindowItems is the Space w menu.
var WindowItems = []SpaceItem{
	{"v", "win.item.split", false},
	{"h", "win.item.left", false},
	{"l", "win.item.right", false},
	{"w", "win.item.other", false},
	{"q", "win.item.close", false},
	{"o", "win.item.only", false},
}
