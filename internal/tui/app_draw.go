package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// renderTitle is the line above the editor: the path of the file being
// edited (● when it is unsaved). Open files are listed with "Space b".
func (a *App) renderTitle(t *tab, w int, focused bool) string {
	return drawTitle(a.titleOf(t), w, focused)
}

// findInputCells is the width of the find bar between its label and hint.
func findInputCells(w int) int {
	return w - lipgloss.Width(stFindLabel.Render(findLabel())) - lipgloss.Width(stFind.Render(i18n.T("app.findhint")))
}

func (a *App) renderFind(w int) string {
	return drawFind(*a.findState(), a.findInput.view(inputStyleOn(cPaletteBg)), w)
}

var modeStyles map[editMode]lipgloss.Style

func (a *App) renderStatus() string { return drawStatus(a.statusOf(), a.confirmOverlay(), a.w) }
