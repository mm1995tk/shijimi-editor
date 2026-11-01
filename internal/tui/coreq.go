package tui

import (
	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// coreq.go is what the files that stay in tui (the pointer, the layout, the
// terminal, the multiplexer) ask of Core, and the few things they tell it,
// in place of reading its fields (TestFrontEndAsksCore). When Core is its
// own package, these are its exported methods.

// findShown reports whether the find bar is open.
func (a *Core) findShown() bool { return a.findOpen }

// cmdlineOpen reports whether the bottom command line is open.
func (a *Core) cmdlineOpen() bool { return a.cmdOpen }

// singleFile reports whether this is the single file editor.
func (a *Core) singleFile() bool { return a.single }

// helixOn reports whether the Helix keys are on.
func (a *Core) helixOn() bool { return a.helix }

// focusIs reports whether region f has the keys (focusIn is the other
// question: whether a floating window is over it).
func (a *Core) focusIs(f focus) bool { return a.focus == f }

// followCursor makes the next frame scroll to the cursor.
func (a *Core) followCursor() { a.follow = true }

// proposalCount is the number of agent changes waiting for the user.
func (a *Core) proposalCount() int { return a.props.Len() }

// lspEvents is where the language servers send what they have to say.
func (a *Core) lspEvents() <-chan lsp.Event { return a.lsp.events }

// settings is the configuration.
func (a *Core) settings() *config.Config { return a.cfg }

// rootDir is the directory the editor was opened on.
func (a *Core) rootDir() string { return a.root }

// clearMsg clears the message on the status line.
func (a *Core) clearMsg() { a.msg = "" }

// closeCompletion closes the completion popup.
func (a *Core) closeCompletion() { a.comp = nil }

// cmdlineCaretCols is how far into the command line (the ":" or "/" and the
// text before the cursor) the cursor is, in cells.
func (a *Core) cmdlineCaretCols() int {
	v := []rune(a.cmdInput.Value())
	pos := clamp(a.cmdInput.Position(), 0, len(v))
	return ansi.StringWidth(a.cmdKind) + ansi.StringWidth(string(v[:pos]))
}

// paletteCaretCols is how far into the palette's box (its label, then the
// query) the cursor is, in cells.
func (a *Core) paletteCaretCols() int {
	return ansi.StringWidth(a.pal.label()) + a.pal.input.Caret()
}

// findCaretCols is how far into the find bar (its label, then the query) the
// cursor is, in cells.
func (a *Core) findCaretCols() int { return ansi.StringWidth(findLabel()) + a.findInput.Caret() }

// openCard is the review thread shown like a hover, or nil.
func (a *Core) openCard() *prCard { return a.card }

// openComposer is the review comment editor, or nil.
func (a *Core) openComposer() *composer { return a.composer }

// composerArea is the text area of the comment editor (a copy to draw).
func (a *Core) composerArea() textarea.Model { return a.composer.ta }

// lineThreads are the review threads on line (1-based) of the file p.
func (a *Core) lineThreads(p string, line int) []*prThread { return a.gd.rv.lineThreads(p, line) }

// otherPane is the side (0 = left) the pane that is not focused is on.
func (a *Core) otherPane() int { return 1 - a.pane }

// isDragging reports whether a drag with the pointer is going on in the
// editor or the file tree; setDragging starts and ends one.
func (a *Core) isDragging() bool    { return a.dragging }
func (a *Core) setDragging(on bool) { a.dragging = on }

// agentDragging reports whether the pointer is selecting text in the
// suggestion panel's log.
func (a *Core) agentDragging() bool { return a.ag.sel.dragging }

// findField and cmdField are the inputs of the find bar and the command line.
func (a *Core) findField() *inputField { return &a.findInput }
func (a *Core) cmdField() *inputField  { return &a.cmdInput }

// hoverContent is the text of the open hover popup; hoverTop is how far it
// is scrolled, and setHoverTop scrolls it.
func (a *Core) hoverContent() string { return a.hoverText }
func (a *Core) hoverTop() int        { return a.hoverScroll }
func (a *Core) setHoverTop(n int)    { a.hoverScroll = n }

// searchWindow is the state of the search window.
func (a *Core) searchWindow() *searchPanel { return a.search }

// contextPreview is the context preview that is open (nil when none is).
func (a *Core) contextPreview() *ctxPreview { return a.ctx }
