package tui

// frontend.go is what the state that goes to core asks of the terminal front
// end: how big the screen is, which regions are shown, what the user dragged
// the borders to, the terminal under the editor. The fields behind them
// stay on App (appOnlyFields in split_test.go); when App is split, these are
// the calls core makes on the front end.

// screenW and screenH are the size of the screen in cells.
func (a *App) screenW() int { return a.w }
func (a *App) screenH() int { return a.h }

// setScreen takes the size of the screen, and the cell size in pixels where
// the terminal says it.
func (a *App) setScreen(w, h int) {
	a.w, a.h = w, h
	if a.cellSize != nil {
		updateCellSize(a.cellSize())
	}
}

// sideShown, termShown and agentShown... say which regions are on screen.
func (a *App) sideShown() bool  { return a.showSide }
func (a *App) termShown() bool  { return a.showTerm }
func (a *App) agentOpen() bool  { return a.showAgent }
func (a *App) setSide(on bool)  { a.showSide = on }
func (a *App) setTerm(on bool)  { a.showTerm = on }
func (a *App) setAgent(on bool) { a.showAgent = on }

// agentWidthPref is the width of the suggestion panel the user dragged to
// (0 = automatic); explorerShare is the file tree's share of the diff mode
// sidebar in percent (0 = automatic).
func (a *App) agentWidthPref() int { return a.agentWPref }
func (a *App) explorerShare() int  { return a.explorerPct }

// inResizeMode reports whether the keys move the borders; dragBorder starts
// a drag of one with the pointer.
func (a *App) inResizeMode() bool        { return a.resizeMode }
func (a *App) dragBorder(r resizeTarget) { a.resizing = r }

// releasesReported reports whether the terminal sends key releases.
func (a *App) releasesReported() bool      { return a.keyReleases }
func (a *App) setReleasesReported(on bool) { a.keyReleases = on }

// sendToTerminal writes bytes to the terminal the program runs in, outside
// the frame (an OSC 52 clipboard write). It does nothing without one.
func (a *App) sendToTerminal(b []byte) {
	if a.caret != nil {
		_ = a.caret.Send(b)
	}
}

// The terminal under the editor.
func (a *App) termBlur()                    { a.term.blur() }
func (a *App) termFocus() []Effect          { return a.termEffects(a.term.focus()) }
func (a *App) termCloseAll()                { a.term.closeAll() }
func (a *App) termKey(k Key)                { a.term.key(k) }
func (a *App) termRun(line string) []Effect { return a.termEffects(a.term.run(line)) }
func (a *App) termPaste(s string) []Effect  { return a.termEffects(a.term.paste(s)) }
func (a *App) termShell() *termTab          { return a.term.sh }

// muxReset makes the multiplexer be looked for again (the config changed).
func (a *App) muxReset() { a.mux.resolved = false }
