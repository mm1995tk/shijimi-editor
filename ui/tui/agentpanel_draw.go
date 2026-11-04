package tui

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/i18n"
)

// drawAgent draws the panel as h lines of w cells.
func drawAgent(ag core.AgentPanel, w, h int, focused bool) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelTitle.Render(core.Fit(fmt.Sprintf(i18n.T("panel.suggestions"), ag.Count), w)))
	if ag.Count == 0 {
		out = append(out, stPanelDim.Render(core.Fit(i18n.T("panel.nosuggestions"), w)))
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
	// The agent starts only when a suggestion runs, so its status line stays
	// hidden until then; a log left by an agent that stopped keeps the rule.
	if ag.Started {
		status := i18n.T("panel.idle")
		if ag.Working {
			status = i18n.T("panel.working")
		}
		name := ag.Name
		if name == "" {
			name = i18n.T("panel.agent")
		}
		out = append(out, stPanelBord.Render(strings.Repeat("─", w)))
		out = append(out, stPanelTitle.Render(core.Fit(" "+name+": "+status, w)))
	} else if ag.HasLog {
		out = append(out, stPanelBord.Render(strings.Repeat("─", w)))
	}
	body := h - len(out) - core.BoolInt(ag.Hint != "")
	var card []string
	for _, lines := range ag.Perms {
		card = append(card, stPaletteHi.Render(core.Fit(i18n.T("panel.permtitle"), w)))
		for _, l := range lines {
			card = append(card, stPalette.Render(core.Fit("  "+l, w)))
		}
		card = append(card, stPaletteDim.Render(core.Fit(i18n.T("panel.permkeys"), w)))
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
