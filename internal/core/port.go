package core

// Port is the one interface the core knows its UI by (docs/design/gui-protocol.md,
// 「core が知る相手は Port だけ」). The TUI and the adapter of `shj serve` meet it.
// Its methods mean the same in every UI and return nothing: the core goes on
// when it has called one, and an answer that matters comes back later as an
// Event (OpenURLResult).
type Port interface {
	// WriteClipboard puts text on the user's OS clipboard.
	WriteClipboard(text string)
	// OpenURL opens url in the user's browser. A UI that can tell whether it
	// worked answers with an OpenURLResult.
	OpenURL(url string)
	// Exit says the program may end. A core of `shj serve` never ends (closing
	// the GUI leaves the server running), so its adapter drops it.
	Exit()
	// ShowSide, ShowTerm and ShowAgent open or close the sidebar, the terminal
	// panel and the suggestion panel. The UI answers with a Layout.
	ShowSide(on bool)
	ShowTerm(on bool)
	ShowAgent(on bool)
	// DragBorder starts a drag of the border r with the pointer.
	DragBorder(r ResizeTarget)
	// StartResize starts moving the borders with the keys; ResizeKey takes the
	// keys while Layout.ResizeMode is on; ResetPanes puts the borders back.
	// The UI answers with a Layout.
	StartResize()
	ResizeKey(key string)
	ResetPanes()
}

// nopPort is the Port of a core whose UI has not set one: it does nothing.
type nopPort struct{}

func (nopPort) WriteClipboard(string)   {}
func (nopPort) OpenURL(string)          {}
func (nopPort) Exit()                   {}
func (nopPort) ShowSide(bool)           {}
func (nopPort) ShowTerm(bool)           {}
func (nopPort) ShowAgent(bool)          {}
func (nopPort) DragBorder(ResizeTarget) {}
func (nopPort) StartResize()            {}
func (nopPort) ResizeKey(string)        {}
func (nopPort) ResetPanes()             {}

// SetPort sets the Port. The TUI sets itself (internal/tui.New); a core of
// `shj serve` gets the server's (Workspaces.SetPort).
func (a *Core) SetPort(p Port) {
	if p == nil {
		p = nopPort{}
	}
	a.port = p
}
