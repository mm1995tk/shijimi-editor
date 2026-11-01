package tui

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
