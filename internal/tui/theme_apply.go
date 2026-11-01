package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
)

// applyTheme makes t the active theme: it rewrites the palette variables,
// rebuilds precomputed styles, and invalidates color caches.
func applyTheme(t *Theme) {
	curTheme = t
	themeGen++

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

	modeStyles = map[editMode]lipgloss.Style{
		modeNormal: bgfg(t.ModeNormal, t.CursorFg).Bold(true),
		modeInsert: bgfg(t.ModeInsert, t.CursorFg).Bold(true),
		modeSelect: bgfg(t.ModeSelect, t.CursorFg).Bold(true),
	}

	s := t.Syntax
	chromaStyle = chroma.MustNewStyle(t.Name, chroma.StyleEntries{
		chroma.Text:                s.Text,
		chroma.Comment:             s.Comment,
		chroma.CommentPreproc:      s.Preproc,
		chroma.Keyword:             s.Keyword,
		chroma.KeywordNamespace:    s.Namespace,
		chroma.KeywordType:         s.Type,
		chroma.Name:                s.Name,
		chroma.NameBuiltin:         s.Type,
		chroma.NameFunction:        s.Function,
		chroma.NameClass:           s.Type,
		chroma.NameConstant:        s.Const,
		chroma.NameTag:             s.Keyword,
		chroma.NameAttribute:       s.Name,
		chroma.NameDecorator:       s.Function,
		chroma.LiteralString:       s.String,
		chroma.LiteralStringEscape: s.Escape,
		chroma.LiteralNumber:       s.Number,
		chroma.Operator:            s.Text,
		chroma.Punctuation:         s.Text,
		chroma.GenericHeading:      "bold " + s.Keyword,
		chroma.GenericSubheading:   "bold " + s.Keyword,
		chroma.GenericInserted:     s.Number,
		chroma.GenericDeleted:      s.String,
		chroma.GenericEmph:         "italic",
		chroma.GenericStrong:       "bold",
	})

	cellStyles = map[cellStyle]lipgloss.Style{}
	tokenColors = map[chroma.TokenType]string{}
	rebuildStyles(t)
}
