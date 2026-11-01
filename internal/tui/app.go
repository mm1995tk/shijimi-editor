// Package tui implements the shijimi terminal IDE: it draws a core.Core to a
// terminal and turns what bubbletea delivers into the Core's Input and Events.
package tui

import (
	"path/filepath"

	"github.com/mm1995tk/shijimi-editor/internal/caret"
	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// App is the root Bubble Tea model: a core.Core and what draws it to a
// terminal. It holds only what a terminal has (the size of the screen, which
// regions are shown, the pane sizes the user dragged to, the terminal under
// the editor); the Core is the front end's FrontEnd.
type App struct {
	*core.Core

	caret       *caret.Writer     // the terminal output; nil in tests
	cellSize    func() (w, h int) // terminal cell size in pixels; nil in tests
	w, h        int
	term        *terminal
	mux         muxPane // the multiplexer pane that replaces term inside a multiplexer
	showSide    bool
	showTerm    bool
	sideWPref   int
	panelHPref  int
	previewPct  int
	diffLeftPct int // base pane share of a split diff; 0 = half
	explorerPct int // file tree share of the diff mode sidebar; 0 = 7/10
	resizeMode  bool
	resizing    core.ResizeTarget // border being dragged with the mouse
	showAgent   bool
	agentWPref  int    // panel width picked by the user (0 = automatic)
	capture     core.Region // the region a held pointer button is captured by (see locate)
	lastClick   click  // the previous left press (see clicks in inputtea.go)
	keyReleases bool   // the terminal reports key releases
	askReleases bool   // key releases were asked for in the last View
	quitting    bool   // tea.Quit was returned (see exit)
}

// New creates the app rooted at dir, optionally opening file.
func New(dir, file string) *App {
	abs, _ := filepath.Abs(dir)
	a := &App{term: newTerminal(abs), showSide: true}
	a.Core = core.New(a, dir, file)
	return a
}

// NewSingle creates the app in single-file mode: only file is edited, with no
// file tree, language server, terminal, diff mode or agent panel, and the app
// ends when the file is closed. A file that does not exist opens as an empty
// buffer, which creates the file when saved. It is meant for $EDITOR.
func NewSingle(file string) (*App, error) {
	abs, _ := filepath.Abs(file)
	a := &App{term: newTerminal(filepath.Dir(abs))}
	c, err := core.NewSingle(a, file)
	if err != nil {
		return nil, err
	}
	a.Core = c
	return a, nil
}

// Close stops language servers and removes what the program left in the
// terminal's way; call after the program exits.
func (a *App) Close() {
	a.Core.Close()
	removeMuxShellIntegration()
}
