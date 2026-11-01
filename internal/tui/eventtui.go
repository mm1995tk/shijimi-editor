package tui

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// effectCmd turns Effects into the tea.Cmd that carries them out. An Event
// that a timer delivers reaches Update as the tea.Msg itself.
func (a *App) effectCmd(effs ...Effect) tea.Cmd {
	var cmds []tea.Cmd
	for _, e := range effs {
		switch e := e.(type) {
		case StartTimer:
			fire := e.Fire
			cmds = append(cmds, tea.Tick(e.After, func(time.Time) tea.Msg { return fire }))
		case RunLSP:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), lspTimeout)
				defer cancel()
				if ev := run(ctx); ev != nil {
					return ev
				}
				return nil
			})
		case WaitLSP:
			from := e.From
			cmds = append(cmds, func() tea.Msg { return lspEventMsg(<-from) })
		case RunSearch:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				if ev := run(); ev != nil {
					return ev
				}
				return nil
			})
		case WaitTerm:
			from := e.From
			cmds = append(cmds, func() tea.Msg { return <-from })
		case WaitAgent:
			from := e.From
			cmds = append(cmds, func() tea.Msg { return <-from })
		case RunAgent:
			run := e.Run
			cmds = append(cmds, func() tea.Msg { run(); return nil })
		case RunProbe:
			run := e.Run
			cmds = append(cmds, func() tea.Msg {
				if ev := run(); ev != nil {
					return ev
				}
				return nil
			})
		case ReplyCtl:
			e.To <- e.Resp
		case ReplyIDE:
			e.To <- e.Reply
		case Quit:
			// From here on the View keeps asking the terminal for the same
			// keyboard enhancements (see exit).
			a.quitting = true
			cmds = append(cmds, tea.Quit)
		case Run:
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
func (a *App) HandleTermEvent(ev Event) []Effect {
	effs := a.term.update(ev)
	// A program in the terminal panel (Claude Code, tmux, …) copies with
	// OSC 52; pass the text on to the clipboard. Any program there can do
	// this unasked, so every copy is shown.
	if tab := a.term.sh; tab != nil {
		c, ok := tab.vt.TakeClipboard()
		switch {
		case !ok:
		case c.TooLarge:
			a.fail(errors.New(i18n.T("app.termcopybig")))
		default:
			a.copyText(c.Text)
			a.info(i18n.T("app.termcopied"), utf8.RuneCountInString(c.Text))
		}
	}
	if a.term.sh == nil && a.showTerm {
		a.showTerm = false
		if a.focusIs(focusTerminal) {
			return batch(effs, a.setFocus(focusEditor))
		}
	}
	return effs
}

// termFailed is no shell starting: say so and close the panel.
func (a *App) termFailed(err error) []Effect {
	a.fail(err)
	if a.term.sh == nil && a.showTerm {
		a.showTerm = false
		if a.focusIs(focusTerminal) {
			return a.setFocus(focusEditor)
		}
	}
	return nil
}

// termEffects is the result of a terminal call that may fail to start a shell.
func (a *App) termEffects(effs []Effect, err error) []Effect {
	if err != nil {
		return batch(effs, a.termFailed(err))
	}
	return effs
}
