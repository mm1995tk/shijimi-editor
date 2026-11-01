package core

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// coreq.go is what the files that stay in tui (the pointer, the layout, the
// terminal, the multiplexer) ask of Core, and the few things they tell it,
// in place of reading its fields (TestFrontEndAsksCore). When Core is its
// own package, these are its exported methods.

// findShown reports whether the find bar is open.
func (a *Core) FindShown() bool { return a.FindOpen }

// cmdlineOpen reports whether the bottom command line is open.
func (a *Core) CmdlineOpen() bool { return a.CmdOpen }

// singleFile reports whether this is the single file editor.
func (a *Core) SingleFile() bool { return a.single }

// helixOn reports whether the Helix keys are on.
func (a *Core) HelixOn() bool { return a.Helix }

// focusIs reports whether region f has the keys (focusIn is the other
// question: whether a floating window is over it).
func (a *Core) FocusIs(f Focus) bool { return a.Focus == f }

// followCursor makes the next frame scroll to the cursor.
func (a *Core) FollowCursor() { a.Follow = true }

// proposalCount is the number of agent changes waiting for the user.
func (a *Core) ProposalCount() int { return a.props.Len() }

// lspEvents is where the language servers send what they have to say.
func (a *Core) LspEvents() <-chan lsp.Event { return a.Lsp.events }

// settings is the configuration.
func (a *Core) Settings() *config.Config { return a.Cfg }

// rootDir is the directory the editor was opened on.
func (a *Core) RootDir() string { return a.Root }

// clearMsg clears the message on the status line.
func (a *Core) ClearMsg() { a.Msg = "" }

// closeCompletion closes the completion popup.
func (a *Core) CloseCompletion() { a.Comp = nil }

// cmdlineCaretCols is how far into the command line (the ":" or "/" and the
// text before the cursor) the cursor is, in cells.
func (a *Core) CmdlineCaretCols() int {
	v := []rune(a.cmdInput.Value())
	pos := Clamp(a.cmdInput.Position(), 0, len(v))
	return ansi.StringWidth(a.cmdKind) + ansi.StringWidth(string(v[:pos]))
}

// paletteCaretCols is how far into the palette's box (its label, then the
// query) the cursor is, in cells.
func (a *Core) PaletteCaretCols() int {
	return ansi.StringWidth(a.Pal.label()) + a.Pal.Input.Caret()
}

// findCaretCols is how far into the find bar (its label, then the query) the
// cursor is, in cells.
func (a *Core) FindCaretCols() int { return ansi.StringWidth(FindLabel()) + a.FindInput.Caret() }

// openCard is the review thread shown like a hover, or nil.
func (a *Core) OpenCard() *PrCard { return a.Card }

// openComposer is the review comment editor, or nil.
func (a *Core) OpenComposer() *composer { return a.Composer }

// composerArea is the text area of the comment editor (a copy to draw).
func (a *Core) ComposerArea() TextArea { return a.Composer.Ta }

// lineThreads are the review threads on line (1-based) of the file p.
func (a *Core) LineThreads(p string, line int) []*PrThread { return a.Gd.Rv.LineThreads(p, line) }

// otherPane is the side (0 = left) the pane that is not focused is on.
func (a *Core) OtherPane() int { return 1 - a.Pane }

// isDragging reports whether a drag with the pointer is going on in the
// editor or the file tree; setDragging starts and ends one.
func (a *Core) IsDragging() bool    { return a.dragging }
func (a *Core) SetDragging(on bool) { a.dragging = on }

// agentDragging reports whether the pointer is selecting text in the
// suggestion panel's log.
func (a *Core) AgentDragging() bool { return a.Ag.Sel.Dragging }

// findField and cmdField are the inputs of the find bar and the command line.
func (a *Core) FindField() *InputField { return &a.FindInput }
func (a *Core) CmdField() *InputField  { return &a.cmdInput }

// hoverContent is the text of the open hover popup; hoverTop is how far it
// is scrolled, and setHoverTop scrolls it.
func (a *Core) HoverContent() string { return a.HoverText }
func (a *Core) HoverTop() int        { return a.HoverScroll }
func (a *Core) SetHoverTop(n int)    { a.HoverScroll = n }

// searchWindow is the state of the search window.
func (a *Core) SearchWindow() *SearchPanel { return a.Search }

// contextPreview is the context preview that is open (nil when none is).
func (a *Core) ContextPreview() *CtxPreview { return a.Ctx }
