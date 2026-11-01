package tui

import (
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// drawAgent draws the panel as h lines of w cells.
func drawAgent(ag screen.Agent, w, h int, focused bool) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelTitle.Render(core.Fit(ag.Title, w)))
	if ag.Empty != "" {
		out = append(out, stPanelDim.Render(core.Fit(ag.Empty, w)))
	}
	for _, s := range ag.Items {
		st := stPanel
		mark := "   "
		if s.Selected {
			mark = " ▸ "
			st = stSideSel
			if focused {
				st = stSideSelFocus
			}
		}
		out = append(out, st.Render(core.Fit(mark+s.Title, w)))
		out = append(out, stPanelDim.Render(core.Fit("     "+s.Detail, w)))
	}
	if ag.Rule {
		out = append(out, stPanelBord.Render(strings.Repeat("─", w)))
	}
	if ag.Status != "" {
		out = append(out, stPanelTitle.Render(core.Fit(ag.Status, w)))
	}
	body := h - len(out) - core.BoolInt(ag.Hint != "")
	var card []string
	for _, p := range ag.Perms {
		card = append(card, stPaletteHi.Render(core.Fit(p.Title, w)))
		for _, l := range p.Lines {
			card = append(card, stPalette.Render(core.Fit("  "+l, w)))
		}
		card = append(card, stPaletteDim.Render(core.Fit(p.Keys, w)))
	}
	if len(card) > body {
		card = card[:max(0, body)]
	}
	out = append(out, card...)
	for _, r := range ag.Log {
		row := drawMdLine(r.Line, w)
		if r.SelFrom < r.SelTo {
			row = core.MarkRow(row, w, r.SelFrom, r.SelTo)
		}
		out = append(out, row)
	}
	for len(out) < h-core.BoolInt(ag.Hint != "") {
		out = append(out, stPanel.Render(strings.Repeat(" ", w)))
	}
	if ag.Hint != "" {
		out = append(out, stPanelDim.Render(core.Fit(" "+ag.Hint, w)))
	}
	return out[:h]
}
