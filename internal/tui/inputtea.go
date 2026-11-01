package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// This file is the only place bubbletea's input types are read or built. fromTea
// turns the messages Update gets (tea.KeyPressMsg, tea.PasteMsg, tea.MouseMsg,
// tea.KeyReleaseMsg, tea.KeyboardEnhancementsMsg, tea.WindowSizeMsg) into Input
// or the Resized, KeyReleased and ReleasesReported notices. It also turns an Input back into a tea.Msg for
// the bubbles widget that still takes one (the review composer's textarea; P10h
// removes it).

// Named keys. Their values are bubbletea's, so a Key from the TUI needs no
// translation; inputtea_test.go checks they match.
const (
	KeyExtended  = rune(tea.KeyExtended)
	KeyUp        = rune(tea.KeyUp)
	KeyDown      = rune(tea.KeyDown)
	KeyRight     = rune(tea.KeyRight)
	KeyLeft      = rune(tea.KeyLeft)
	KeyHome      = rune(tea.KeyHome)
	KeyEnd       = rune(tea.KeyEnd)
	KeyPgUp      = rune(tea.KeyPgUp)
	KeyPgDown    = rune(tea.KeyPgDown)
	KeyInsert    = rune(tea.KeyInsert)
	KeyDelete    = rune(tea.KeyDelete)
	KeyF1        = rune(tea.KeyF1)
	KeyF2        = rune(tea.KeyF2)
	KeyF3        = rune(tea.KeyF3)
	KeyF4        = rune(tea.KeyF4)
	KeyF5        = rune(tea.KeyF5)
	KeyF6        = rune(tea.KeyF6)
	KeyF7        = rune(tea.KeyF7)
	KeyF8        = rune(tea.KeyF8)
	KeyF9        = rune(tea.KeyF9)
	KeyF10       = rune(tea.KeyF10)
	KeyF11       = rune(tea.KeyF11)
	KeyF12       = rune(tea.KeyF12)
	KeyBackspace = rune(tea.KeyBackspace)
	KeyTab       = rune(tea.KeyTab)
	KeyEnter     = rune(tea.KeyEnter)
	KeyEscape    = rune(tea.KeyEscape)
	KeyEsc       = rune(tea.KeyEsc)
	KeySpace     = rune(tea.KeySpace)
)

var modPairs = [...]struct {
	in  core.Mod
	tea tea.KeyMod
}{
	{core.ModShift, tea.ModShift},
	{core.ModAlt, tea.ModAlt},
	{core.ModCtrl, tea.ModCtrl},
	{core.ModMeta, tea.ModMeta},
	{core.ModHyper, tea.ModHyper},
	{core.ModSuper, tea.ModSuper},
}

func modFromTea(m tea.KeyMod) (r core.Mod) {
	for _, p := range modPairs {
		if m.Contains(p.tea) {
			r |= p.in
		}
	}
	return r
}

func modToTea(m core.Mod) (r tea.KeyMod) {
	for _, p := range modPairs {
		if m.Has(p.in) {
			r |= p.tea
		}
	}
	return r
}

func keyFromTea(m tea.KeyPressMsg) core.Key {
	return core.Key{
		Code: m.Code, ShiftedCode: m.ShiftedCode, BaseCode: m.BaseCode,
		Text: m.Text, Mod: modFromTea(m.Mod), Repeat: m.IsRepeat,
	}
}

func keyToTea(k core.Key) tea.KeyPressMsg {
	return tea.KeyPressMsg{
		Code: k.Code, ShiftedCode: k.ShiftedCode, BaseCode: k.BaseCode,
		Text: k.Text, Mod: modToTea(k.Mod), IsRepeat: k.Repeat,
	}
}

// keyReleaseFromTea is the key of a release message. A release is a TUI-only
// notice (App.keyReleases), so it has no Input of its own.
func keyReleaseFromTea(m tea.KeyReleaseMsg) core.Key {
	return keyFromTea(tea.KeyPressMsg(m))
}

func pasteFromTea(m tea.PasteMsg) core.Paste { return core.Paste{Content: m.Content} }

// widgetMsg is the tea message a bubbles widget wants for an Input, or nil.
func widgetMsg(in core.Input) tea.Msg {
	switch in := in.(type) {
	case core.Key:
		return keyToTea(in)
	case core.Paste:
		return tea.PasteMsg{Content: in.Content}
	}
	return nil
}

var buttonFromTea = map[tea.MouseButton]core.MouseButton{
	tea.MouseNone:       core.MouseNone,
	tea.MouseLeft:       core.MouseLeft,
	tea.MouseMiddle:     core.MouseMiddle,
	tea.MouseRight:      core.MouseRight,
	tea.MouseWheelUp:    core.MouseWheelUp,
	tea.MouseWheelDown:  core.MouseWheelDown,
	tea.MouseWheelLeft:  core.MouseWheelLeft,
	tea.MouseWheelRight: core.MouseWheelRight,
}

