package core

import (
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
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
	termKeys          []Key
	termRuns          []string
	muxSent           []string
}

func (f *testFront) ScreenW() int            { return f.w }
func (f *testFront) ScreenH() int            { return f.h }
func (f *testFront) SideShown() bool         { return f.side }
func (f *testFront) TermShown() bool         { return f.term }
func (f *testFront) AgentOpen() bool         { return f.agent }
func (f *testFront) SetSide(on bool)         { f.side = on }
func (f *testFront) SetTerm(on bool)         { f.term = on }
func (f *testFront) SetAgent(on bool)        { f.agent = on }
func (f *testFront) AgentWidthPref() int     { return 0 }
func (f *testFront) ExplorerShare() int      { return 0 }
func (f *testFront) InResizeMode() bool      { return f.resize }
func (f *testFront) DragBorder(ResizeTarget) {}
func (f *testFront) ReleasesReported() bool  { return f.releases }
func (f *testFront) SetReleasesReported(on bool) {
	f.releases = on
}
func (f *testFront) SendClipboard(s string)       { f.clip = s }
func (f *testFront) TermBlur()                    {}
func (f *testFront) TermFocus() []Effect          { return nil }
func (f *testFront) TermCloseAll()                {}
func (f *testFront) TermKey(k Key)                { f.termKeys = append(f.termKeys, k) }
func (f *testFront) TermRun(line string) []Effect { f.termRuns = append(f.termRuns, line); return nil }
func (f *testFront) TermPaste(string) []Effect    { return nil }
func (f *testFront) TermShell() Shell             { return nil }
func (f *testFront) MuxReset()                    {}
func (f *testFront) MuxOn() bool                  { return false }
func (f *testFront) MuxOpen() error               { return nil }
func (f *testFront) MuxHide() error               { return nil }
func (f *testFront) MuxToggle() []Effect          { return nil }
func (f *testFront) MuxSend(text string, _ bool) []Effect {
	f.muxSent = append(f.muxSent, text)
	return nil
}
func (f *testFront) MuxClose()                       {}
func (f *testFront) MuxResult(error) []Effect        { return nil }
func (f *testFront) CommandEnded(int)                {}
func (f *testFront) StartResize() []Effect           { f.resize = true; return nil }
func (f *testFront) HandleResizeKey(string) []Effect { return nil }
func (f *testFront) ScrollHover(int)                 {}
func (f *testFront) HoverCellWord() (buffer.Pos, WordSpan, bool) {
	return buffer.Pos{}, WordSpan{}, false
}
func (f *testFront) SuggestionsShown() bool         { return false }
func (f *testFront) ResetPanes()                    {}
func (f *testFront) SetScreen(w, h int)             { f.w, f.h = w, h }
func (f *testFront) SetPalette(t *Theme)            { ActivateTheme(t) }
func (f *testFront) HandleTermEvent(Event) []Effect { return nil }
func (f *testFront) HandleMouse(Mouse) []Effect     { return nil }
func (f *testFront) NewTextArea(initial string) TextArea {
	return &testArea{text: initial}
}

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

func (f *testFront) EditRows() int {
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

func (f *testFront) TextCols(*Tab) int { return f.rightW() }
func (f *testFront) DiffBaseCols() int { return 0 }
func (f *testFront) EditorCols() int   { return f.rightW() }

// testArea is a TextArea that holds its text and nothing else.
type testArea struct{ text string }

func (a *testArea) Value() string                   { return a.text }
func (a *testArea) SetValue(s string)               { a.text = s }
func (a *testArea) InsertString(s string)           { a.text += s }
func (a *testArea) Line() int                       { return strings.Count(a.text, "\n") }
func (a *testArea) Column() int                     { return len([]rune(a.text[strings.LastIndex(a.text, "\n")+1:])) }
func (a *testArea) SetWidth(int)                    {}
func (a *testArea) View() string                    { return a.text }
func (a *testArea) CursorCell() (x, y int, ok bool) { return a.Column(), a.Line(), true }
func (a *testArea) Update(in Input) {
	switch in := in.(type) {
	case Paste:
		a.text += in.Content
	case Key:
		a.text += in.Text
	}
}

// newTestCore is a Core rooted at dir, drawn by a testFront of w x h cells.
func newTestCore(t testing.TB, dir, file string, w, h int) (*Core, *testFront) {
	t.Helper()
	f := &testFront{w: w, h: h, side: true}
	c := New(f, dir, file)
	f.c = c
	return c, f
}

// App is the name the tests use for Core.
type App = Core

// newApp is a Core rooted at dir, as New makes it, on a screen of no size
// until a test sets one (SetScreen).
func newApp(dir, file string) *Core {
	f := &testFront{side: true}
	c := New(f, dir, file)
	f.c = c
	return c
}

// newSingleCore is NewSingle's Core.
func newSingleCore(file string) (*Core, error) {
	f := &testFront{}
	c, err := NewSingle(f, file)
	if c != nil {
		f.c = c
	}
	return c, err
}
