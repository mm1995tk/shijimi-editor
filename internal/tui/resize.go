package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Pane resizing. The sizes the user picked are kept on the App, so they
// survive closing and reopening the pane; layout() clamps them on every draw.

type resizeTarget int

const (
	resizeNone resizeTarget = iota
	resizeSide
	resizeTerm
	resizePreview
	resizeAgent
	resizeDiff
	resizeExplorer
)

const (
	resizeStep      = 2 // columns / rows per key press
	previewStepPct  = 5
	defaultPreviewP = 50
	minSideW        = 10
	minPaneW        = 10
	minSideSectionH = 3 // header + two rows
)

// sideWidth is the sidebar width: the user's choice, or a fifth of the screen.
func (a *App) sideWidth() int {
	if a.sideWPref > 0 {
		return clamp(a.sideWPref, minSideW, max(minSideW, min(a.w/2, a.w-30)))
	}
	return clamp(a.w/5, 18, 36)
}

// panelHeight is the terminal panel height: the user's choice, or 2/5 of the screen.
func (a *App) panelHeight() int {
	h := (a.h - 1) * 2 / 5
	if a.panelHPref > 0 {
		h = a.panelHPref
	}
	return clamp(h, 6, max(6, a.h-8))
}

// previewPercent is the share of the width the editor gets beside the preview.
func (a *App) previewPercent() int {
	if a.previewPct > 0 {
		return a.previewPct
	}
	return defaultPreviewP
}

// editorWidth splits the width left of the Markdown preview.
func (a *App) editorWidth(total int) int {
	avail := total - 1
	return clamp(avail*a.previewPercent()/100, minPaneW, max(minPaneW, avail-minPaneW))
}

// diffLeftWidth is the base pane width of a split diff whose two panes and
// separator take w columns: the user's share, or half.
func (a *App) diffLeftWidth(w int) int {
	avail := w - 1
	if a.diffLeftPct <= 0 {
		return avail / 2
	}
	return clamp(avail*a.diffLeftPct/100, minPaneW, max(minPaneW, avail-minPaneW))
}

func (a *App) resizeSide(delta int) {
	a.sideWPref = a.sideWidth() + delta
	a.sideWPref = a.sideWidth()
	a.followCursor()
}

func (a *App) resizeTerm(delta int) {
	a.panelHPref = a.panelHeight() + delta
	a.panelHPref = a.panelHeight()
	a.followCursor()
}

func (a *App) resizePreviewBy(deltaPct int) {
	a.previewPct = clamp(a.previewPercent()+deltaPct, 10, 90)
	a.followCursor()
}

func (a *App) resetPanes() {
	a.sideWPref, a.panelHPref, a.previewPct, a.agentWPref, a.diffLeftPct, a.explorerPct = 0, 0, 0, 0, 0, 0
	a.followCursor()
}

func (a *App) startResize() []Effect {
	a.closeCompletion()
	a.closeHover()
	a.resizeMode = true
	a.infoText(i18n.T("resize.hint"))
	return nil
}

// editorBorder is the border inside the editor area of t: the │ of a split
// diff or the one beside the Markdown preview.
func (a *App) editorBorder(t *tab) resizeTarget {
	switch {
	case t == nil:
	case a.diffFile(t) != nil:
		if a.splitShown() {
			return resizeDiff
		}
	case t.preview == previewSplit:
		return resizePreview
	}
	return resizeNone
}

// keyBorder is the border the arrow keys move in resize mode: the one beside
// the focused pane, a vertical border for ← → and a horizontal one for ↑ ↓.
func (a *App) keyBorder(l layout, across bool) resizeTarget {
	if !across {
		if (a.focusIs(focusExplorer) || a.focusIs(focusChanges)) && a.showSide && a.diffMode() {
			return resizeExplorer
		}
		return resizeTerm
	}
	switch {
	case a.focusIs(focusEditor):
		if b := a.editorBorder(a.curText()); b != resizeNone {
			return b
		}
		if l.agentW > 0 {
			return resizeAgent
		}
	case a.focusIs(focusAgent):
		if l.agentW > 0 {
			return resizeAgent
		}
	}
	return resizeSide
}

