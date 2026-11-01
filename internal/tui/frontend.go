package tui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// frontend.go is App's side of core.FrontEnd: how big the screen is, which
// regions are shown, what the user dragged the borders to, the terminal under
// the editor. The fields behind them are App's; Core asks through these calls.

// ScreenW is the width of the screen in cells.
func (a *App) ScreenW() int { return a.w }

// ScreenH is the height of the screen in cells.
func (a *App) ScreenH() int { return a.h }

// SetScreen takes the size of the screen, and the cell size in pixels where
// the terminal says it.
func (a *App) SetScreen(w, h int) {
	a.w, a.h = w, h
	if a.cellSize != nil {
		core.UpdateCellSize(a.cellSize())
	}
}

// SideShown reports whether the user has the sidebar on screen.
func (a *App) SideShown() bool { return a.showSide }

// TermShown reports whether the terminal panel is on screen.
func (a *App) TermShown() bool { return a.showTerm }

// AgentOpen reports whether the suggestion panel is on. (Core.AgentShown is
// another question: whether the panel fits beside the editor.)
func (a *App) AgentOpen() bool { return a.showAgent }

// SetSide shows or hides the sidebar.
func (a *App) SetSide(on bool) { a.showSide = on }

// SetTerm shows or hides the terminal panel.
func (a *App) SetTerm(on bool) { a.showTerm = on }

// SetAgent turns the suggestion panel on or off.
func (a *App) SetAgent(on bool) { a.showAgent = on }

// AgentWidthPref is the width of the suggestion panel the user dragged to
// (0 = automatic).
func (a *App) AgentWidthPref() int { return a.agentWPref }

// ExplorerShare is the file tree's share of the diff mode sidebar in percent
// (0 = automatic).
func (a *App) ExplorerShare() int { return a.explorerPct }

// InResizeMode reports whether the keys move the borders.
func (a *App) InResizeMode() bool { return a.resizeMode }

// DragBorder starts a drag of the border r with the pointer.
func (a *App) DragBorder(r core.ResizeTarget) { a.resizing = r }

// ReleasesReported reports whether the terminal sends key releases.
func (a *App) ReleasesReported() bool { return a.keyReleases }

// SetReleasesReported records whether the terminal sends key releases.
func (a *App) SetReleasesReported(on bool) { a.keyReleases = on }

// SendClipboard asks the terminal the program runs in to set the system
// clipboard (an OSC 52 write outside the frame). It does nothing without one.
func (a *App) SendClipboard(s string) {
	if a.caret != nil {
		_ = a.caret.Send([]byte(ansi.SetSystemClipboard(s)))
	}
}

// TermBlur takes the keys away from the terminal panel.
func (a *App) TermBlur() { a.term.blur() }

// TermFocus gives the keys to the terminal panel.
func (a *App) TermFocus() []core.Effect { return a.termEffects(a.term.focus()) }

// TermCloseAll closes every shell of the terminal panel.
func (a *App) TermCloseAll() { a.term.closeAll() }

// TermKey sends k to the terminal panel's shell.
func (a *App) TermKey(k core.Key) { a.term.key(k) }

// TermRun runs line in the terminal panel.
func (a *App) TermRun(line string) []core.Effect { return a.termEffects(a.term.run(line)) }

// TermPaste pastes s into the terminal panel.
func (a *App) TermPaste(s string) []core.Effect { return a.termEffects(a.term.paste(s)) }

// TermShell is the shell of the terminal panel, or nil before one starts.
func (a *App) TermShell() core.Shell {
	if a.term.sh == nil {
		return nil
	}
	return a.term.sh
}

// MuxReset makes the multiplexer be looked for again (the config changed).
func (a *App) MuxReset() { a.mux.resolved = false }

// SetPalette makes t the colors the front end draws with (applyTheme, in
// theme_apply.go, rewrites the palette variables and the styles built from
// them). Core keeps which theme is chosen; how it looks is the front end's.
func (a *App) SetPalette(t *core.Theme) { applyTheme(t) }
