package tui

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

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

// sideShown, termShown and agentOpen say which regions the user has on screen,
// and setSide, setTerm and setAgent turn them on and off. (agentShown, in
// agentpanel.go, is another question: whether the panel fits beside the editor.)
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

// frontEnd is everything the state that goes to core asks of the front end
// (a terminal): the calls above, the multiplexer pane, the borders the user
// drags and the few questions that need the screen's layout. Core holds the
// front end it is drawn by; App is the one there is (set in New).
type frontEnd interface {
	screenW() int
	screenH() int
	sideShown() bool
	termShown() bool
	agentOpen() bool
	setSide(on bool)
	setTerm(on bool)
	setAgent(on bool)
	agentWidthPref() int
	explorerShare() int
	inResizeMode() bool
	dragBorder(r resizeTarget)
	releasesReported() bool
	setReleasesReported(on bool)
	sendToTerminal(b []byte)
	termBlur()
	termFocus() []Effect
	termCloseAll()
	termKey(k Key)
	termRun(line string) []Effect
	termPaste(s string) []Effect
	termShell() *termTab
	muxReset()
	muxOn() bool
	muxOpen() error
	muxHide() error
	muxToggle() []Effect
	muxSend(text string, enter bool) []Effect
	muxClose()
	muxResult(err error) []Effect
	commandEnded(code int)
	startResize() []Effect
	handleResizeKey(key string) []Effect
	scrollHover(d int)
	hoverCellWord() (buffer.Pos, wordSpan, bool)
	editRows() int
	textCols(t *tab) int
	diffBaseCols() int
	suggestionsShown() bool
	editorCols() int
	resetPanes()
	setScreen(w, h int)
	setPalette(t *Theme)
	handleEvent(ev Event) []Effect
	handleMouse(msg Mouse) []Effect
}

var _ frontEnd = (*App)(nil)

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
func (a *App) setPalette(t *Theme) { applyTheme(t) }
