package core

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

// ProposalWidth is the width of the proposal box on a screen w columns wide.
// The TUI and the GUI both use it.
func ProposalWidth(w int) int { return Clamp(w*4/5, 30, 140) }

// ProposalOverlay is the first waiting proposal as a screen.Overlay for the
// GUI: the title (with how many more wait), and the rows of the diff in view
// (the scroll fit has applied). The TUI draws the same state with ui/tui's
// renderProposalBox. The box is w columns wide, as the TUI's is.
func (a *Core) ProposalOverlay(w int) screen.Overlay {
	p := a.PropView()
	p.fit(a)
	body := max(1, a.ProposalH()-3)
	title := msg.T("idep.title", a.Rel(p.NewPath))
	if n := a.ProposalCount(); n > 1 {
		title = msg.Cat(title, msg.T("idep.waiting", n-1))
	}
	end := min(len(p.Lines), p.Scroll+body)
	ov := screen.Overlay{
		Kind:  screen.OverlayProposal,
		Title: title,
		View:  &screen.Viewport{Top: p.Scroll, Rows: body, Cols: w - 2, Total: len(p.Lines)},
	}
	if len(p.Lines) == 1 && p.Lines[0].Text.Key != "" {
		ov.Empty = p.Lines[0].Text // the diff has no lines: UnifiedDiff says so with a note
		return ov
	}
	for i, l := range p.Lines[p.Scroll:end] {
		ov.Lines = append(ov.Lines, proposalLine(p.Scroll+i, l))
	}
	return ov
}

// proposalLine is a row of the diff: the sign and the text, on the colour of
// an added or removed line, as the TUI paints it. The text of a diff line is
// never translated (the note of an empty diff is the only one: Overlay.Empty).
func proposalLine(no int, l PropLine) screen.Line {
	var text string
	for _, a := range l.Text.Args {
		text += fmt.Sprint(a)
	}
	text = strings.ReplaceAll(text, "\t", "    ")
	sl := screen.Line{No: no}
	switch l.Kind {
	case '+':
		sl.Band = screen.StyleDiffAddBg
	case '-':
		sl.Band = screen.StyleDiffDelBg
	}
	if l.Kind == '@' {
		sl.Spans = []screen.Span{{Text: " " + text, Style: screen.StyleDiffHunkFg}}
	} else {
		sl.Spans = []screen.Span{{Text: " " + string(l.Kind) + " " + text}}
	}
	return sl
}
