package tui

import (
	"context"
	"errors"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

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
		case core.WaitTerm:
			from := e.From
			cmds = append(cmds, func() tea.Msg { return <-from })
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
		case core.Quit:
			// From here on the View keeps asking the terminal for the same
			// keyboard enhancements (see exit).
			a.quitting = true
			cmds = append(cmds, tea.Quit)
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
	return tea.Batch(cmds...)
}

// HandleTermEvent is the shell of the terminal panel printing or ending.
func (a *App) HandleTermEvent(ev core.Event) []core.Effect {
	effs := a.term.update(ev)
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
		a.showTerm = false
		if a.FocusIs(core.FocusTerminal) {
			return core.Batch(effs, a.SetFocus(core.FocusEditor))
		}
	}
	return effs
}

// termFailed is no shell starting: say so and close the panel.
func (a *App) termFailed(err error) []core.Effect {
	a.Fail(err)
	if a.term.sh == nil && a.showTerm {
		a.showTerm = false
		if a.FocusIs(core.FocusTerminal) {
			return a.SetFocus(core.FocusEditor)
		}
	}
	return nil
}

// termEffects is the result of a terminal call that may fail to start a shell.
func (a *App) termEffects(effs []core.Effect, err error) []core.Effect {
	if err != nil {
		return core.Batch(effs, a.termFailed(err))
	}
	return effs
}
