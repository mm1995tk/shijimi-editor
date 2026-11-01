package core

import "github.com/mm1995tk/shijimi-editor/internal/buffer"

// FrontEnd is everything the state that goes to core asks of the front end
// (a terminal): the calls above, the multiplexer pane, the borders the user
// drags and the few questions that need the screen's layout. Core holds the
// front end it is drawn by; App is the one there is (set in New).
type FrontEnd interface {
	ScreenW() int
	ScreenH() int
	SideShown() bool
	TermShown() bool
	AgentOpen() bool
	SetSide(on bool)
	SetTerm(on bool)
	SetAgent(on bool)
	AgentWidthPref() int
	ExplorerShare() int
	InResizeMode() bool
	DragBorder(r ResizeTarget)
	ReleasesReported() bool
	SetReleasesReported(on bool)
	SendClipboard(s string)
	TermBlur()
	TermFocus() []Effect
	TermCloseAll()
	TermKey(k Key)
	TermRun(line string) []Effect
	TermPaste(s string) []Effect
	TermShell() Shell
	MuxReset()
	MuxOn() bool
	MuxOpen() error
	MuxHide() error
	MuxToggle() []Effect
	MuxSend(text string, enter bool) []Effect
	MuxClose()
	MuxResult(err error) []Effect
	CommandEnded(code int)
	StartResize() []Effect
	HandleResizeKey(key string) []Effect
	ScrollHover(d int)
	HoverCellWord() (buffer.Pos, WordSpan, bool)
	EditRows() int
	TextCols(t *Tab) int
	DiffBaseCols() int
	SuggestionsShown() bool
	EditorCols() int
	ResetPanes()
	SetScreen(w, h int)
	SetPalette(t *Theme)
	// HandleTermEvent takes the terminal panel's TermOutputMsg and TermDoneMsg.
	HandleTermEvent(ev Event) []Effect
	HandleMouse(msg Mouse) []Effect
	// NewTextArea is a focused multi-line input holding initial.
	NewTextArea(initial string) TextArea
}

// ResizeTarget is a border the user drags or moves with the keys.
type ResizeTarget int

const (
	ResizeNone ResizeTarget = iota
	ResizeSide
	ResizeTerm
	ResizePreview
	ResizeAgent
	ResizeDiff
	ResizeExplorer
)

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
