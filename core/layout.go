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

// SideShown reports whether the sidebar is wanted on screen. It is not on
// screen when the file tree is hidden (SetTreeHidden) and there is no CHANGES
// list to show; SideVisible says what is drawn.
func (a *Core) SideShown() bool { return a.layout.SideShown }

// SideVisible reports whether the sidebar is on screen: it is shown and has
// something in it. Without a file tree it has CHANGES in diff mode only (and not when the GUI shows them: SetChangesInGUI).
func (a *Core) SideVisible() bool {
	return a.layout.SideShown && (!a.treeHidden || a.ChangesListShown())
}

// SetChangesInGUI says the GUI shows the CHANGES list of the diff mode against
// HEAD in its own sidebar (the shj in the primary terminal of a GUI workspace
// whose kisui replied "changes"). The shj then draws no list of its own for
// that diff mode and tells the GUI the list instead (ChangesReport). The diff
// mode against a pull request keeps the shj's own list unless the GUI also
// shows that one (SetChangesPRInGUI).
func (a *Core) SetChangesInGUI(on bool) { a.changesInGUI = on }

// SetChangesPRInGUI says the GUI also shows the CHANGES list of the diff mode
// against a pull request (the kisui replied "changesPR"). It has an effect only
// together with SetChangesInGUI.
func (a *Core) SetChangesPRInGUI(on bool) { a.changesPRInGUI = on }

// changesInGUIFor reports whether the GUI lists the changes of the diff mode
// against base.
func (a *Core) changesInGUIFor(base string) bool {
	return a.changesInGUI && (base == BaseHead || (base == BasePR && a.changesPRInGUI))
}

// ChangesListShown reports whether the shj draws the CHANGES list itself: in
// diff mode, unless the GUI shows the list of its base (HEAD or the PR).
func (a *Core) ChangesListShown() bool {
	return a.Gd.On && !a.changesInGUIFor(a.Gd.Base)
}

// ChangesListInGUI reports whether the GUI shows the CHANGES list now: the diff
// mode against HEAD, with the GUI listing it (SetChangesInGUI).
func (a *Core) ChangesListInGUI() bool {
	return a.Gd.On && a.changesInGUIFor(a.Gd.Base)
}

// SetTreeHidden takes the file tree out of the sidebar, or puts it back. It is
// for a shj whose tree is somewhere else (the GUI sidebar, for the shj in the
// primary terminal of a workspace). The sidebar then holds only the CHANGES
// list of diff mode, and nothing moves the keys to the tree.
func (a *Core) SetTreeHidden(on bool) {
	a.treeHidden = on
	if on && a.Focus == FocusExplorer {
		a.Focus = a.explorerFocus()
	}
}

// SetGUIExplorer sets how Space e asks the GUI to focus the file explorer that
// stands in for the hidden file tree. f returns an error when the GUI cannot
// (a kisui that does not know the request); the keys then go where they would
// without it. Pass nil outside a GUI workspace.
func (a *Core) SetGUIExplorer(f func() error) { a.guiExplorer = f }

// SetGUIChanges sets how Space E (and the "Diff: Go to the changes list"
// command) asks the GUI to focus the CHANGES list it shows (SetChangesInGUI).
// f returns an error when the GUI cannot (a kisui that does not know the
// request); nothing moves then. Pass nil outside a GUI workspace.
func (a *Core) SetGUIChanges(f func() error) { a.guiChanges = f }

// TreeHidden reports whether the file tree is left out of the sidebar.
func (a *Core) TreeHidden() bool { return a.treeHidden }

// explorerFocus is where the keys go when they would go to the file tree: the
// tree itself, or without one, the CHANGES list in diff mode and the editor
// otherwise.
func (a *Core) explorerFocus() Focus {
	switch {
	case !a.treeHidden:
		return FocusExplorer
	case a.ChangesListShown():
		return FocusChanges
	}
	return FocusEditor
}

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
