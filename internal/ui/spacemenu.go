package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// spaceItem is one row of the Space menu popup. textOnly marks commands that
// do nothing unless the current tab is a text tab.
type spaceItem struct {
	key, label string
	textOnly   bool
}

// spaceItems lists the keys spaceMode understands, in display order.
var spaceItems = []spaceItem{
	{"f", "ファイルを開く", false},
	{"b", "バッファ一覧", false},
	{"k", "ホバー", true},
	{"o", "URL を開く", false},
	{"r", "名前の変更", true},
	{".", "コードアクション", true},
	{"d", "診断の一覧", false},
	{"e", "ファイルツリー", false},
	{"m", "プレビュー切替", false},
	{"g", "PR 一覧", false},
	{"G", "変更一覧", false},
	{"y", "コピー", true},
	{"p", "貼り付け(後)", true},
	{"P", "貼り付け(前)", true},
	{"/", "全文検索", false},
	{"c", "コメント切替", true},
	{"t", "ターミナル", false},
	{"a", "エージェントへ送る", false},
	{"?", "コマンド一覧", false},
}

// spaceMenuItems returns the items usable right now.
func (a *App) spaceMenuItems() []spaceItem {
	t := a.cur()
	text := t != nil && t.kind == tabText
	full := t != nil && a.fullPreview(t)
	var out []spaceItem
	for _, it := range spaceItems {
		if it.textOnly && !text {
			continue
		}
		if full && !strings.Contains("fbkodDeEmgGy?/ta", it.key) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// renderSpaceMenu draws the popup over the bottom-right corner of the editor
// area while a leader key is waiting for its follow-up.
func (a *App) renderSpaceMenu(rows []string, l layout) []string {
	items := a.spaceMenuItems()
	if len(items) == 0 {
		return rows
	}
	cols := 2
	if len(items) < 6 || l.rightW < 60 {
		cols = 1
	}
	per := (len(items) + cols - 1) / cols
	cellW := 0
	for _, it := range items {
		cellW = max(cellW, 2+ansi.StringWidth(it.label))
	}
	w := min(cols*cellW+(cols-1)*2+2, l.rightW)
	if w < 8 {
		return rows
	}
	inner := w - 2
	box := []string{stPaletteBox.Render(fit(" Space", w))}
	for r := 0; r < per; r++ {
		var sb strings.Builder
		sb.WriteString(stPalette.Render(" "))
		used := 1
		for c := 0; c < cols; c++ {
			i := c*per + r
			if i >= len(items) {
				break
			}
			if c > 0 {
				sb.WriteString(stPalette.Render("  "))
				used += 2
			}
			it := items[i]
			sb.WriteString(stPaletteHi.Render(it.key))
			sb.WriteString(stPalette.Render(" " + it.label))
			used += 2 + ansi.StringWidth(it.label)
			if pad := cellW - 2 - ansi.StringWidth(it.label); pad > 0 && c < cols-1 {
				sb.WriteString(stPalette.Render(strings.Repeat(" ", pad)))
				used += pad
			}
		}
		line := sb.String()
		if used > inner+1 {
			line = ansi.Truncate(line, inner+1, "")
			used = inner + 1
		}
		box = append(box, line+stPalette.Render(strings.Repeat(" ", max(0, w-used))))
	}
	x := l.rightX + l.rightW - w
	y := l.editY + l.editH - len(box)
	return overlay(rows, box, x, max(0, y))
}
