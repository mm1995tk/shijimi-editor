package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

func (p *proposal) render(a *App, w, maxH int) []string {
	body := max(1, maxH-3)
	title := fmt.Sprintf(i18n.T("idep.title"), a.rel(p.NewPath))
	if a.props.Len() > 1 {
		title += fmt.Sprintf(i18n.T("idep.waiting"), a.props.Len()-1)
	}
	out := []string{stPaletteBox.Render(fit(title, w))}
	for _, l := range p.Lines[p.scroll:min(len(p.Lines), p.scroll+body)] {
		text := fit(" "+string(l.Kind)+" "+strings.ReplaceAll(l.Text, "\t", "    "), w)
		if l.Kind == '@' {
			text = fit(" "+l.Text, w)
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
		out = append(out, stPaletteDim.Render(fit(fmt.Sprintf(i18n.T("ctx.lines"), p.scroll+1, min(len(p.Lines), p.scroll+body), len(p.Lines)), w)))
	}
	return out
}

func (a *App) renderProposal(rows []string) []string {
	w := clamp(a.w*4/5, 30, 140)
	box := a.propView().render(a, w, a.proposalH())
	if ansi.StringWidth(box[0]) > w {
		box[0] = ansi.Truncate(box[0], w, "")
	}
	return overlay(rows, box, (a.w-w)/2, 1)
}
