package core

import "github.com/mm1995tk/shijimi-editor/internal/screen"

// Palette は、screen.Style（意味の名前）→ このテーマの色の対応を返す。
// 設計書 docs/design/gui-protocol.md の `theme.get` が送る表の元。
// Control と IndentGuide は空のとき、applyTheme と同じ代わりの色を使う。
func (t *Theme) Palette() screen.Palette {
	s := t.Syntax
	control := s.Control
	if control == "" {
		control = s.Keyword
	}
	guide := t.IndentGuide
	if guide == "" {
		guide = t.LineNo
	}
	return screen.Palette{
		screen.StyleSyntaxText: s.Text, screen.StyleSyntaxComment: s.Comment,
		screen.StyleSyntaxPreproc: s.Preproc, screen.StyleSyntaxKeyword: s.Keyword,
		screen.StyleSyntaxNamespace: s.Namespace, screen.StyleSyntaxType: s.Type,
		screen.StyleSyntaxName: s.Name, screen.StyleSyntaxFunction: s.Function,
		screen.StyleSyntaxConst: s.Const, screen.StyleSyntaxString: s.String,
		screen.StyleSyntaxEscape: s.Escape, screen.StyleSyntaxNumber: s.Number,
		screen.StyleSyntaxControl: control,

		screen.StyleDiagError: t.DiagError, screen.StyleDiagWarn: t.DiagWarn, screen.StyleDiagInfo: t.DiagInfo,
		screen.StyleDiagHint: t.DiagInfo,

		screen.StyleEditorBg: t.EditorBg, screen.StyleEditorFg: t.EditorFg,
		screen.StyleLineNo: t.LineNo, screen.StyleLineNoCur: t.LineNoCur,
		screen.StyleCurLine: t.CurLine, screen.StyleSelection: t.Selection, screen.StyleMatch: t.Match,
		screen.StyleIndent: guide, screen.StyleCursor: t.Cursor, screen.StyleCursorFg: t.CursorFg,
		screen.StyleBorder: t.Border, screen.StyleAccent: t.Accent, screen.StyleDirty: t.Dirty,
		screen.StyleError: t.Error, screen.StyleStrong: t.Strong, screen.StyleDim: t.Dim,
		screen.StyleHeading: t.Heading,
		screen.StyleSideBg:  t.SideBg, screen.StyleSideFg: t.SideFg, screen.StyleSideTitle: t.SideTitle,
		screen.StyleTabBarBg: t.TabBarBg, screen.StyleTabInact: t.TabInact, screen.StyleTabInactFg: t.TabInactFg,

		screen.StylePaletteBg: t.PaletteBg, screen.StylePaletteHi: t.PaletteHi,
		screen.StyleListSel: t.ListSel, screen.StyleListSelUnf: t.ListSelUnf, screen.StyleListSubSel: t.SubSel,
		screen.StylePanelTitle: t.PanelTitle, screen.StylePanelCmd: t.PanelCmd,
		screen.StyleStatusFg: t.AccentFg, screen.StyleStatusErrBg: t.StatusErrBg,
		screen.StyleModeNormal: t.ModeNormal, screen.StyleModeInsert: t.ModeInsert, screen.StyleModeSelect: t.ModeSelect,

		screen.StyleGitAdded: t.GitAdded, screen.StyleGitDeleted: t.GitDeleted, screen.StyleGitRenamed: t.Renamed,

		screen.StyleHoverCodeBg: t.HoverCodeBg, screen.StyleHoverInline: t.HoverInline,
		screen.StyleHoverHead: t.HoverHead, screen.StyleHoverRule: t.HoverRule,

		// Markdown プレビューの色はテーマに依らず決まっている（今までと同じ色）
		screen.StyleMdH1: "#4fc1ff", screen.StyleMdH2: "#569cd6", screen.StyleMdH3: "#9cdcfe",
		screen.StyleMdCode: "#ce9178", screen.StyleMdCodeBg: "#2d2d2d", screen.StyleMdLink: "#4fc1ff",
		screen.StyleMdImage: "#6e6e6e", screen.StyleMdQuote: "#6a9955", screen.StyleMdListMark: "#9cdcfe",

		screen.StyleDiffAddBg: t.AddBg, screen.StyleDiffDelBg: t.DelBg,
		screen.StyleDiffAddSign: t.AddSign, screen.StyleDiffDelSign: t.DelSign, screen.StyleDiffModSign: t.ModSign,
		screen.StyleDiffAddWordBg: t.AddWordBg, screen.StyleDiffDelWordBg: t.DelWordBg,
		screen.StyleDiffHunkFg: t.HunkFg, screen.StyleDiffHunkBg: t.HunkBg, screen.StyleDiffNoteBg: t.NoteBg,
		screen.StyleDiffBarView: t.BarView,
	}
}
