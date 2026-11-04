package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// The App is the TUI's core.Port: the core calls these directly, in the same
// process. Work that ends in a message for the program is queued in later and
// handed out by the next effectCmd.

// WriteClipboard puts s on the clipboard: through the helper tool of this
// machine when there is one, and through the terminal the program runs in (an
// OSC 52 write outside the frame), which also reaches the user's own machine
// over SSH.
func (a *App) WriteClipboard(s string) {
	core.WriteSystemClipboard(s)
	if a.caret != nil {
		_ = a.caret.Send([]byte(ansi.SetSystemClipboard(s)))
	}
}

// OpenURL opens url with the opener of this machine and tells the core how it
// went (core.OpenURLResult). Over SSH it does not try: the browser would not
// appear on the user's screen.
func (a *App) OpenURL(url string) {
	a.later = append(a.later, func() tea.Msg {
		if core.IsRemoteSession() {
			return core.OpenURLResult{URL: url, Err: core.ErrRemoteSession}
		}
		return core.OpenURLResult{URL: url, Err: core.OpenExternal(url)}
	})
}

// Exit ends the program. From here on the View keeps asking the terminal for
// the same keyboard enhancements: when they change, Bubble Tea queries the
// terminal, and the reply (CSI ? flags u) can arrive after input is no longer
// read, so the shell would get it as typed text such as "3u".
func (a *App) Exit() {
	a.quitting = true
	a.later = append(a.later, tea.Quit)
}
