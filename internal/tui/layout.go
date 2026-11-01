package tui

// layout.go is where the screen is laid out in terminal cells: how wide each
// pane is and where it starts. It belongs to the TUI. The state that decides
// what is shown (core) never takes a layout; it asks the few questions below
// (editRows, textCols, ...), which answer in rows and columns of a region.
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
	l.bodyH = a.bodyH()
	if a.diffMode() {
		l.top = 1
	}
	if a.showSide {
		l.sideW = a.sideWidth()
		l.rightX = l.sideW + 1
	}
	l.rightW = max(0, a.w-l.rightX)
	if a.agentShown(l.sideW) {
		l.agentW = a.agentWidth()
		l.agentX = a.w - l.agentW
		l.rightW = max(0, l.agentX-1-l.rightX)
	}
	l.fullX, l.fullW = l.rightX, l.rightW
	if a.splitWanted() && l.fullW >= minSplitW {
		l.split = true
		first := (l.fullW - 1) / 2
		second := l.fullW - 1 - first
		if a.focusedFirst() {
			l.rightW, l.otherX, l.otherW = first, l.fullX+first+1, second
		} else {
			l.rightX, l.rightW, l.otherX, l.otherW = l.fullX+first+1, second, l.fullX, first
		}
	}
	l.editY = l.top + 1
	if a.findShown() {
		l.editY++
	}
	if a.showTerm {
		l.panelH = a.panelHeight()
	}
	l.editH = max(0, l.top+l.bodyH-l.editY-l.panelH)
	l.panelY = l.editY + l.editH
	return l
}

// editRows is the number of text rows the editor shows.
func (a *App) editRows() int { return a.layout().editH }

// textCols is the width of the pane of t, its line numbers included,
// from which a sideways scroll subtracts gutterWidth.
func (a *App) textCols(t *tab) int { return a.editW(t, a.layout()) }

// diffBaseCols is the width of the base side of a split diff.
func (a *App) diffBaseCols() int { return a.dvGeom(a.layout()).leftW }

// editorCols is the width of the editor area, which a split divides.
func (a *App) editorCols() int { return a.layout().fullW }

// suggestionsShown reports whether the suggestion panel is open, in which case
// the status line leaves the suggestion count to it.
func (a *App) suggestionsShown() bool { return a.layout().agentW > 0 }

// inEditorText reports whether the pointer is over the text of the current text
// tab, where the sideways wheel scrolls it.
func (a *App) inEditorText(l layout, msg Mouse) bool {
	t := a.cur()
	if t == nil || t.kind != tabText || a.fullPreview(t) || msg.Region != RegionEditor {
		return false
	}
	top := l.editY - l.top
	return msg.Row >= top && msg.Row < top+l.editH
}

// cursorShown reports whether the cursor drawn at (x, y) is inside the text
// of its pane: sideways scrolling can take it over the line numbers, the
// base side of a split diff or off the edge.
func (a *App) cursorShown(t *tab, l layout, x, y int) bool {
	left, w, top, h := l.rightX, a.editW(t, l), l.editY, l.editH
	if dv := a.diffFor(t); dv != nil {
		g := a.dvGeom(l)
		left, top, h = g.rightX, g.top, g.h
	}
	return x >= left+gutterWidth(t.buf) && x < left+w && x < a.w && y >= top && y < top+h
}
