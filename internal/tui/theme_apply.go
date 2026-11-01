package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// applyTheme makes t the active theme: it rewrites the palette variables,
// rebuilds precomputed styles, and invalidates color caches.
func applyTheme(t *core.Theme) {
	core.ActivateTheme(t)

	cEditorBg, cEditorFg, cSideBg, cSideFg = t.EditorBg, t.EditorFg, t.SideBg, t.SideFg
	cTabBarBg, cTabInact, cTabInactFg = t.TabBarBg, t.TabInact, t.TabInactFg
	cAccent, cLineNo, cLineNoCur, cCurLine = t.Accent, t.LineNo, t.LineNoCur, t.CurLine
	cSelection, cMatch, cBorder = t.Selection, t.Match, t.Border
	if cIndentGuide = t.IndentGuide; cIndentGuide == "" {
		cIndentGuide = t.LineNo
	}
	cPaletteBg, cListSel, cListSelUnf = t.PaletteBg, t.ListSel, t.ListSelUnf
	cDirty, cError = t.Dirty, t.Error

	cAddBg, cDelBg, cAddSign, cDelSign = t.AddBg, t.DelBg, t.AddSign, t.DelSign
	cHunkFg, cHunkBg, cNoteBg = t.HunkFg, t.HunkBg, t.NoteBg
	cAddWordBg, cDelWordBg, cModSign, cBarView = t.AddWordBg, t.DelWordBg, t.ModSign, t.BarView

	cHoverBg, cHoverFg, cHoverBorder = t.PaletteBg, t.EditorFg, t.Accent
	cHoverRule = t.HoverRule

	modeStyles = map[core.EditMode]lipgloss.Style{
		core.ModeNormal: bgfg(t.ModeNormal, t.CursorFg).Bold(true),
		core.ModeInsert: bgfg(t.ModeInsert, t.CursorFg).Bold(true),
		core.ModeSelect: bgfg(t.ModeSelect, t.CursorFg).Bold(true),
	}

	cellStyles = map[cellStyle]lipgloss.Style{}
	rebuildStyles(t)
}

// init makes the default theme the active one before any config is read.
func init() { applyTheme(core.DefaultTheme) }
