package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// proposal is what the TUI draws of a core.Proposal: the one the user is asked
// about, and how far its diff is scrolled.
type proposal struct {
	*core.Proposal
	scroll int
}

// propView is the first waiting proposal as the TUI shows it; nil when none.
// A new first proposal starts at the top.
func (a *App) propView() *proposal {
	first := a.props.First()
	if first == nil {
		a.propShown = nil
		return nil
	}
	if a.propShown == nil || a.propShown.Proposal != first {
		a.propShown = &proposal{Proposal: first}
	}
	return a.propShown
}

// resolveProposal answers the agent and drops the proposal. The editor never
// writes the file: that is left to Claude Code after FILE_SAVED.
func (a *App) resolveProposal(p *core.Proposal, accept bool) {
	if accept {
		a.info(i18n.T("idep.accepted"), a.rel(p.NewPath))
		a.treeTicks = treeRefreshTicks - 2 // the client writes the file after this reply
	} else {
		a.info(i18n.T("idep.rejected"), a.rel(p.NewPath))
	}
	a.props.Resolve(p, accept)
}

// closeProposal drops a proposal the agent closed itself (close_tab,
// closeAllDiffTabs), typically after the decision was made on the Claude Code
// side. The pending request is still answered, but the editor rejected
// nothing, so the status bar must not say so.
func (a *App) closeProposal(p *core.Proposal) {
	a.info(i18n.T("idep.decided"), a.rel(p.NewPath))
	a.props.Close(p)
}

// cancelProposal drops the proposal whose client disconnected.
func (a *App) cancelProposal(reply chan<- IDEReply) { a.props.Cancel(reply) }

// handleProposalKey drives the first pending proposal: Enter accepts, Esc
// rejects.
func (a *App) handleProposalKey(msg Key) []Effect {
	p := a.propView()
	switch msg.String() {
	case "enter", "y":
		a.resolveProposal(p.Proposal, true)
	case "esc", "ctrl+c", "n", "q":
		a.resolveProposal(p.Proposal, false)
	case "ctrl+d", "pgdown", "down", "j":
		p.scroll++
	case "ctrl+u", "pgup", "up", "k":
		p.scroll = max(0, p.scroll-1)
	}
	return nil
}

// proposalH is the height the proposal box is given.
func (a *App) proposalH() int { return max(4, a.h-4) }

// fit keeps the scroll of the proposal inside its lines. It is the part of
// drawing the proposal that changes state, so it runs before drawing (App.fit).
func (p *proposal) fit(a *App) {
	p.scroll = clamp(p.scroll, 0, max(0, len(p.Lines)-max(1, a.proposalH()-3)))
}

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
