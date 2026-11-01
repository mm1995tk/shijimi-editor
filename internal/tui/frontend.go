package tui

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// frontend.go is what the state that goes to core asks of the terminal front
// end: how big the screen is, which regions are shown, what the user dragged
// the borders to, the terminal under the editor. The fields behind them
// stay on App (appOnlyFields in split_test.go); when App is split, these are
// the calls core makes on the front end.

// screenW and screenH are the size of the screen in cells.
func (a *App) ScreenW() int { return a.w }
func (a *App) ScreenH() int { return a.h }

// setScreen takes the size of the screen, and the cell size in pixels where
// the terminal says it.
func (a *App) SetScreen(w, h int) {
	a.w, a.h = w, h
	if a.cellSize != nil {
		updateCellSize(a.cellSize())
	}
}

// sideShown, termShown and agentOpen say which regions the user has on screen,
// and setSide, setTerm and setAgent turn them on and off. (agentShown, in
// agentpanel.go, is another question: whether the panel fits beside the editor.)
func (a *App) SideShown() bool  { return a.showSide }
func (a *App) TermShown() bool  { return a.showTerm }
func (a *App) AgentOpen() bool  { return a.showAgent }
func (a *App) SetSide(on bool)  { a.showSide = on }
func (a *App) SetTerm(on bool)  { a.showTerm = on }
func (a *App) SetAgent(on bool) { a.showAgent = on }

// agentWidthPref is the width of the suggestion panel the user dragged to
// (0 = automatic); explorerShare is the file tree's share of the diff mode
// sidebar in percent (0 = automatic).
func (a *App) AgentWidthPref() int { return a.agentWPref }
func (a *App) ExplorerShare() int  { return a.explorerPct }

// inResizeMode reports whether the keys move the borders; dragBorder starts
// a drag of one with the pointer.
func (a *App) InResizeMode() bool        { return a.resizeMode }
func (a *App) DragBorder(r ResizeTarget) { a.resizing = r }

// releasesReported reports whether the terminal sends key releases.
func (a *App) ReleasesReported() bool      { return a.keyReleases }
func (a *App) SetReleasesReported(on bool) { a.keyReleases = on }

// SendClipboard asks the terminal the program runs in to set the system
// clipboard (an OSC 52 write outside the frame). It does nothing without one.
func (a *App) SendClipboard(s string) {
	if a.caret != nil {
		_ = a.caret.Send([]byte(ansi.SetSystemClipboard(s)))
	}
}

// The terminal under the editor.
func (a *App) TermBlur()                    { a.term.blur() }
func (a *App) TermFocus() []Effect          { return a.termEffects(a.term.focus()) }
func (a *App) TermCloseAll()                { a.term.closeAll() }
func (a *App) TermKey(k Key)                { a.term.key(k) }
func (a *App) TermRun(line string) []Effect { return a.termEffects(a.term.run(line)) }
func (a *App) TermPaste(s string) []Effect  { return a.termEffects(a.term.paste(s)) }
func (a *App) TermShell() core.Shell {
	if a.term.sh == nil {
		return nil
	}
	return a.term.sh
}

// muxReset makes the multiplexer be looked for again (the config changed).
func (a *App) MuxReset() { a.mux.resolved = false }


// appOf is an App around c, for a Core made by hand (tests): c is drawn by
// the App it returns.
func appOf(c *Core) *App {
	a := &App{Core: c}
	c.frontEnd = a
	return a
}

// setPalette makes t the colors the front end draws with (applyTheme, in
// theme_apply.go, rewrites the palette variables and the styles built from
// them). Core keeps which theme is chosen; how it looks is the front end's.
func (a *App) SetPalette(t *Theme) { applyTheme(t) }
