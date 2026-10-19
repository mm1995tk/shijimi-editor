package ui

import tea "charm.land/bubbletea/v2"

// mouseEvent is a mouse message flattened into one struct, so handlers can
// switch on the action and button together.
type mouseEvent struct {
	X, Y             int
	Button           tea.MouseButton
	Action           mouseAction
	Shift, Alt, Ctrl bool
}

type mouseAction int

const (
	mousePress mouseAction = iota // a button press or a wheel step
	mouseRelease
	mouseMotion
)

func toMouseEvent(msg tea.MouseMsg) mouseEvent {
	m := msg.Mouse()
	e := mouseEvent{
		X: m.X, Y: m.Y, Button: m.Button,
		Shift: m.Mod.Contains(tea.ModShift),
		Alt:   m.Mod.Contains(tea.ModAlt),
		Ctrl:  m.Mod.Contains(tea.ModCtrl),
	}
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		e.Action = mouseRelease
	case tea.MouseMotionMsg:
		e.Action = mouseMotion
	}
	return e
}
