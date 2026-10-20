package ui

import tea "charm.land/bubbletea/v2"

// Pane resizing. The sizes the user picked are kept on the App, so they
// survive closing and reopening the pane; layout() clamps them on every draw.

type resizeTarget int

const (
	resizeNone resizeTarget = iota
	resizeSide
	resizeTerm
	resizePreview
)

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

func (a *App) resizeSide(delta int) {
	a.sideWPref = a.sideWidth() + delta
	a.sideWPref = a.sideWidth()
	a.follow = true
}

func (a *App) resizeTerm(delta int) {
	a.panelHPref = a.panelHeight() + delta
	a.panelHPref = a.panelHeight()
	a.follow = true
}

func (a *App) resizePreviewBy(deltaPct int) {
	a.previewPct = clamp(a.previewPercent()+deltaPct, 10, 90)
	a.follow = true
}

func (a *App) resetPanes() {
	a.sideWPref, a.panelHPref, a.previewPct = 0, 0, 0
	a.follow = true
}

func (a *App) startResize() tea.Cmd {
	a.comp = nil
	a.hoverText = ""
	a.resizeMode = true
	a.info("リサイズ: ←→ サイドバー / ↑↓ ターミナル / < > プレビュー / = 初期値 / Esc で終了")
	return nil
}

// handleResizeKey runs while resize mode is on; every key either resizes or
// leaves the mode.
func (a *App) handleResizeKey(key string) tea.Cmd {
	switch key {
	case "left", "h":
		a.resizeSide(-resizeStep)
	case "right", "l":
		a.resizeSide(resizeStep)
	case "up", "k":
		a.resizeTerm(resizeStep)
	case "down", "j":
		a.resizeTerm(-resizeStep)
	case "<", ",":
		a.resizePreviewBy(-previewStepPct)
	case ">", ".":
		a.resizePreviewBy(previewStepPct)
	case "=":
		a.resetPanes()
	case "esc", "enter", "q", "alt+r":
		a.resizeMode = false
		a.msg = ""
		return nil
	}
	a.info("リサイズ: ←→ サイドバー / ↑↓ ターミナル / < > プレビュー / = 初期値 / Esc で終了")
	return nil
}

// resizeHit returns the border under (x, y), if any.
func (a *App) resizeHit(l layout, x, y int) resizeTarget {
	switch {
	case a.showSide && x == l.sideW && y >= l.top && y < l.top+l.bodyH:
		return resizeSide
	case a.showTerm && y == l.panelY && x >= l.rightX:
		return resizeTerm
	}
	if t := a.curText(); t != nil && t.preview == previewSplit && y >= l.editY && y < l.editY+l.editH && x == l.rightX+a.editW(t, l) {
		return resizePreview
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
	case resizePreview:
		if avail := l.rightW - 1; avail > 0 {
			a.previewPct = clamp((x-l.rightX)*100/avail, 10, 90)
		}
	}
	a.follow = true
}
