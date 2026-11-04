package core

import (
	"testing"
)

// testFront is the front end a Core is tested with: a screen of a given size
// whose layout is worked out the plain way (a sidebar, the editor, a panel
// under it), with no terminal behind it. What draws it in the program is
// internal/tui.App; its layout is tested there.
type testFront struct {
	c                 *Core
	w, h              int
	side, term, agent bool
	releases          bool
	resize            bool
	clip              string
	urls              []string
	exited            bool
	termKeys          []Key
	termRuns          []string
	termPasted        []string
	termFocused       bool
	termClosed        bool
	termToggled       int
	inPane            bool   // the terminal is a pane outside the UI (Layout.TermInPane)
	termShows         string // what ShowTerm was last told while inPane
}

// testFront is also the Port, recording what the core asks of the UI.
func (f *testFront) WriteClipboard(s string) { f.clip = s }
func (f *testFront) OpenURL(u string)        { f.urls = append(f.urls, u) }
func (f *testFront) Exit()                   { f.exited = true }

func (f *testFront) ShowSide(on bool) { f.side = on; f.push() }
func (f *testFront) ShowTerm(on bool) {
	if f.inPane {
		f.termShows = map[bool]string{true: "show", false: "hide"}[on]
		return
	}
	f.term = on
	f.push()
}
func (f *testFront) ShowAgent(on bool)       { f.agent = on; f.push() }
func (f *testFront) DragBorder(ResizeTarget) {}
func (f *testFront) ReleasesReported() bool  { return f.releases }
func (f *testFront) SetReleasesReported(on bool) {
	f.releases = on
}
func (f *testFront) TermFocus(on bool)   { f.termFocused = on }
func (f *testFront) TermKey(k Key)       { f.termKeys = append(f.termKeys, k) }
func (f *testFront) TermToggle()         { f.termToggled++ }
func (f *testFront) TermRun(line string) { f.termRuns = append(f.termRuns, line) }
func (f *testFront) TermPaste(s string)  { f.termPasted = append(f.termPasted, s) }
func (f *testFront) TermClose()          { f.termClosed = true }
func (f *testFront) CommandEnded(int)    {}
func (f *testFront) StartResize()        { f.resize = true; f.push() }
func (f *testFront) ResizeKey(string)    {}
func (f *testFront) ResetPanes()         {}
func (f *testFront) SetPalette(t *Theme) { ActivateTheme(t) }

// sideW is the sidebar's width and rightW the editor area's.
func (f *testFront) sideW() int {
	if !f.side {
		return 0
	}
	return Clamp(f.w/5, 18, 36)
}

func (f *testFront) rightW() int {
	if f.side {
		return max(0, f.w-f.sideW()-1)
	}
	return f.w
}

// sizes is what the UI would hand the core: the sizes of the plain layout.
func (f *testFront) sizes() Sizes {
	return Sizes{ScreenW: f.w, ScreenH: f.h, EditRows: f.editRows(), EditorCols: f.rightW(), TextCols: f.rightW()}
}

// push hands the core the layout and, once there is a screen, the sizes now.
func (f *testFront) push() {
	if f.c == nil {
		return
	}
	f.c.SetLayout(Layout{SideShown: f.side, TermShown: f.term, AgentOpen: f.agent, ResizeMode: f.resize, TermInPane: f.inPane})
	if f.w > 0 {
		f.c.SetSizes(f.sizes())
	}
}

// resized is a window of w x h cells: the core is told the sizes as an Input.
func resized(a *Core, w, h int) {
	f := a.FrontEnd.(*testFront)
	f.w, f.h = w, h
	a.Update(f.sizes())
}

func (f *testFront) editRows() int {
	top, editY := 0, 1
	if f.c.DiffMode() {
		top = 1
	}
	editY += top
	if f.c.FindShown() {
		editY++
	}
	panelH := 0
	if f.term {
		panelH = Clamp((f.h-1)*2/5, 6, max(6, f.h-8))
	}
	return max(0, top+f.c.BodyH()-editY-panelH)
}

// newTestCore is a Core rooted at dir, drawn by a testFront of w x h cells.
func newTestCore(t testing.TB, dir, file string, w, h int) (*Core, *testFront) {
	t.Helper()
	f := &testFront{w: w, h: h, side: true}
	c, _ := New(f, dir, file, Build{})
	c.SetPort(f)
	f.c = c
	f.push()
	return c, f
}

// App is the name the tests use for Core.
type App = Core

// newApp is a Core rooted at dir, as New makes it, on a screen of no size
// until a test sets one (resized).
func newApp(dir, file string) *Core {
	f := &testFront{side: true}
	c, _ := New(f, dir, file, Build{})
	c.SetPort(f)
	f.c = c
	f.push()
	return c
}

// newSingleCore is NewSingle's Core.
func newSingleCore(file string) (*Core, error) {
	f := &testFront{}
	c, err := NewSingle(f, file, Build{})
	if c != nil {
		c.SetPort(f)
		f.c = c
	}
	return c, err
}
