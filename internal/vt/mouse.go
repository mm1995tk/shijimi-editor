package vt

import "fmt"

// MouseButton is a mouse button or wheel direction.
type MouseButton int

const (
	ButtonNone MouseButton = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
	WheelUp
	WheelDown
	WheelLeft
	WheelRight
)

// MouseAction is what happened to the button.
type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMotion
)

// MouseEvent is a mouse event at a cell of the screen (0-based).
type MouseEvent struct {
	Button           MouseButton
	Action           MouseAction
	X, Y             int
	Shift, Alt, Ctrl bool
}

// EncodeMouse returns the bytes to send to the program for ev, or nil when the
// program did not ask for this kind of event.
func (t *Term) EncodeMouse(ev MouseEvent) []byte {
	m := t.Modes()
	return encodeMouse(m.Mouse, m.MouseSGR, ev)
}

func encodeMouse(mode MouseMode, sgr bool, ev MouseEvent) []byte {
	if mode == MouseOff || ev.X < 0 || ev.Y < 0 {
		return nil
	}
	wheel := ev.Button >= WheelUp
	var code int
	switch ev.Button {
	case ButtonLeft:
		code = 0
	case ButtonMiddle:
		code = 1
	case ButtonRight:
		code = 2
	case ButtonNone:
		code = 3
	default:
		code = 64 + int(ev.Button-WheelUp)
	}
	switch ev.Action {
	case MouseRelease:
		if wheel {
			return nil
		}
	case MouseMotion:
		if mode == MouseClick || (mode == MouseDrag && ev.Button == ButtonNone) {
			return nil
		}
		code += 32
	}
	if ev.Shift {
		code += 4
	}
	if ev.Alt {
		code += 8
	}
	if ev.Ctrl {
		code += 16
	}
	if sgr {
		final := 'M'
		if ev.Action == MouseRelease {
			final = 'm'
		}
		return []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", code, ev.X+1, ev.Y+1, final))
	}
	if ev.Action == MouseRelease {
		code = code&^3 | 3
	}
	if ev.X+33 > 255 || ev.Y+33 > 255 {
		return nil // out of range for the legacy encoding
	}
	return []byte{0x1b, '[', 'M', byte(32 + code), byte(33 + ev.X), byte(33 + ev.Y)}
}
