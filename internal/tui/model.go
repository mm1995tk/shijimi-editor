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

func (a *App) Init() tea.Cmd {
	if a.single {
		return a.effectCmd(watchTimer())
	}
	return a.effectCmd(append([]Effect{WaitLSP{a.lsp.events}, watchTimer()}, a.probeRepo()...)...)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	effs := a.update(a.fromTea(msg))
	a.keepTransients()
	a.normalizeSplit()
	a.trackLast()
	a.syncLSP()
	a.notifySelection()
	return a, a.effectCmd(batch(effs, a.scheduleAutoSave(), a.syncPreview(), a.lexTabs(), a.semanticTabs())...)
}
