package ui

// Input is what the user does, in a form that does not depend on where it came
// from. The TUI turns bubbletea messages into Inputs (see inputtea.go); the GUI
// protocol's input.key, input.mouse, input.paste and input.composition map onto
// the same four types (docs/design/gui-protocol.md, chapter 3). Everything that
// handles input takes an Input; only the adapter in inputtea.go knows tea.
type Input interface{ input() }

// Mod is a set of modifier keys.
type Mod uint8

const (
	ModShift Mod = 1 << iota
	ModAlt
	ModCtrl
	ModMeta
	ModHyper
	ModSuper
)

// Has reports whether every modifier in m2 is held.
func (m Mod) Has(m2 Mod) bool { return m&m2 == m2 }

// Key is a key press.
//
// Code is the key: a printable character, or one of the Key* constants for a
// named key. Text is what typing the key produces ("" for named keys and
// Ctrl/Alt combinations). ShiftedCode and BaseCode are filled where the source
// knows them (the Kitty keyboard protocol on a TUI; W3C KeyboardEvent.key and
// .code on a GUI).
type Key struct {
	Code        rune
	ShiftedCode rune
	BaseCode    rune
	Text        string
	Mod         Mod
	Repeat      bool
	// Seq numbers the inputs a GUI sends, so a screen can say how far it has
	// caught up (Screen.AckSeq). The TUI leaves it 0.
	Seq uint64
}

// String is the key in the form the keymaps use: "ctrl+s", "enter", "a", "?".
func (k Key) String() string { return keyToTea(k).String() }

func (Key) input() {}

// Paste is text pasted in one piece.
type Paste struct {
	Content string
	Seq     uint64
}

func (Paste) input() {}

// Composition is text an input method (IME) is still composing, or has just
// committed. The TUI never produces one: a terminal hands over only the
// committed text, as a Key. It is here so the GUI's input.composition has a
// place to go.
type Composition struct {
	Text   string
	Cursor int // where the caret sits inside Text, in runes
	Phase  CompositionPhase
	Seq    uint64
}

func (Composition) input() {}

// CompositionPhase is how far a composition has come: CompositionStart, then
// any number of CompositionUpdate, then CompositionEnd. At the end Text is the
// committed text (or empty if the composition was cancelled).
type CompositionPhase uint8

const (
	CompositionStart  CompositionPhase = iota // the first composing text
	CompositionUpdate                         // the composing text changed
	CompositionEnd                            // committed or cancelled
)

// MouseButton is the button of a Mouse input, or a wheel step.
type MouseButton uint8

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
	MouseWheelLeft
	MouseWheelRight
	MouseOther
)

// MouseAction is what the pointer did.
type MouseAction int

const (
	MousePress MouseAction = iota // a button press or a wheel step
	MouseRelease
	MouseMotion
)

// Region is the part of the screen a pointer is over.
type Region uint8

const (
	RegionNone     Region = iota // the diff header, the help line, the status line, a border
	RegionEditor                 // the focused editor pane
	RegionOther                  // the other editor pane while split
	RegionExplorer               // the file tree, and the changes list under it
	RegionTerminal
	RegionAgent
)

// Mouse is a pointer event.
//
// Region, Row and Col say where it is as a GUI would: which part of the screen,
// and the line and column inside that part (0-based, from the part's top-left
// corner). Clicks is how many presses in a row hit the same spot: a GUI counts
// them and sends the number; the TUI leaves it 0 and keeps its own double
// click timer (App.lastClick).
//
// X and Y are the cell on the whole screen. Only the TUI has them; the
// handlers still hit-test on them, not yet on Region/Row/Col.
type Mouse struct {
	Region           Region
	Row, Col         int
	X, Y             int
	Button           MouseButton
	Action           MouseAction
	Shift, Alt, Ctrl bool
	Clicks           int
	Seq              uint64
}

func (Mouse) input() {}
