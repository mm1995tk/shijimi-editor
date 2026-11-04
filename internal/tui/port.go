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

// ShowSide shows or hides the sidebar.
func (a *App) ShowSide(on bool) { a.showSide = on; a.pushLayout() }

// ShowTerm shows or hides the terminal panel. Inside a multiplexer it shows the
// pane and gives it the keys, or hides it (mux.go).
func (a *App) ShowTerm(on bool) {
	if a.muxOn() {
		if on {
			a.muxResult(a.muxOpen())
		} else {
			a.muxResult(a.muxHide())
		}
		return
	}
	a.showTerm = on
	a.pushLayout()
}

// ShowAgent turns the suggestion panel on or off.
func (a *App) ShowAgent(on bool) { a.showAgent = on; a.pushLayout() }

// DragBorder starts a drag of the border r with the pointer.
func (a *App) DragBorder(r core.ResizeTarget) { a.resizing = r }

// pushLayout hands the core the regions shown and the shares dragged to now
// (core.Layout). It runs whenever one changes, and after every message.
func (a *App) pushLayout() {
	if a.Core == nil {
		return
	}
	a.Core.SetLayout(core.Layout{
		SideShown: a.showSide, TermShown: a.showTerm, AgentOpen: a.showAgent,
		AgentWidthPref: a.agentWPref, ExplorerShare: a.explorerPct,
		ResizeMode: a.resizeMode, TermInPane: a.muxOn(),
	})
}

// TermFocus gives the keys to the terminal panel, or takes them away. The
// panel is never empty once it has the keys: a shell starts if it has none.
func (a *App) TermFocus(on bool) {
	if on && !a.muxOn() {
		a.termStarted(a.term.focus())
	}
}

// TermKey sends k to the terminal panel's shell.
func (a *App) TermKey(k core.Key) { a.term.key(k) }

// TermRun runs line in the terminal panel.
func (a *App) TermRun(line string) {
	if a.muxOn() {
		a.muxSend(line, true)
		return
	}
	a.termStarted(a.term.run(line))
}

// TermPaste pastes s into the terminal panel.
func (a *App) TermPaste(s string) {
	if a.muxOn() {
		a.muxSend(s, false)
		return
	}
	a.termStarted(a.term.paste(s))
}

// TermToggle is the toggle of Ctrl+`, Space t and :sh for the multiplexer pane
// (the core asks only when Layout.TermInPane is set).
func (a *App) TermToggle() { a.muxToggle() }

// TermClose closes the shell of the terminal panel.
func (a *App) TermClose() {
	a.term.closeAll()
	a.muxClose()
}

// hideTermPanel closes the built-in terminal panel (its shell is gone), which
// is never the multiplexer pane.
func (a *App) hideTermPanel() {
	a.showTerm = false
	a.pushLayout()
}
