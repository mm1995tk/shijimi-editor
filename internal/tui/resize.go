package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Pane resizing. The sizes the user picked are kept on the App, so they
// survive closing and reopening the pane; layout() clamps them on every draw.

const (
	resizeStep      = 2 // columns / rows per key press
	previewStepPct  = 5
	defaultPreviewP = 50
	minSideW        = 10
	minPaneW        = 10
)

// sideWidth is the sidebar width: the user's choice, or a fifth of the screen.
func (a *App) sideWidth() int {
	if a.sideWPref > 0 {
		return core.Clamp(a.sideWPref, minSideW, max(minSideW, min(a.w/2, a.w-30)))
	}
	return core.Clamp(a.w/5, 18, 36)
}

// panelHeight is the terminal panel height: the user's choice, or 2/5 of the screen.
func (a *App) panelHeight() int {
	h := (a.h - 1) * 2 / 5
	if a.panelHPref > 0 {
		h = a.panelHPref
	}
	return core.Clamp(h, 6, max(6, a.h-8))
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
	return core.Clamp(avail*a.previewPercent()/100, minPaneW, max(minPaneW, avail-minPaneW))
}

// diffLeftWidth is the base pane width of a split diff whose two panes and
// separator take w columns: the user's share, or half.
func (a *App) diffLeftWidth(w int) int {
	avail := w - 1
	if a.diffLeftPct <= 0 {
		return avail / 2
	}
	return core.Clamp(avail*a.diffLeftPct/100, minPaneW, max(minPaneW, avail-minPaneW))
}

// ResizeSide moves the sidebar's border by delta cells.
func (a *App) ResizeSide(delta int) {
	a.sideWPref = a.sideWidth() + delta
	a.sideWPref = a.sideWidth()
	a.FollowCursor()
}

// ResizeTerm moves the terminal panel's border by delta rows.
func (a *App) ResizeTerm(delta int) {
	a.panelHPref = a.panelHeight() + delta
	a.panelHPref = a.panelHeight()
	a.FollowCursor()
}

func (a *App) resizePreviewBy(deltaPct int) {
	a.previewPct = core.Clamp(a.previewPercent()+deltaPct, 10, 90)
	a.FollowCursor()
}

// ResetPanes puts the borders back where they start.
func (a *App) ResetPanes() {
	a.sideWPref, a.panelHPref, a.previewPct, a.agentWPref, a.diffLeftPct, a.explorerPct = 0, 0, 0, 0, 0, 0
	a.FollowCursor()
}

// StartResize starts moving the borders with the keys.
func (a *App) StartResize() []core.Effect {
	a.CloseCompletion()
	a.CloseHover()
	a.resizeMode = true
	a.InfoText(i18n.T("resize.hint"))
	return nil
}

// editorBorder is the border inside the editor area of t: the │ of a split
// diff or the one beside the Markdown preview.
func (a *App) editorBorder(t *core.Tab) core.ResizeTarget {
	switch {
	case t == nil:
	case a.DiffFile(t) != nil:
		if a.SplitShown() {
			return core.ResizeDiff
		}
	case t.Preview == core.PreviewSplit:
		return core.ResizePreview
	}
	return core.ResizeNone
}

// keyBorder is the border the arrow keys move in resize mode: the one beside
// the focused pane, a vertical border for ← → and a horizontal one for ↑ ↓.
func (a *App) keyBorder(l layout, across bool) core.ResizeTarget {
	if !across {
		if (a.FocusIs(core.FocusExplorer) || a.FocusIs(core.FocusChanges)) && a.showSide && a.DiffMode() {
			return core.ResizeExplorer
		}
		return core.ResizeTerm
	}
	switch {
	case a.FocusIs(core.FocusEditor):
		if b := a.editorBorder(a.CurText()); b != core.ResizeNone {
			return b
		}
		if l.agentW > 0 {
			return core.ResizeAgent
		}
	case a.FocusIs(core.FocusAgent):
		if l.agentW > 0 {
			return core.ResizeAgent
		}
	}
	return core.ResizeSide
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
func (a *App) moveBorder(l layout, target core.ResizeTarget, d int) {
	switch target {
	case core.ResizeSide:
		a.ResizeSide(d)
	case core.ResizeTerm:
		a.ResizeTerm(-d)
	case core.ResizeAgent:
		a.agentWPref = a.AgentWidth() - d
		a.agentWPref = a.AgentWidth()
		a.FollowCursor()
	case core.ResizeDiff:
		if avail := l.rightW - 2; avail > 0 {
			a.diffLeftPct = core.Clamp(pctToward(a.dvGeom(l).LeftW+d, avail, d), 1, 99)
			a.FollowCursor()
		}
	case core.ResizePreview:
		if avail := l.rightW - 1; avail > 0 {
			a.previewPct = core.Clamp(pctToward(a.editW(a.CurText(), l)+d, avail, d), 10, 90)
			a.FollowCursor()
		}
	case core.ResizeExplorer:
		if body := a.BodyH(); body > 0 {
			a.explorerPct = core.Clamp(pctToward(a.ExplorerH()+d, body, d), 1, 99)
		}
	}
}

// HandleResizeKey runs while resize mode is on; every key either resizes or
// leaves the mode.
func (a *App) HandleResizeKey(key string) []core.Effect {
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
		a.ResetPanes()
	case "esc", "enter", "q", "alt+r":
		a.resizeMode = false
		a.ClearMsg()
		return nil
	}
	a.InfoText(i18n.T("resize.hint"))
	return nil
}

// resizeHit returns the border under (x, y), if any.
func (a *App) resizeHit(l layout, x, y int) core.ResizeTarget {
	switch {
	case a.showSide && x == l.sideW && y >= l.top && y < l.top+l.bodyH:
		return core.ResizeSide
	case a.showTerm && y == l.panelY && x >= l.fullX:
		return core.ResizeTerm
	case a.showSide && a.DiffMode() && x < l.sideW && y == l.top+a.ExplorerH():
		return core.ResizeExplorer
	}
	t := a.CurText()
	switch a.editorBorder(t) {
	case core.ResizeDiff:
		if y > l.editY && y < l.editY+l.editH && x == l.rightX+a.dvGeom(l).LeftW {
			return core.ResizeDiff
		}
	case core.ResizePreview:
		if y >= l.editY && y < l.editY+l.editH && x == l.rightX+a.editW(t, l) {
			return core.ResizePreview
		}
	}
	return core.ResizeNone
}

// dragResize moves the border being dragged to the pointer.
func (a *App) dragResize(l layout, x, y int) {
	switch a.resizing {
	case core.ResizeSide:
		a.sideWPref = x
		a.sideWPref = a.sideWidth()
	case core.ResizeTerm:
		a.panelHPref = l.statusY - y
		a.panelHPref = a.panelHeight()
	case core.ResizeAgent:
		a.agentWPref = a.w - x - 1
		a.agentWPref = a.AgentWidth()
	case core.ResizeExplorer:
		if body := a.BodyH(); body > 0 {
			a.explorerPct = core.Clamp(((y-l.top)*100+body-1)/body, 1, 99)
		}
	case core.ResizeDiff:
		if avail := l.rightW - 2; avail > 0 {
			a.diffLeftPct = core.Clamp((x-l.rightX)*100/avail, 1, 99)
		}
	case core.ResizePreview:
		if avail := l.rightW - 1; avail > 0 {
			a.previewPct = core.Clamp((x-l.rightX)*100/avail, 10, 90)
		}
	}
	a.FollowCursor()
}
