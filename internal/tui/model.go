package tui

import "github.com/mm1995tk/shijimi-editor/internal/core"

import tea "charm.land/bubbletea/v2"

// model.go is the part of App that talks to bubbletea: what it draws and asks
// of the terminal each frame, and the two calls bubbletea makes (Init, Update). It stays in tui.

// View is bubbletea's view of the app: the frame (after fit brought the state up
// to date) and the modes it needs of the terminal (alt screen, mouse motion,
// key releases, where the IME composes).
func (a *App) View() tea.View {
	a.fit()
	frame, caretAt := a.renderFrame()
	v := tea.NewView(frame)
	// Image data goes out before the frame whose placeholder cells show it.
	if g := core.TakeGraphics(); g != "" && a.caret != nil {
		_ = a.caret.Send([]byte(g))
	}
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if a.MouseHover() {
		// A resting pointer is only noticed if motion without a button is reported.
		v.MouseMode = tea.MouseModeAllMotion
	}
	// askReleases is decided in fit.
	v.KeyboardEnhancements.ReportEventTypes = a.askReleases
	// The terminal cursor stays hidden (see package caret), but an IME shows
	// its composition where it is.
	v.Cursor = caretAt
	return v
}

// Init is bubbletea's first call: the effects that start the app (the watch
// timer; unless in single-file mode, waiting on the language servers and
// probing the repository).
func (a *App) Init() tea.Cmd {
	if a.SingleFile() {
		return a.effectCmd(core.WatchTimer())
	}
	return a.effectCmd(append([]core.Effect{core.WaitLSP{a.LspEvents()}, core.WatchTimer()}, a.ProbeRepo()...)...)
}

// Update is bubbletea's call for each message: fromTea turns it into an Input
// or an Event for Core.Handle, the state is brought up to date (transients,
// split, last tab, LSP, selection), and the effects of the message and the
// ones every round owes (auto-save, preview, lexing) become one tea.Cmd.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	effs := a.Handle(a.fromTea(msg))
	a.KeepTransients()
	a.NormalizeSplit()
	a.TrackLast()
	a.SyncLSP()
	a.NotifySelection()
	return a, a.effectCmd(core.Batch(effs, a.ScheduleAutoSave(), a.SyncPreview(), a.LexTabs(), a.SemanticTabs())...)
}
