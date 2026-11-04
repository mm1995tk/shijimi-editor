package tui

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// effectCmd turns Effects into the tea.Cmd that carries them out. An Event
// that a timer delivers reaches Update as the tea.Msg itself.
func (a *App) effectCmd(effs ...core.Effect) tea.Cmd {
	var cmds []tea.Cmd
	for _, e := range effs {
		switch e := e.(type) {
		case core.StartTimer:
			fire := e.Fire
			cmds = append(cmds, tea.Tick(e.After, func(time.Time) tea.Msg { return fire }))
		case core.RunLSP:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), core.LspTimeout)
				defer cancel()
				if ev := run(ctx); ev != nil {
					return ev
				}
				return nil
			})
		case core.WaitLSP:
			from := e.From
			cmds = append(cmds, func() tea.Msg { return core.LspEventMsg(<-from) })
		case core.RunSearch:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				if ev := run(); ev != nil {
					return ev
				}
				return nil
			})
		case core.WaitAgent:
			from := e.From
			cmds = append(cmds, func() tea.Msg { return <-from })
		case core.RunAgent:
			run := e.Run
			cmds = append(cmds, func() tea.Msg { run(); return nil })
		case core.RunProbe:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				if ev := run(); ev != nil {
					return ev
				}
				return nil
			})
		case core.ReplyCtl:
			e.To <- e.Resp
		case core.ReplyIDE:
			e.To <- e.Reply
		case core.Run:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				if ev := run(); ev != nil {
					return ev
				}
				return nil
			})
		}
	}
	cmds = append(cmds, a.later...)
	a.later = nil
	return tea.Batch(cmds...)
}

// handleTermEvent is the shell of the terminal panel printing or ending. It
// reads what the shell printed for the core (scanTerm).
func (a *App) handleTermEvent(ev tea.Msg) {
	a.later = append(a.later, a.term.update(ev))
	defer a.scanTerm()
	// A program in the terminal panel (Claude Code, tmux, …) copies with
	// OSC 52; pass the text on to the clipboard. Any program there can do
	// this unasked, so every copy is shown.
	if tab := a.term.sh; tab != nil {
		c, ok := tab.vt.TakeClipboard()
		switch {
		case !ok:
		case c.TooLarge:
			a.Fail(errors.New(i18n.T("app.termcopybig")))
		default:
			a.CopyText(c.Text)
			a.Info(i18n.T("app.termcopied"), utf8.RuneCountInString(c.Text))
		}
	}
	if a.term.sh == nil && a.showTerm {
		a.ShowTerm(false)
		if a.FocusIs(core.FocusTerminal) {
			a.SetFocus(core.FocusEditor)
		}
	}
}

// termFailed is no shell starting: say so and close the panel.
func (a *App) termFailed(err error) {
	a.Fail(err)
	if a.term.sh == nil && a.showTerm {
		a.ShowTerm(false)
		if a.FocusIs(core.FocusTerminal) {
			a.SetFocus(core.FocusEditor)
		}
	}
}

// termStarted takes the result of a terminal call that may start a shell: the
// wait for its output is started by the next effectCmd; a shell that cannot
// start is shown as a failure.
func (a *App) termStarted(cmd tea.Cmd, err error) {
	if err != nil {
		a.termFailed(err)
		return
	}
	a.later = append(a.later, cmd)
}
