package core

import (
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// ProposalView is what the TUI draws of a Proposal: the one the user is asked
// about, and how far its diff is scrolled.
type ProposalView struct {
	*Proposal
	Scroll int
}

// PropView is the first waiting proposal as the TUI shows it; nil when none.
// A new first proposal starts at the top.
func (a *Core) PropView() *ProposalView {
	first := a.props.First()
	if first == nil {
		a.propShown = nil
		return nil
	}
	if a.propShown == nil || a.propShown.Proposal != first {
		a.propShown = &ProposalView{Proposal: first}
	}
	return a.propShown
}

// resolveProposal answers the agent and drops the proposal. The editor never
// writes the file: that is left to Claude Code after FILE_SAVED.
func (a *Core) resolveProposal(p *Proposal, accept bool) {
	if accept {
		a.Info(i18n.T("idep.accepted"), a.Rel(p.NewPath))
		a.treeTicks = treeRefreshTicks - 2 // the client writes the file after this reply
	} else {
		a.Info(i18n.T("idep.rejected"), a.Rel(p.NewPath))
	}
	a.props.Resolve(p, accept)
}

// closeProposal drops a proposal the agent closed itself (close_tab,
// closeAllDiffTabs), typically after the decision was made on the Claude Code
// side. The pending request is still answered, but the editor rejected
// nothing, so the status bar must not say so.
func (a *Core) closeProposal(p *Proposal) {
	a.Info(i18n.T("idep.decided"), a.Rel(p.NewPath))
	a.props.Close(p)
}

// cancelProposal drops the proposal whose client disconnected.
func (a *Core) cancelProposal(reply chan<- IDEReply) { a.props.Cancel(reply) }

// handleProposalKey drives the first pending proposal: Enter accepts, Esc
// rejects.
func (a *Core) handleProposalKey(msg Key) []Effect {
	p := a.PropView()
	switch msg.String() {
	case "enter", "y":
		a.resolveProposal(p.Proposal, true)
	case "esc", "ctrl+c", "n", "q":
		a.resolveProposal(p.Proposal, false)
	case "ctrl+d", "pgdown", "down", "j":
		p.Scroll++
	case "ctrl+u", "pgup", "up", "k":
		p.Scroll = max(0, p.Scroll-1)
	}
	return nil
}

// ProposalH is the height the proposal box is given.
func (a *Core) ProposalH() int { return max(4, a.ScreenH()-4) }

// fit keeps the scroll of the proposal inside its lines. It is the part of
// drawing the proposal that changes state, so it runs before drawing (App.fit).
func (p *ProposalView) fit(a *Core) {
	p.Scroll = Clamp(p.Scroll, 0, max(0, len(p.Lines)-max(1, a.ProposalH()-3)))
}
