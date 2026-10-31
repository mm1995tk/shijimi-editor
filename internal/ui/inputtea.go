package ui

import (
	tea "charm.land/bubbletea/v2"
)

// This file is the only place bubbletea's input types are read or built (apart
// from Update, which hands them over). It turns tea.KeyPressMsg,
// tea.PasteMsg and tea.MouseMsg into Input, and Input back into a tea.Msg for
// the bubbles widgets that still take one (textinput, textarea; P4 removes
// them).

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
	in  Mod
	tea tea.KeyMod
}{
	{ModShift, tea.ModShift},
	{ModAlt, tea.ModAlt},
	{ModCtrl, tea.ModCtrl},
	{ModMeta, tea.ModMeta},
	{ModHyper, tea.ModHyper},
	{ModSuper, tea.ModSuper},
}

func modFromTea(m tea.KeyMod) (r Mod) {
	for _, p := range modPairs {
		if m.Contains(p.tea) {
			r |= p.in
		}
	}
	return r
}

func modToTea(m Mod) (r tea.KeyMod) {
	for _, p := range modPairs {
		if m.Has(p.in) {
			r |= p.tea
		}
	}
	return r
}

func keyFromTea(m tea.KeyPressMsg) Key {
	return Key{
		Code: m.Code, ShiftedCode: m.ShiftedCode, BaseCode: m.BaseCode,
		Text: m.Text, Mod: modFromTea(m.Mod), Repeat: m.IsRepeat,
	}
}

func keyToTea(k Key) tea.KeyPressMsg {
	return tea.KeyPressMsg{
		Code: k.Code, ShiftedCode: k.ShiftedCode, BaseCode: k.BaseCode,
		Text: k.Text, Mod: modToTea(k.Mod), IsRepeat: k.Repeat,
	}
}

// keyReleaseFromTea is the key of a release message. A release is a TUI-only
// notice (App.keyReleases), so it has no Input of its own.
func keyReleaseFromTea(m tea.KeyReleaseMsg) Key {
	return keyFromTea(tea.KeyPressMsg(m))
}

func pasteFromTea(m tea.PasteMsg) Paste { return Paste{Content: m.Content} }

// widgetMsg is the tea message a bubbles widget wants for an Input, or nil.
func widgetMsg(in Input) tea.Msg {
	switch in := in.(type) {
	case Key:
		return keyToTea(in)
	case Paste:
		return tea.PasteMsg{Content: in.Content}
	}
	return nil
}

var buttonFromTea = map[tea.MouseButton]MouseButton{
	tea.MouseNone:       MouseNone,
	tea.MouseLeft:       MouseLeft,
	tea.MouseMiddle:     MouseMiddle,
	tea.MouseRight:      MouseRight,
	tea.MouseWheelUp:    MouseWheelUp,
	tea.MouseWheelDown:  MouseWheelDown,
	tea.MouseWheelLeft:  MouseWheelLeft,
	tea.MouseWheelRight: MouseWheelRight,
}

// mouseFromTea builds the Mouse for a tea mouse message. The hit test (which
// region the cell is in, and where inside it) is the TUI's, so it happens here.
func (a *App) mouseFromTea(msg tea.MouseMsg) Mouse {
	m := msg.Mouse()
	e := Mouse{
		X: m.X, Y: m.Y, Button: MouseOther,
		Shift: m.Mod.Contains(tea.ModShift),
		Alt:   m.Mod.Contains(tea.ModAlt),
		Ctrl:  m.Mod.Contains(tea.ModCtrl),
	}
	if b, ok := buttonFromTea[m.Button]; ok {
		e.Button = b
	}
	switch msg.(type) {
	case tea.MouseReleaseMsg:
		e.Action = MouseRelease
	case tea.MouseMotionMsg:
		e.Action = MouseMotion
	}
	e.Region, e.Row, e.Col = a.hitTest(a.layout(), m.X, m.Y)
	return e
}

// hitTest says which part of the screen a cell is in, and where inside it.
func (a *App) hitTest(l layout, x, y int) (Region, int, int) {
	switch {
	case y < l.top || y >= l.top+l.bodyH:
		return RegionNone, 0, 0
	case a.showSide && x < l.sideW:
		return RegionExplorer, y - l.top, x
	case l.agentW > 0 && x >= l.agentX-1: // the border column belongs to the panel (it is dragged to resize)
		return RegionAgent, y - l.top, x - l.agentX
	case a.showTerm && y >= l.panelY:
		return RegionTerminal, y - l.panelY - 2, x - l.fullX // row 0 is the first body line, under the frame and title
	case l.inOtherPane(x, y):
		return RegionOther, y - l.top, x - l.otherX
	case x >= l.rightX && x < l.rightX+l.rightW:
		return RegionEditor, y - l.top, x - l.rightX // the title row is row 0
	}
	return RegionNone, 0, 0
}