// pctToward is the share that gives n cells of avail, or failing that the
// nearest width past n in the direction d, so a key press always moves a
// border (one percent may be more than a cell).
func pctToward(n, avail, d int) int {
	if d > 0 {
		return (n*100 + avail - 1) / avail
	}
	return ((n+1)*100 - 1) / avail
}

// moveBorder moves a border by d columns (or rows): right or down when d > 0.
func (a *App) moveBorder(l layout, target resizeTarget, d int) {
	switch target {
	case resizeSide:
		a.resizeSide(d)
	case resizeTerm:
		a.resizeTerm(-d)
	case resizeAgent:
		a.agentWPref = a.agentWidth() - d
		a.agentWPref = a.agentWidth()
		a.followCursor()
	case resizeDiff:
		if avail := l.rightW - 2; avail > 0 {
			a.diffLeftPct = clamp(pctToward(a.dvGeom(l).leftW+d, avail, d), 1, 99)
			a.followCursor()
		}
	case resizePreview:
		if avail := l.rightW - 1; avail > 0 {
			a.previewPct = clamp(pctToward(a.editW(a.curText(), l)+d, avail, d), 10, 90)
			a.followCursor()
		}
	case resizeExplorer:
		if body := a.bodyH(); body > 0 {
			a.explorerPct = clamp(pctToward(a.explorerH()+d, body, d), 1, 99)
		}
	}
}

// handleResizeKey runs while resize mode is on; every key either resizes or
// leaves the mode.
func (a *App) handleResizeKey(key string) []Effect {
	l := a.layout()
	switch key {
	case "left", "h":
		a.moveBorder(l, a.keyBorder(l, true), -resizeStep)
	case "right", "l":
		a.moveBorder(l, a.keyBorder(l, true), resizeStep)
	case "up", "k":
		a.moveBorder(l, a.keyBorder(l, false), -resizeStep)
	case "down", "j":
		a.moveBorder(l, a.keyBorder(l, false), resizeStep)
	case "<", ",":
		a.resizePreviewBy(-previewStepPct)
	case ">", ".":
		a.resizePreviewBy(previewStepPct)
	case "=":
		a.resetPanes()
	case "esc", "enter", "q", "alt+r":
		a.resizeMode = false
		a.clearMsg()
		return nil
	}
	a.infoText(i18n.T("resize.hint"))
	return nil
}

// resizeHit returns the border under (x, y), if any.
func (a *App) resizeHit(l layout, x, y int) resizeTarget {
	switch {
	case a.showSide && x == l.sideW && y >= l.top && y < l.top+l.bodyH:
		return resizeSide
	case a.showTerm && y == l.panelY && x >= l.fullX:
		return resizeTerm
	case a.showSide && a.diffMode() && x < l.sideW && y == l.top+a.explorerH():
		return resizeExplorer
	}
	t := a.curText()
	switch a.editorBorder(t) {
	case resizeDiff:
		if y > l.editY && y < l.editY+l.editH && x == l.rightX+a.dvGeom(l).leftW {
			return resizeDiff
		}
	case resizePreview:
		if y >= l.editY && y < l.editY+l.editH && x == l.rightX+a.editW(t, l) {
			return resizePreview
		}
	}
	return resizeNone
}

// dragResize moves the border being dragged to the pointer.
func (a *App) dragResize(l layout, x, y int) {
	switch a.resizing {
	case resizeSide:
		a.sideWPref = x
		a.sideWPref = a.sideWidth()
	case resizeTerm:
		a.panelHPref = l.statusY - y
		a.panelHPref = a.panelHeight()
	case resizeAgent:
		a.agentWPref = a.w - x - 1
		a.agentWPref = a.agentWidth()
	case resizeExplorer:
		if body := a.bodyH(); body > 0 {
			a.explorerPct = clamp(((y-l.top)*100+body-1)/body, 1, 99)
		}
	case resizeDiff:
		if avail := l.rightW - 2; avail > 0 {
			a.diffLeftPct = clamp((x-l.rightX)*100/avail, 1, 99)
		}
	case resizePreview:
		if avail := l.rightW - 1; avail > 0 {
			a.previewPct = clamp((x-l.rightX)*100/avail, 10, 90)
		}
	}
	a.followCursor()
}
