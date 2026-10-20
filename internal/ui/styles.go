package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Palette colors. They are variables so applyTheme can swap them at runtime;
// see theme.go.
var (
	cEditorBg, cEditorFg, cSideBg, cSideFg, cTabBarBg, cTabInact, cTabInactFg string
	cAccent, cLineNo, cLineNoCur, cCurLine, cSelection, cMatch, cBorder       string
	cPaletteBg, cListSel, cListSelUnf, cDirty, cError                         string
	cIndentGuide                                                              string
)

// Diff colors; set by applyTheme.
var (
	cAddBg, cDelBg, cAddSign, cDelSign, cHunkFg, cHunkBg, cNoteBg string
	cAddWordBg, cDelWordBg, cModSign, cBarView                    string
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
func rebuildStyles(t *Theme) {
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

// fit truncates or pads s (which may contain ANSI codes) to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw > w {
		s = ansi.Truncate(s, w, "…")
		sw = ansi.StringWidth(s)
	}
	if sw < w {
		s += strings.Repeat(" ", w-sw)
	}
	return s
}

// overlay draws box lines over base lines starting at (x, y).
func overlay(base []string, box []string, x, y int) []string {
	for i, bl := range box {
		r := y + i
		if r < 0 || r >= len(base) {
			continue
		}
		line := base[r]
		bw := ansi.StringWidth(bl)
		left := ansi.Truncate(line, x, "")
		right := ansi.TruncateLeft(line, x+bw, "")
		base[r] = left + "\x1b[0m" + bl + "\x1b[0m" + right
	}
	return base
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