// mouseFromTea builds the Mouse for a tea mouse message. The hit test (which
// region the cell is in, and where inside it) is the TUI's, so it happens here.
func (a *App) mouseFromTea(msg tea.MouseMsg) core.Mouse {
	m := msg.Mouse()
	e := core.Mouse{
		Button: core.MouseOther,
		Shift:  m.Mod.Contains(tea.ModShift),
		Alt:    m.Mod.Contains(tea.ModAlt),
		Ctrl:   m.Mod.Contains(tea.ModCtrl),
	}
	if b, ok := buttonFromTea[m.Button]; ok {
		e.Button = b
	}
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		e.Action = core.MouseRelease
	case tea.MouseMotionMsg:
		e.Action = core.MouseMotion
	}
	if e.Action == core.MousePress && e.Button == core.MouseLeft {
		e.Clicks = a.clicks(m.X, m.Y)
	}
	return a.locate(m.X, m.Y, e)
}

// locate fills in Region, Row and Col from the screen cell x, y. It keeps
// state: a press starts the capture and a release ends it.
//
// While a button is held, the pointer is captured by the region the press was
// in (as a browser does with setPointerCapture): the drag and the release keep
// that Region, and Row and Col go on counting from its corner, so they may be
// negative or past its edge. A GUI sends its events the same way.
func (a *App) locate(x, y int, m core.Mouse) core.Mouse {
	l := a.layout()
	button := m.Button.IsButton()
	switch {
	case m.Action == core.MousePress && button:
		// capture starts
		m.Region, m.Row, m.Col = a.hitTest(l, x, y)
		a.capture = m.Region
	case m.Action != core.MousePress && a.capture != core.RegionNone && (m.Action == core.MouseRelease || m.Button != core.MouseNone || a.pointerHeld()):
		x0, y0 := a.regionOrigin(l, a.capture)
		m.Region, m.Row, m.Col = a.capture, y-y0, x-x0
		if m.Action == core.MouseRelease { // capture ends
			a.capture = core.RegionNone
		}
	default:
		m.Region, m.Row, m.Col = a.hitTest(l, x, y)
	}
	return m
}

// pointerHeld reports whether something is being dragged, for the terminals
// that report a drag as motion with no button.
func (a *App) pointerHeld() bool {
	return a.resizing != core.ResizeNone || a.IsDragging() || a.term.sel.dragging || a.term.grab || a.AgentDragging()
}

// regionOrigin is the screen cell that is row 0, column 0 of a region.
func (a *App) regionOrigin(l layout, r core.Region) (x, y int) {
	switch r {
	case core.RegionExplorer:
		return 0, l.top
	case core.RegionAgent:
		return l.agentX, l.top
	case core.RegionTerminal:
		return l.fullX, l.panelY + 2
	case core.RegionOther:
		return l.otherX, l.top
	case core.RegionEditor:
		return l.rightX, l.top
	}
	return 0, 0
}

// hitTest says which part of the screen a cell is in, and where inside it.
func (a *App) hitTest(l layout, x, y int) (core.Region, int, int) {
	switch {
	case y < l.top || y >= l.top+l.bodyH:
		return core.RegionNone, y, x
	case a.showSide && x < l.sideW:
		return core.RegionExplorer, y - l.top, x
	case l.agentW > 0 && x >= l.agentX-1: // the border column belongs to the panel (it is dragged to resize)
		return core.RegionAgent, y - l.top, x - l.agentX
	case a.showTerm && y >= l.panelY:
		return core.RegionTerminal, y - l.panelY - 2, x - l.fullX // row 0 is the first body line, under the frame and title
	case l.inOtherPane(x, y):
		return core.RegionOther, y - l.top, x - l.otherX
	case x >= l.rightX && x < l.rightX+l.rightW:
		return core.RegionEditor, y - l.top, x - l.rightX // the title row is row 0
	}
	return core.RegionNone, y, x
}

// fromTea is the one place Update's tea.Msg becomes an Input or an Event. What
// is not a bubbletea input message (the Events and the terminal panel's own
// messages) passes through as it is.
func (a *App) fromTea(msg tea.Msg) any {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return core.Resized{W: msg.Width, H: msg.Height}
	case tea.KeyboardEnhancementsMsg:
		return core.ReleasesReported{On: msg.SupportsEventTypes()}
	case tea.KeyReleaseMsg:
		return core.KeyReleased{Key: keyReleaseFromTea(msg)}
	case tea.MouseMsg:
		return a.mouseFromTea(msg)
	case tea.KeyPressMsg:
		return keyFromTea(msg)
	case tea.PasteMsg:
		return pasteFromTea(msg)
	}
	return msg
}

// click is a left press, kept to tell a double click from two single ones.
type click struct {
	x, y int
	at   time.Time
}

const doubleClickTime = 400 * time.Millisecond

// clicks records a left press at (x, y) and returns how many presses in a row
// it makes: 2 for the second press of a double click, else 1. A third press
// starts over. A GUI counts for itself and sends Mouse.Clicks.
func (a *App) clicks(x, y int) int {
	now := time.Now()
	p := a.lastClick
	double := !p.at.IsZero() && p.x == x && p.y == y && now.Sub(p.at) < doubleClickTime
	a.lastClick = click{x: x, y: y, at: now}
	if double {
		a.lastClick.at = time.Time{}
		return 2
	}
	return 1
}

// screenCell is the cell on the whole screen that a Mouse is over: the inverse
// of locate. Popups and borders hang from screen cells, which only the TUI has.
func (a *App) screenCell(l layout, m core.Mouse) (x, y int) {
	x0, y0 := a.regionOrigin(l, m.Region)
	return x0 + m.Col, y0 + m.Row
}
