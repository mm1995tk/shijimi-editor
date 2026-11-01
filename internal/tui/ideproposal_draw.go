package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

func renderProposalBox(p *core.ProposalView, a *App, w, maxH int) []string {
	body := max(1, maxH-3)
	title := fmt.Sprintf(i18n.T("idep.title"), a.Rel(p.NewPath))
	if a.ProposalCount() > 1 {
		title += fmt.Sprintf(i18n.T("idep.waiting"), a.ProposalCount()-1)
	}
	out := []string{stPaletteBox.Render(core.Fit(title, w))}
	for _, l := range p.Lines[p.Scroll:min(len(p.Lines), p.Scroll+body)] {
		text := core.Fit(" "+string(l.Kind)+" "+strings.ReplaceAll(l.Text, "\t", "    "), w)
		if l.Kind == '@' {
			text = core.Fit(" "+l.Text, w)
		}
		switch l.Kind {
		case '+':
			out = append(out, lipgloss.NewStyle().Background(lipgloss.Color(cAddBg)).Render(text))
		case '-':
			out = append(out, lipgloss.NewStyle().Background(lipgloss.Color(cDelBg)).Render(text))
		case '@':
			out = append(out, stPaletteDim.Render(text))
		default:
			out = append(out, stPalette.Render(text))
		}
	}
	if len(p.Lines) > body {
		out = append(out, stPaletteDim.Render(core.Fit(fmt.Sprintf(i18n.T("ctx.lines"), p.Scroll+1, min(len(p.Lines), p.Scroll+body), len(p.Lines)), w)))
	}
	return out
}

func (a *App) renderProposal(rows []string) []string {
	w := core.Clamp(a.w*4/5, 30, 140)
	box := renderProposalBox(a.PropView(), a, w, a.ProposalH())
	if ansi.StringWidth(box[0]) > w {
		box[0] = ansi.Truncate(box[0], w, "")
	}
	return core.Overlay(rows, box, (a.w-w)/2, 1)
}
