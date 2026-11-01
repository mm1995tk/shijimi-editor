package tui

// Host is the front end's side for what the core cannot do itself. In `shj
// serve` the GUI sits on the user's machine, so the OS clipboard and the
// browser are the GUI's, not the server's (clipboard.write and url.open in
// docs/design/gui-protocol.md). The TUI has no Host: it writes the clipboard
// itself (a helper tool, OSC 52) and opens the browser itself.
type Host interface {
	WriteClipboard(text string)
	OpenURL(url string)
}

// SetHost sets the Host. nil is the TUI.
func (a *App) SetHost(h Host) { a.host = h }

// openInBrowser opens url on the user's screen: through the Host when there is
// one, otherwise with the opener of this machine.
func (a *App) openInBrowser(url string) error {
	if a.host != nil {
		a.host.OpenURL(url)
		return nil
	}
	return openExternal(url)
}
