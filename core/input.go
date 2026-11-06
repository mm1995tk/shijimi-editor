package core

// Input is what the user does, in a form that does not depend on where it came
// from. The TUI turns bubbletea messages into Inputs (see inputtea.go).
// Everything that handles input takes an Input; only the adapter in inputtea.go
// knows tea.
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
// knows them (the Kitty keyboard protocol).
type Key struct {
	Code        rune
	ShiftedCode rune
	BaseCode    rune
	Text        string
	Mod         Mod
	Repeat      bool
}

func (Key) input() {}

// Paste is text pasted in one piece.
type Paste struct {
	Content string
}

func (Paste) input() {}

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
// Region, Row and Col say where it is: which part of the screen, and the line
// and column inside that part (0-based, from the part's top-left corner).
// Clicks is how many presses in a row hit the same spot: the TUI counts in its
// adapter (App.clicks in inputtea.go). The core only reads the number. Zero means not counted (not a left press).
//
// There is no screen cell in a Mouse. Where a TUI popup or a border hangs from
// a screen cell, the TUI works the cell out from Region, Row and Col
// (App.screenCell). While a button is held, Region stays the one the press was in (see App.locate) and Row and Col may fall
// outside it. For RegionNone, which has no corner of its own, Row and Col count
// from the top-left of the screen.
type Mouse struct {
	Region           Region
	Row, Col         int
	Button           MouseButton
	Action           MouseAction
	Shift, Alt, Ctrl bool
	Clicks           int
}

func (Mouse) input() {}

// IsButton reports whether b is a real button, not a wheel step or nothing.
func (b MouseButton) IsButton() bool {
	return b == MouseLeft || b == MouseMiddle || b == MouseRight || b == MouseOther
}
