package core

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

// headless is the front end of a core nothing draws yet: a workspace of `shj
// serve` before a GUI looks at it. It has only the editor on its screen, no
// terminal panel and no multiplexer; until the GUI sends a size the screen is
// 80 x 24 cells (newHeadlessCore). The GUI will tell the
// sizes of its regions (view.resize) and pick the theme (theme.get); until
// then the core keeps its state as it would behind any screen.
type headless struct {
	// core is the core it fronts (newHeadlessCore). The pointer is worked out
	// in the core, from the panes the GUI was sent (paneMouse).
	core *Core
}

// newHeadless is a headless front end.
func newHeadless() *headless { return &headless{} }

// newHeadlessCore is a core rooted at dir on a headless front end.
func newHeadlessCore(sh *Shared, dir string) (*Core, error) {
	h := newHeadless()
	c, err := NewIn(sh, h, dir, "")
	if err != nil {
		return nil, err
	}
	h.core = c
	c.SetSizes(Sizes{ScreenW: 80, ScreenH: 24, EditRows: 23, EditorCols: 80, TextCols: 80})
	return c, nil
}

func (h *headless) ReleasesReported() bool   { return false }
func (h *headless) SetReleasesReported(bool) {}
func (h *headless) ScrollHover(int)          {}
func (h *headless) HoverCellWord() (buffer.Pos, WordSpan, bool) {
	return buffer.Pos{}, WordSpan{}, false
}

func (h *headless) SetPalette(*Theme) {} // the theme is the process's (ActivateTheme); the GUI asks for it with theme.get
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
