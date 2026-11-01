package core

import "github.com/mm1995tk/shijimi-editor/internal/screen"

// frameq.go is what the drawing of a frame (internal/tui's renderFrame) asks
// of Core: which region has the keys, what is open, the screens Core builds.

// FocusIn reports whether region f has the keys: it is the focused one and no
// floating window (palette, project search) is over it.
func (a *Core) FocusIn(f Focus) bool { return a.Focus == f && !a.overlayOpen() }

// OtherTab is the file of the pane that is not focused while the editor is
// split (nil = empty or not split).
func (a *Core) OtherTab() *Tab { return a.Other }

// FocusedFirst reports whether the focused pane is the first (left) one.
func (a *Core) FocusedFirst() bool { return a.Pane == 0 }

// DiffMode reports whether the diff mode is on.
func (a *Core) DiffMode() bool { return a.Gd.On }

// ExplorerScreen is the file tree as shown in h rows, with activePath marked.
func (a *Core) ExplorerScreen(h int, activePath string) screen.Explorer {
	return a.Explorer.Screen(h, activePath)
}

// Completing reports whether the completion popup is open.
func (a *Core) Completing() bool { return a.Comp != nil }

// Hovering reports whether a hover popup is open.
func (a *Core) Hovering() bool { return a.HoverText != "" }

// SignatureTab is the tab whose signature help is to be shown, or nil when
// none is (a completion or a hover takes its place).
func (a *Core) SignatureTab() *Tab {
	if a.sig != nil && a.Comp == nil && a.HoverText == "" && a.sigVisible() {
		return a.sig.tab
	}
	return nil
}

// SignatureOverlay is the signature help as an Overlay.
func (a *Core) SignatureOverlay() screen.Overlay { return a.sig.Overlay() }

// CardOpen reports whether a review thread is shown (it gives way to the
// comment editor).
func (a *Core) CardOpen() bool { return a.Card != nil && a.Composer == nil }

// Composing reports whether the review comment editor is open.
func (a *Core) Composing() bool { return a.Composer != nil }

// MenuOpen reports whether the key menu for the pending keys is on: the
// palette and the command line hide it.
func (a *Core) MenuOpen(pending string) bool {
	return a.Pending == pending && !a.Pal.Open && !a.CmdOpen
}

// PaletteOpen reports whether the command palette is open.
func (a *Core) PaletteOpen() bool { return a.Pal.Open }

// PaletteOverlay is the command palette as an Overlay.
func (a *Core) PaletteOverlay() screen.Overlay { return a.Pal.Overlay() }

// PaletteField is the palette's input line, which the drawing places the
// cursor in.
func (a *Core) PaletteField() *InputField { return &a.Pal.Input }

// SearchOpen reports whether the project search is open.
func (a *Core) SearchOpen() bool { return a.Search.Open }

// ContextOpen reports whether the context preview is open.
func (a *Core) ContextOpen() bool { return a.Ctx != nil }

// ProposalsOpen reports whether an agent's changes are waiting for a decision.
func (a *Core) ProposalsOpen() bool { return a.props.Len() > 0 }

// CommandLineOpen reports whether the ":" / "/" line replaces the status line.
func (a *Core) CommandLineOpen() bool { return a.CmdOpen }

// CompletionOverlay is the completion popup as an Overlay, showing at most
// rows of its items.
func (a *Core) CompletionOverlay(rows int) screen.Overlay { return a.Comp.Overlay(rows) }
