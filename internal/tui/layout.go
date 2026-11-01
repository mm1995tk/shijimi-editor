package tui

import "github.com/mm1995tk/shijimi-editor/internal/core"

// layout.go is where the screen is laid out in terminal cells: how wide each
// pane is and where it starts. It belongs to the TUI. The state that decides
// what is shown (core) never takes a layout; it asks the few questions below
// (EditRows, TextCols, ...), which answer in rows and columns of a region.
// Mouse positions arrive as a region plus a row and column in it (see Mouse),
// and only the TUI turns those into screen cells.

type layout struct {
	top, bodyH     int // rows between the diff mode header and the help/status lines
	sideW          int
	rightX, rightW int // the focused editor pane (the whole editor area unless split)
	fullX, fullW   int // the editor area, which the terminal under it also spans
	split          bool
	otherX, otherW int // the other editor pane while split
	agentX, agentW int // the suggestion panel; agentW is 0 when it is not shown
	editY, editH   int
	panelY, panelH int
	statusY        int
}

func (a *App) layout() layout {
	var l layout
	l.statusY = a.h - 1
	l.bodyH = a.BodyH()
	if a.DiffMode() {
		l.top = 1
	}
	if a.showSide {
		l.sideW = a.sideWidth()
		l.rightX = l.sideW + 1
	}
	l.rightW = max(0, a.w-l.rightX)
	if a.AgentShown(l.sideW) {
		l.agentW = a.AgentWidth()
		l.agentX = a.w - l.agentW
		l.rightW = max(0, l.agentX-1-l.rightX)
	}
	l.fullX, l.fullW = l.rightX, l.rightW
	if a.SplitWanted() && l.fullW >= core.MinSplitW {
		l.split = true
		first := (l.fullW - 1) / 2
		second := l.fullW - 1 - first
		if a.FocusedFirst() {
			l.rightW, l.otherX, l.otherW = first, l.fullX+first+1, second
		} else {
			l.rightX, l.rightW, l.otherX, l.otherW = l.fullX+first+1, second, l.fullX, first
		}
	}
	l.editY = l.top + 1
	if a.FindShown() {
		l.editY++
	}
	if a.showTerm {
		l.panelH = a.panelHeight()
	}
	l.editH = max(0, l.top+l.bodyH-l.editY-l.panelH)
	l.panelY = l.editY + l.editH
	return l
}

// EditRows is the number of text rows the editor shows.
func (a *App) EditRows() int { return a.layout().editH }

// TextCols is the width of the pane of t, its line numbers included,
// from which a sideways scroll subtracts GutterWidth.
func (a *App) TextCols(t *core.Tab) int { return a.editW(t, a.layout()) }

// DiffBaseCols is the width of the base side of a split diff.
func (a *App) DiffBaseCols() int { return a.dvGeom(a.layout()).LeftW }

// EditorCols is the width of the editor area, which a split divides.
func (a *App) EditorCols() int { return a.layout().fullW }

// SuggestionsShown reports whether the suggestion panel is open, in which case
// the status line leaves the suggestion count to it.
func (a *App) SuggestionsShown() bool { return a.layout().agentW > 0 }

// inEditorText reports whether the pointer is over the text of the current text
// tab, where the sideways wheel scrolls it.
func (a *App) inEditorText(l layout, msg core.Mouse) bool {
	t := a.Cur()
	if t == nil || t.Kind != core.TabText || a.FullPreview(t) || msg.Region != core.RegionEditor {
		return false
	}
	top := l.editY - l.top
	return msg.Row >= top && msg.Row < top+l.editH
}

// cursorShown reports whether the cursor drawn at (x, y) is inside the text
// of its pane: sideways scrolling can take it over the line numbers, the
// base side of a split diff or off the edge.
func (a *App) cursorShown(t *core.Tab, l layout, x, y int) bool {
	left, w, top, h := l.rightX, a.editW(t, l), l.editY, l.editH
	if dv := a.DiffOf(t); dv != nil {
		g := a.dvGeom(l)
		left, top, h = g.RightX, g.Top, g.H
	}
	return x >= left+core.GutterWidth(t.Buf) && x < left+w && x < a.w && y >= top && y < top+h
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
