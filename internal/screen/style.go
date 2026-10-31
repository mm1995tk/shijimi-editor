package screen

import "slices"

// Style はテーマ上の色の名前（意味）。RGB は持たない。
// 名前と色の対応は Palette で別に渡す。
type Style string

const (
	// 構文
	StyleSyntaxText      Style = "syntax.text"
	StyleSyntaxComment   Style = "syntax.comment"
	StyleSyntaxPreproc   Style = "syntax.preproc"
	StyleSyntaxKeyword   Style = "syntax.keyword"
	StyleSyntaxNamespace Style = "syntax.namespace"
	StyleSyntaxType      Style = "syntax.type"
	StyleSyntaxName      Style = "syntax.name"
	StyleSyntaxFunction  Style = "syntax.function"
	StyleSyntaxConst     Style = "syntax.const"
	StyleSyntaxString    Style = "syntax.string"
	StyleSyntaxEscape    Style = "syntax.escape"
	StyleSyntaxNumber    Style = "syntax.number"
	StyleSyntaxControl   Style = "syntax.control"

	// 診断
	StyleDiagError Style = "diag.error"
	StyleDiagWarn  Style = "diag.warn"
	StyleDiagInfo  Style = "diag.info"
	StyleDiagHint  Style = "diag.hint" // info と同じ色

	// エディタ
	StyleEditorBg   Style = "editor.bg"
	StyleEditorFg   Style = "editor.fg"
	StyleLineNo     Style = "lineno"
	StyleLineNoCur  Style = "lineno.current"
	StyleCurLine    Style = "curline"
	StyleSelection  Style = "selection"
	StyleMatch      Style = "match"
	StyleIndent     Style = "indent.guide"
	StyleCursor     Style = "cursor"
	StyleCursorFg   Style = "cursor.fg"
	StyleBorder     Style = "border"
	StyleAccent     Style = "accent"
	StyleDirty      Style = "dirty"
	StyleError      Style = "error"
	StyleStrong     Style = "strong"
	StyleDim        Style = "dim"
	StyleHeading    Style = "heading"
	StyleSideBg     Style = "side.bg"
	StyleSideFg     Style = "side.fg"
	StyleSideTitle  Style = "side.title"
	StyleTabBarBg   Style = "tab.bg"
	StyleTabInact   Style = "tab.inactive"
	StyleTabInactFg Style = "tab.inactive.fg"

	// 一覧・パレット・状態行
	StylePaletteBg   Style = "palette.bg"
	StylePaletteHi   Style = "palette.hi"
	StyleListSel     Style = "list.selected"
	StyleListSelUnf  Style = "list.selected.unfocused"
	StyleListSubSel  Style = "list.selected.sub"
	StylePanelTitle  Style = "panel.title"
	StylePanelCmd    Style = "panel.cmd"
	StyleStatusFg    Style = "status.fg"
	StyleStatusErrBg Style = "status.error.bg"
	StyleModeNormal  Style = "mode.normal"
	StyleModeInsert  Style = "mode.insert"
	StyleModeSelect  Style = "mode.select"

	// git
	StyleGitAdded   Style = "git.added"
	StyleGitDeleted Style = "git.deleted"
	StyleGitRenamed Style = "git.renamed"

	// ホバー
	StyleHoverCodeBg Style = "hover.code.bg"
	StyleHoverInline Style = "hover.inline"
	StyleHoverHead   Style = "hover.head"
	StyleHoverRule   Style = "hover.rule"

	// 差分
	StyleDiffAddBg     Style = "diff.add.bg"
	StyleDiffDelBg     Style = "diff.del.bg"
	StyleDiffAddSign   Style = "diff.add.sign"
	StyleDiffDelSign   Style = "diff.del.sign"
	StyleDiffModSign   Style = "diff.mod.sign"
	StyleDiffAddWordBg Style = "diff.add.word.bg"
	StyleDiffDelWordBg Style = "diff.del.word.bg"
	StyleDiffHunkFg    Style = "diff.hunk.fg"
	StyleDiffHunkBg    Style = "diff.hunk.bg"
	StyleDiffNoteBg    Style = "diff.note.bg"
	StyleDiffBarView   Style = "diff.bar.view"
)

// Palette は Style → 色（"#rrggbb"）の対応。テーマ 1 つ分。
type Palette map[Style]string

// Styles は Palette が持つ名前を並べ替えて返す。
func (p Palette) Styles() []Style {
	out := make([]Style, 0, len(p))
	for s := range p {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}
