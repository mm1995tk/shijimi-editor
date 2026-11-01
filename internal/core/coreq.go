package core

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// coreq.go is what the front end's pointer, layout, terminal and multiplexer
// code asks of Core, and the few things it tells it.

// FindShown reports whether the find bar is open.
func (a *Core) FindShown() bool { return a.FindOpen }

// CmdlineOpen reports whether the bottom command line is open.
func (a *Core) CmdlineOpen() bool { return a.CmdOpen }

// SingleFile reports whether this is the single file editor.
func (a *Core) SingleFile() bool { return a.single }

// HelixOn reports whether the Helix keys are on.
func (a *Core) HelixOn() bool { return a.Helix }

// FocusIs reports whether region f has the keys (FocusIn is the other
// question: whether a floating window is over it).
func (a *Core) FocusIs(f Focus) bool { return a.Focus == f }

// FollowCursor makes the next frame scroll to the cursor.
func (a *Core) FollowCursor() { a.Follow = true }

// ProposalCount is the number of agent changes waiting for the user.
func (a *Core) ProposalCount() int { return a.props.Len() }

// LspEvents is where the language servers send what they have to say.
func (a *Core) LspEvents() <-chan lsp.Event { return a.Lsp.events }

// Settings is the configuration.
func (a *Core) Settings() *config.Config { return a.Cfg }

// RootDir is the directory the editor was opened on.
func (a *Core) RootDir() string { return a.Root }

// ClearMsg clears the message on the status line.
func (a *Core) ClearMsg() { a.Msg = "" }

// CloseCompletion closes the completion popup.
func (a *Core) CloseCompletion() { a.Comp = nil }

// CmdlineCaretCols is how far into the command line (the ":" or "/" and the
// text before the cursor) the cursor is, in cells.
func (a *Core) CmdlineCaretCols() int {
	v := []rune(a.cmdInput.Value())
	pos := Clamp(a.cmdInput.Position(), 0, len(v))
	return ansi.StringWidth(a.cmdKind) + ansi.StringWidth(string(v[:pos]))
}

// PaletteCaretCols is how far into the palette's box (its label, then the
// query) the cursor is, in cells.
func (a *Core) PaletteCaretCols() int {
	return ansi.StringWidth(a.Pal.label()) + a.Pal.Input.Caret()
}

// FindCaretCols is how far into the find bar (its label, then the query) the
// cursor is, in cells.
func (a *Core) FindCaretCols() int { return ansi.StringWidth(FindLabel()) + a.FindInput.Caret() }

// OpenCard is the review thread shown like a hover, or nil.
func (a *Core) OpenCard() *PrCard { return a.Card }

// OpenComposer is the review comment editor, or nil.
func (a *Core) OpenComposer() *composer { return a.Composer }

// ComposerArea is the text area of the comment editor (a copy to draw).
func (a *Core) ComposerArea() TextArea { return a.Composer.Ta }

// LineThreads are the review threads on line (1-based) of the file p.
func (a *Core) LineThreads(p string, line int) []*PrThread { return a.Gd.Rv.LineThreads(p, line) }

// OtherPane is the side (0 = left) the pane that is not focused is on.
func (a *Core) OtherPane() int { return 1 - a.Pane }

// IsDragging reports whether a drag with the pointer is going on in the
// editor or the file tree; SetDragging starts and ends one.
func (a *Core) IsDragging() bool { return a.dragging }

// SetDragging starts or ends a drag in the editor or the file tree.
func (a *Core) SetDragging(on bool) { a.dragging = on }

// AgentDragging reports whether the pointer is selecting text in the
// suggestion panel's log.
func (a *Core) AgentDragging() bool { return a.Ag.Sel.Dragging }

// FindField is the input of the find bar.
func (a *Core) FindField() *InputField { return &a.FindInput }

// CmdField is the input of the command line.
func (a *Core) CmdField() *InputField { return &a.cmdInput }

// HoverContent is the text of the open hover popup.
func (a *Core) HoverContent() string { return a.HoverText }

// HoverTop is how far the hover popup is scrolled.
func (a *Core) HoverTop() int { return a.HoverScroll }

// SetHoverTop scrolls the hover popup to row n.
func (a *Core) SetHoverTop(n int) { a.HoverScroll = n }

// SearchWindow is the state of the search window.
func (a *Core) SearchWindow() *SearchPanel { return a.Search }

// ContextPreview is the context preview that is open (nil when none is).
func (a *Core) ContextPreview() *CtxPreview { return a.Ctx }
