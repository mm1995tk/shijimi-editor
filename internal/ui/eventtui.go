package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// cmdEffect carries a tea.Cmd through the Effect list. It is the bridge for work
// that has not moved to an Effect yet (saving, reloading the CHANGES list, the
// LSP request behind a hover); it goes away as P7a to P7e move those.
type cmdEffect struct{ cmd tea.Cmd }

func (cmdEffect) isEffect() {}

// effectCmd turns Effects into the tea.Cmd that carries them out. An Event
// that a timer delivers reaches Update as the tea.Msg itself.
func (a *App) effectCmd(effs ...Effect) tea.Cmd {
	var cmds []tea.Cmd
	for _, e := range effs {
		switch e := e.(type) {
		case StartTimer:
			fire := e.Fire
			cmds = append(cmds, tea.Tick(e.After, func(time.Time) tea.Msg { return fire }))
		case Quit:
			// From here on the View keeps asking the terminal for the same
			// keyboard enhancements (see exit).
			a.quitting = true
			cmds = append(cmds, tea.Quit)
		case cmdEffect:
			cmds = append(cmds, e.cmd)
		}
	}
	return tea.Batch(cmds...)
}

// handleEvent is the core's entry for Events. The TUI calls it from Update.
func (a *App) handleEvent(ev Event) []Effect {
	switch ev := ev.(type) {
	case AutoSaveDue:
		return a.handleAutoSave(ev)
	case WatchTick:
		return a.handleWatch()
	case HoldScrollTick:
		return a.holdScrollTick(ev.seq)
	case HoverDwell:
		return a.hoverDwell(ev)
	}
	return nil
}
