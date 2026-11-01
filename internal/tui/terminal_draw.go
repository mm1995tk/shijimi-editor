package tui

import (
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func (t *terminal) render(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelBord.Render(strings.Repeat("─", w)))

	var bar strings.Builder
	bar.WriteString(stPanelTitle.Render(" ") + stPanelTab.Render("TERMINAL"))
	if tab := t.sh; tab != nil {
		bar.WriteString(stPanelCmd.Render(" " + tab.label() + " "))
	}
	out = append(out, fitStyled(bar.String(), w, stPanel))

	body := h - 2
	if body < 0 {
		return out[:h]
	}
	tab := t.cur()
	if tab == nil || w < 1 || body < 1 {
		for len(out) < h {
			out = append(out, stPanel.Render(core.Fit("", w)))
		}
		return out[:h]
	}
	sn := &tab.snap // taken in fit
	for y := range body {
		cx := -1
		if focused && sn.CursorVisible && sn.CursorY == y {
			cx = sn.CursorX
		}
		s0, s1 := t.selCols(sn.Top + y)
		out = append(out, renderVTRowSel(sn.Cells[y], w, cx, s0, s1))
	}
	return out[:h]
}
