package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Colors follow VS Code's "Dark+" theme.
const (
	cEditorBg   = "#1e1e1e"
	cEditorFg   = "#d4d4d4"
	cSideBg     = "#252526"
	cSideFg     = "#cccccc"
	cTabBarBg   = "#252526"
	cTabInact   = "#2d2d2d"
	cTabInactFg = "#969696"
	cAccent     = "#007acc"
	cLineNo     = "#858585"
	cLineNoCur  = "#c6c6c6"
	cCurLine    = "#2a2d2e"
	cSelection  = "#264f78"
	cMatch      = "#613214"
	cBorder     = "#3c3c3c"
	cPaletteBg  = "#252526"
	cListSel    = "#04395e"
	cListSelUnf = "#37373d"
	cDirty      = "#e2c08d"
	cError      = "#f48771"
)

func bgfg(bg, fg string) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(fg))
}

var (
	stSide         = bgfg(cSideBg, cSideFg)
	stSideDir      = bgfg(cSideBg, cSideFg).Bold(true)
	stSideTitle    = bgfg(cSideBg, "#bbbbbb").Bold(true)
	stSideActive   = bgfg(cSideBg, "#ffffff")
	stSideSel      = bgfg(cListSelUnf, "#ffffff")
	stSideSelFocus = bgfg(cListSel, "#ffffff")
	stSeparator    = bgfg(cSideBg, cBorder)

	stTabBar      = bgfg(cTabBarBg, cTabInactFg)
	stTabActive   = bgfg(cEditorBg, "#ffffff")
	stTabInactive = bgfg(cTabInact, cTabInactFg)
	stTabDirty    = bgfg(cEditorBg, cDirty)

	stLineNo    = bgfg(cEditorBg, cLineNo)
	stLineNoCur = bgfg(cCurLine, cLineNoCur)
	stEmpty     = bgfg(cEditorBg, cEditorFg)
	stWelcome   = bgfg(cEditorBg, "#6e6e6e")
	stWelcomeH  = bgfg(cEditorBg, "#9cdcfe").Bold(true)

	stFind      = bgfg(cPaletteBg, cEditorFg)
	stFindLabel = bgfg(cPaletteBg, "#9cdcfe")

	stPanelTitle = bgfg(cEditorBg, "#e7e7e7").Bold(true)
	stPanelTab   = bgfg(cEditorBg, "#e7e7e7").Underline(true)
	stPanel      = bgfg(cEditorBg, cEditorFg)
	stPanelDim   = bgfg(cEditorBg, cLineNo)
	stPanelCmd   = bgfg(cEditorBg, "#4ec9b0")
	stPanelBord  = bgfg(cEditorBg, cBorder)

	stStatus      = bgfg(cAccent, "#ffffff")
	stStatusError = bgfg("#c72e0f", "#ffffff")

	stPalette    = bgfg(cPaletteBg, cEditorFg)
	stPaletteSel = bgfg(cListSel, "#ffffff")
	stPaletteDim = bgfg(cPaletteBg, cLineNo)
	stPaletteHi  = bgfg(cPaletteBg, "#2aaaff").Bold(true)
	stPaletteHiS = bgfg(cListSel, "#2aaaff").Bold(true)
	stPaletteBox = bgfg(cPaletteBg, cAccent)
)

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
