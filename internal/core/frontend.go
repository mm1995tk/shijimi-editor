package core

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

// FrontEnd is what Core asks of the front end that draws it: which regions
// are shown, the multiplexer pane, the borders the user drags, and the few questions that need the
// layout. Core holds the front end it is drawn by (set in New); in the program
// it is internal/tui.App.
type FrontEnd interface {
	ReleasesReported() bool
	SetReleasesReported(on bool)
	MuxReset()
	MuxOn() bool
	MuxOpen() error
	MuxHide() error
	MuxToggle() []Effect
	MuxSend(text string, enter bool) []Effect
	MuxClose()
	MuxResult(err error) []Effect
	ScrollHover(d int)
	HoverCellWord() (buffer.Pos, WordSpan, bool)
	SetPalette(t *Theme)
	HandleMouse(msg Mouse) []Effect
	// NewTextArea is a focused multi-line input holding initial.
	NewTextArea(initial string) TextArea
}

// MinSideSectionH is the least height of a section of the diff mode sidebar:
// a header and two rows.
const MinSideSectionH = 3

// MuxFailTab is the termFailure.tab of failures seen in the multiplexer pane.
const MuxFailTab = -1

// DiffGeom is where the parts of a diff view sit on screen, in cells.
type DiffGeom struct {
	Top, H         int // first content row and number of content rows
	LeftW          int // width of the base pane (0 when unified)
	RightX, RightW int // the editable pane
	BarX           int // the change bar at the right edge
}

// TextArea is the multi-line input of the review comment editor. The front end
// makes it (FrontEnd.NewTextArea): it knows how the text is edited, wrapped
// and shown, which core does not.
type TextArea interface {
	Value() string
	SetValue(s string)
	InsertString(s string)
	// Line and Column are the cursor's row and column in the text.
	Line() int
	Column() int
	// Update takes a Key or a Paste.
	Update(in Input)
	SetWidth(w int)
	View() string
	// CursorCell is the cursor's cell inside the drawn View.
	CursorCell() (x, y int, ok bool)
}
