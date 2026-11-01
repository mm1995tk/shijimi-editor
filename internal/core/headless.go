package core

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

// Headless is the front end of a core nothing draws yet: a workspace of `shj
// serve` before a GUI looks at it. It has a screen of W x H cells with only the
// editor on it, no terminal panel and no multiplexer. The GUI will tell the
// sizes of its regions (view.resize) and pick the theme (theme.get); until
// then the core keeps its state as it would behind any screen.
type Headless struct {
	W, H int
}

// NewHeadless is a Headless of 80 x 24 cells.
func NewHeadless() *Headless { return &Headless{W: 80, H: 24} }

func (h *Headless) ScreenW() int                  { return h.W }
func (h *Headless) ScreenH() int                  { return h.H }
func (h *Headless) SideShown() bool               { return false }
func (h *Headless) TermShown() bool               { return false }
func (h *Headless) AgentOpen() bool               { return false }
func (h *Headless) SetSide(bool)                  {}
func (h *Headless) SetTerm(bool)                  {}
func (h *Headless) SetAgent(bool)                 {}
func (h *Headless) AgentWidthPref() int           { return 0 }
func (h *Headless) ExplorerShare() int            { return 0 }
func (h *Headless) InResizeMode() bool            { return false }
func (h *Headless) DragBorder(ResizeTarget)       {}
func (h *Headless) ReleasesReported() bool        { return false }
func (h *Headless) SetReleasesReported(bool)      {}
func (h *Headless) SendClipboard(string)          {}
func (h *Headless) TermBlur()                     {}
func (h *Headless) TermFocus() []Effect           { return nil }
func (h *Headless) TermCloseAll()                 {}
func (h *Headless) TermKey(Key)                   {}
func (h *Headless) TermRun(string) []Effect       { return nil }
func (h *Headless) TermPaste(string) []Effect     { return nil }
func (h *Headless) TermShell() Shell              { return nil }
func (h *Headless) MuxReset()                     {}
func (h *Headless) MuxOn() bool                   { return false }
func (h *Headless) MuxOpen() error                { return nil }
func (h *Headless) MuxHide() error                { return nil }
func (h *Headless) MuxToggle() []Effect           { return nil }
func (h *Headless) MuxSend(string, bool) []Effect { return nil }
func (h *Headless) MuxClose()                     {}
func (h *Headless) MuxResult(error) []Effect      { return nil }
func (h *Headless) CommandEnded(int)              {}
func (h *Headless) StartResize() []Effect         { return nil }
func (h *Headless) HandleResizeKey(string) []Effect {
	return nil
}
func (h *Headless) ScrollHover(int) {}
func (h *Headless) HoverCellWord() (buffer.Pos, WordSpan, bool) {
	return buffer.Pos{}, WordSpan{}, false
}

// EditRows leaves out the status line.
func (h *Headless) EditRows() int                  { return max(0, h.H-1) }
func (h *Headless) TextCols(*Tab) int              { return h.W }
func (h *Headless) DiffBaseCols() int              { return 0 }
func (h *Headless) SuggestionsShown() bool         { return false }
func (h *Headless) EditorCols() int                { return h.W }
func (h *Headless) ResetPanes()                    {}
func (h *Headless) SetScreen(w, ht int)            { h.W, h.H = w, ht }
func (h *Headless) SetPalette(*Theme)              {} // the theme is the process's (ActivateTheme); the GUI asks for it with theme.get
func (h *Headless) HandleTermEvent(Event) []Effect { return nil }
func (h *Headless) HandleMouse(Mouse) []Effect     { return nil }
func (h *Headless) NewTextArea(initial string) TextArea {
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
