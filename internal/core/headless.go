package core

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

// headless is the front end of a core nothing draws yet: a workspace of `shj
// serve` before a GUI looks at it. It has a screen of W x H cells with only the
// editor on it, no terminal panel and no multiplexer. The GUI will tell the
// sizes of its regions (view.resize) and pick the theme (theme.get); until
// then the core keeps its state as it would behind any screen.
type headless struct {
	// the screen in cells
	W, H int
	// core is the core it fronts (newHeadlessCore). The pointer is worked out
	// in the core, from the panes the GUI was sent (paneMouse).
	core *Core
}

// newHeadless is a headless front end of 80 x 24 cells.
func newHeadless() *headless { return &headless{W: 80, H: 24} }

// newHeadlessCore is a core rooted at dir on a headless front end.
func newHeadlessCore(sh *Shared, dir string) *Core {
	h := newHeadless()
	h.core = NewIn(sh, h, dir, "")
	return h.core
}

func (h *headless) ScreenW() int                  { return h.W }
func (h *headless) ScreenH() int                  { return h.H }
func (h *headless) SideShown() bool               { return false }
func (h *headless) TermShown() bool               { return false }
func (h *headless) AgentOpen() bool               { return false }
func (h *headless) SetSide(bool)                  {}
func (h *headless) SetTerm(bool)                  {}
func (h *headless) SetAgent(bool)                 {}
func (h *headless) AgentWidthPref() int           { return 0 }
func (h *headless) ExplorerShare() int            { return 0 }
func (h *headless) InResizeMode() bool            { return false }
func (h *headless) DragBorder(ResizeTarget)       {}
func (h *headless) ReleasesReported() bool        { return false }
func (h *headless) SetReleasesReported(bool)      {}
func (h *headless) SendClipboard(string)          {}
func (h *headless) TermBlur()                     {}
func (h *headless) TermFocus() []Effect           { return nil }
func (h *headless) TermCloseAll()                 {}
func (h *headless) TermKey(Key)                   {}
func (h *headless) TermRun(string) []Effect       { return nil }
func (h *headless) TermPaste(string) []Effect     { return nil }
func (h *headless) TermShell() Shell              { return nil }
func (h *headless) MuxReset()                     {}
func (h *headless) MuxOn() bool                   { return false }
func (h *headless) MuxOpen() error                { return nil }
func (h *headless) MuxHide() error                { return nil }
func (h *headless) MuxToggle() []Effect           { return nil }
func (h *headless) MuxSend(string, bool) []Effect { return nil }
func (h *headless) MuxClose()                     {}
func (h *headless) MuxResult(error) []Effect      { return nil }
func (h *headless) CommandEnded(int)              {}
func (h *headless) StartResize() []Effect         { return nil }
func (h *headless) HandleResizeKey(string) []Effect {
	return nil
}
func (h *headless) ScrollHover(int) {}
func (h *headless) HoverCellWord() (buffer.Pos, WordSpan, bool) {
	return buffer.Pos{}, WordSpan{}, false
}

// EditRows leaves out the status line.
func (h *headless) EditRows() int                  { return max(0, h.H-1) }
func (h *headless) TextCols(*Tab) int              { return h.W }
func (h *headless) DiffBaseCols() int              { return 0 }
func (h *headless) SuggestionsShown() bool         { return false }
func (h *headless) EditorCols() int                { return h.W }
func (h *headless) ResetPanes()                    {}
func (h *headless) SetScreen(w, ht int)            { h.W, h.H = w, ht }
func (h *headless) SetPalette(*Theme)              {} // the theme is the process's (ActivateTheme); the GUI asks for it with theme.get
func (h *headless) HandleTermEvent(Event) []Effect { return nil }
func (h *headless) HandleMouse(m Mouse) []Effect {
	if h.core == nil {
		return nil
	}
	return h.core.paneMouse(m)
}
func (h *headless) NewTextArea(initial string) TextArea {
	return &plainArea{text: initial}
}

// plainArea is a TextArea that holds its text: typed text and pastes go at the
// end. The GUI's review comment editor is its own; this keeps the core whole
// until then.
type plainArea struct{ text string }

func (p *plainArea) Value() string         { return p.text }
func (p *plainArea) SetValue(s string)     { p.text = s }
func (p *plainArea) InsertString(s string) { p.text += s }
func (p *plainArea) Line() int {
	n := 0
	for _, r := range p.text {
		if r == '\n' {
			n++
		}
	}
	return n
}
func (p *plainArea) Column() int {
	n := 0
	for _, r := range p.text {
		n++
		if r == '\n' {
			n = 0
		}
	}
	return n
}
func (p *plainArea) SetWidth(int) {}
func (p *plainArea) View() string { return p.text }
func (p *plainArea) CursorCell() (x, y int, ok bool) {
	return p.Column(), p.Line(), true
}
func (p *plainArea) Update(in Input) {
	switch in := in.(type) {
	case Paste:
		p.text += in.Content
	case Key:
		p.text += in.Text
	}
}
