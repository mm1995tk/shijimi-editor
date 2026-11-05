package core

// Layout is what the UI shows of the regions it lays out, as far as the core
// needs to know: whether the sidebar, the terminal panel and the suggestion
// panel are open, and the shares the user dragged the borders to. The UI owns
// them (a GUI keeps its own) and hands the core a Layout as an Input whenever
// one changes. The core asks for a change through the Port (ShowSide, ...) and
// reads the last Layout it was given.
type Layout struct {
	SideShown, TermShown, AgentOpen bool
	// AgentWidthPref is the width of the suggestion panel the user dragged to
	// (0 = automatic).
	AgentWidthPref int
	// ExplorerShare is the file tree's share of the diff mode sidebar in
	// percent (0 = automatic).
	ExplorerShare int
	// ResizeMode is whether the keys move the borders.
	ResizeMode bool
	// TermInPane is whether the terminal is not a panel of the UI but a pane
	// of something outside it (a terminal multiplexer). The core then leaves
	// showing, focusing and hiding it to the UI: it says TermToggle, ShowTerm,
	// TermRun and TermPaste, and keeps neither TermShown nor the terminal focus.
	TermInPane bool
}

func (Layout) input() {}

// ResizeTarget is a border the user drags or moves with the keys.
type ResizeTarget int

const (
	ResizeNone ResizeTarget = iota
	ResizeSide
	ResizeTerm
	ResizePreview
	ResizeAgent
	ResizeDiff
	ResizeExplorer
)

// SetLayout takes the layout the UI gave.
func (a *Core) SetLayout(l Layout) { a.layout = l }

// SideShown reports whether the sidebar is on screen.
func (a *Core) SideShown() bool { return a.layout.SideShown }

// TermShown reports whether the terminal panel is on screen.
func (a *Core) TermShown() bool { return a.layout.TermShown }

// TermInPane reports whether the UI's terminal is a pane outside the UI.
func (a *Core) TermInPane() bool { return a.layout.TermInPane }

// AgentOpen reports whether the suggestion panel is on. (AgentShown is
// another question: whether the panel fits beside the editor.)
func (a *Core) AgentOpen() bool { return a.layout.AgentOpen }

// AgentWidthPref is the width of the suggestion panel the user dragged to
// (0 = automatic).
func (a *Core) AgentWidthPref() int { return a.layout.AgentWidthPref }

// ExplorerShare is the file tree's share of the diff mode sidebar in percent
// (0 = automatic).
func (a *Core) ExplorerShare() int { return a.layout.ExplorerShare }

// InResizeMode reports whether the keys move the borders.
func (a *Core) InResizeMode() bool { return a.layout.ResizeMode }

// SetSide shows or hides the sidebar. The core goes on as if it were done;
// the UI's Layout confirms it.
func (a *Core) SetSide(on bool) { a.layout.SideShown = on; a.port.ShowSide(on) }

// SetTerm shows or hides the terminal panel.
func (a *Core) SetTerm(on bool) { a.layout.TermShown = on; a.port.ShowTerm(on) }

// SetAgent turns the suggestion panel on or off.
func (a *Core) SetAgent(on bool) { a.layout.AgentOpen = on; a.port.ShowAgent(on) }

// DragBorder starts a drag of the border r with the pointer.
func (a *Core) DragBorder(r ResizeTarget) { a.port.DragBorder(r) }

// StartResize starts moving the borders with the keys.
func (a *Core) StartResize() []Effect {
	a.layout.ResizeMode = true
	a.Info("resize.hint")
	a.port.StartResize()
	return nil
}

// ResetPanes puts the borders back where they start.
func (a *Core) ResetPanes() {
	a.layout.AgentWidthPref, a.layout.ExplorerShare = 0, 0
	a.port.ResetPanes()
}

// ResizeExitKey reports whether key leaves the resize mode. The core leaves it
// itself on these keys, as the UI may not push a Layout back (a GUI keeps its
// borders to itself); the UI still gets the key to stop its own drawing.
func ResizeExitKey(key string) bool {
	switch key {
	case "esc", "enter", "q", "alt+r":
		return true
	}
	return false
}
