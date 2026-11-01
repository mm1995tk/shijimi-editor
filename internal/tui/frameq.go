package tui

import "github.com/mm1995tk/shijimi-editor/internal/screen"

// frameq.go is what the drawing of a frame (renderFrame) asks of the state:
// which region has the keys, what is open, the screens the state builds. They
// are the questions core answers once App is split (core.Screen() and its
// query methods, #384); renderFrame reads nothing else of App's state
// (TestRenderFrameReadsQueries).

// focusIn reports whether region f has the keys: it is the focused one and no
// floating window (palette, project search) is over it.
func (a *Core) focusIn(f focus) bool { return a.focus == f && !a.overlayOpen() }

// otherTab is the file of the pane that is not focused while the editor is
// split (nil = empty or not split).
func (a *Core) otherTab() *tab { return a.other }

// focusedFirst reports whether the focused pane is the first (left) one.
func (a *Core) focusedFirst() bool { return a.pane == 0 }

// diffMode reports whether the diff mode is on.
func (a *Core) diffMode() bool { return a.gd.on }

// explorerScreen is the file tree as shown in h rows, with activePath marked.
func (a *Core) explorerScreen(h int, activePath string) screen.Explorer {
	return a.explorer.screen(h, activePath)
}

// completing reports whether the completion popup is open.
func (a *Core) completing() bool { return a.comp != nil }

// hovering reports whether a hover popup is open.
func (a *Core) hovering() bool { return a.hoverText != "" }

// signatureTab is the tab whose signature help is to be shown, or nil when
// none is (a completion or a hover takes its place).
func (a *Core) signatureTab() *tab {
	if a.sig != nil && a.comp == nil && a.hoverText == "" && a.sigVisible() {
		return a.sig.tab
	}
	return nil
}

// signatureOverlay is the signature help as an Overlay.
func (a *Core) signatureOverlay() screen.Overlay { return a.sig.overlay() }

// cardOpen reports whether a review thread is shown (it gives way to the
// comment editor).
func (a *Core) cardOpen() bool { return a.card != nil && a.composer == nil }

// composing reports whether the review comment editor is open.
func (a *Core) composing() bool { return a.composer != nil }

// menuOpen reports whether the key menu for the pending keys is on: the
// palette and the command line hide it.
func (a *Core) menuOpen(pending string) bool {
	return a.pending == pending && !a.pal.open && !a.cmdOpen
}

// paletteOpen reports whether the command palette is open.
func (a *Core) paletteOpen() bool { return a.pal.open }

// paletteOverlay is the command palette as an Overlay.
func (a *Core) paletteOverlay() screen.Overlay { return a.pal.overlay() }

// paletteField is the palette's input line, which the drawing places the
// cursor in.
func (a *Core) paletteField() *inputField { return &a.pal.input }

// searchOpen reports whether the project search is open.
func (a *Core) searchOpen() bool { return a.search.open }

// contextOpen reports whether the context preview is open.
func (a *Core) contextOpen() bool { return a.ctx != nil }

// proposalsOpen reports whether an agent's changes are waiting for a decision.
func (a *Core) proposalsOpen() bool { return a.props.Len() > 0 }

// commandLineOpen reports whether the ":" / "/" line replaces the status line.
func (a *Core) commandLineOpen() bool { return a.cmdOpen }

// completionOverlay is the completion popup as an Overlay, showing at most
// rows of its items.
func (a *Core) completionOverlay(rows int) screen.Overlay { return a.comp.overlay(rows) }
