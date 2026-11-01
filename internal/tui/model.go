package tui

import tea "charm.land/bubbletea/v2"

// model.go is the part of App that talks to bubbletea: what it draws and asks
// of the terminal each frame. It stays in tui.

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
