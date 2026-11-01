package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func bgfg(bg, fg string) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg))
}

var (
	stSide, stSideDir, stSideTitle, stSideActive, stSideSel, stSideSelFocus, stSeparator lipgloss.Style
	stTabBar, stTabActive, stTabInactive, stTabDirty                                     lipgloss.Style
	stLineNo, stLineNoCur, stEmpty, stWelcome, stWelcomeH, stFind, stFindLabel           lipgloss.Style
	stPanelTitle, stPanelTab, stPanel, stPanelDim, stPanelCmd, stPanelBord               lipgloss.Style
	stStatus, stStatusError                                                              lipgloss.Style
	stPalette, stPaletteSel, stPaletteDim, stPaletteHi, stPaletteHiS, stPaletteBox       lipgloss.Style
)

// rebuildStyles recreates every precomputed style from the current colors.
func rebuildStyles(t *core.Theme) {
	stSide = bgfg(cSideBg, cSideFg)
	stSideDir = bgfg(cSideBg, cSideFg).Bold(true)
	stSideTitle = bgfg(cSideBg, t.SideTitle).Bold(true)
	stSideActive = bgfg(cSideBg, t.Strong)
	stSideSel = bgfg(cListSelUnf, t.Strong)
	stSideSelFocus = bgfg(cListSel, t.Strong)
	stSeparator = bgfg(cSideBg, cBorder)

	stTabBar = bgfg(cTabBarBg, cTabInactFg)
	stTabActive = bgfg(cEditorBg, t.Strong)
	stTabInactive = bgfg(cTabInact, cTabInactFg)
	stTabDirty = bgfg(cEditorBg, cDirty)

	stLineNo = bgfg(cEditorBg, cLineNo)
	stLineNoCur = bgfg(cCurLine, cLineNoCur)
	stEmpty = bgfg(cEditorBg, cEditorFg)
	stWelcome = bgfg(cEditorBg, t.Dim)
	stWelcomeH = bgfg(cEditorBg, t.Heading).Bold(true)

	stFind = bgfg(cPaletteBg, cEditorFg)
	stFindLabel = bgfg(cPaletteBg, t.Heading)

	stPanelTitle = bgfg(cEditorBg, t.PanelTitle).Bold(true)
	stPanelTab = bgfg(cEditorBg, t.PanelTitle).Underline(true)
	stPanel = bgfg(cEditorBg, cEditorFg)
	stPanelDim = bgfg(cEditorBg, cLineNo)
	stPanelCmd = bgfg(cEditorBg, t.PanelCmd)
	stPanelBord = bgfg(cEditorBg, cBorder)

	stStatus = bgfg(cAccent, t.AccentFg)
	stStatusError = bgfg(t.StatusErrBg, t.AccentFg)

	stPalette = bgfg(cPaletteBg, cEditorFg)
	stPaletteSel = bgfg(cListSel, t.Strong)
	stPaletteDim = bgfg(cPaletteBg, cLineNo)
	stPaletteHi = bgfg(cPaletteBg, t.PaletteHi).Bold(true)
	stPaletteHiS = bgfg(cListSel, t.PaletteHi).Bold(true)
	stPaletteBox = bgfg(cPaletteBg, cAccent)
}
