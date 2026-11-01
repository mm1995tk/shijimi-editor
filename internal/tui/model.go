package tui

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
	if g := takeGraphics(); g != "" && a.caret != nil {
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
	if a.singleFile() {
		return a.effectCmd(watchTimer())
	}
	return a.effectCmd(append([]Effect{WaitLSP{a.lspEvents()}, watchTimer()}, a.probeRepo()...)...)
}

// Update is bubbletea's call for each message: fromTea turns it into an Input
// or an Event for Core.update, the state is brought up to date (transients,
// split, last tab, LSP, selection), and the effects of the message and the
// ones every round owes (auto-save, preview, lexing) become one tea.Cmd.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	effs := a.update(a.fromTea(msg))
	a.keepTransients()
	a.normalizeSplit()
	a.trackLast()
	a.syncLSP()
	a.notifySelection()
	return a, a.effectCmd(batch(effs, a.scheduleAutoSave(), a.syncPreview(), a.lexTabs(), a.semanticTabs())...)
}
