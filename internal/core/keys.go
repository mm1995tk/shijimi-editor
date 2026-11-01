package core

import (
	"strings"
	"unicode"
)

// Named keys. Their values and names are bubbletea's (ultraviolet's), so a Key
// from the TUI needs no translation; internal/tui/inputtea_test.go checks they
// match. The table is copied rather than imported so core does not depend on
// bubbletea.

// KeyExtended is a key whose Text is more than one rune.
const KeyExtended = unicode.MaxRune + 1

const (
	KeyUp rune = KeyExtended + iota + 1
	KeyDown
	KeyRight
	KeyLeft
	KeyBegin
	KeyFind
	KeyInsert
	KeyDelete
	KeySelect
	KeyPgUp
	KeyPgDown
	KeyHome
	KeyEnd
	KeyKpEnter
	KeyKpEqual
	KeyKpMultiply
	KeyKpPlus
	KeyKpComma
	KeyKpMinus
	KeyKpDecimal
	KeyKpDivide
	KeyKp0
	KeyKp1
	KeyKp2
	KeyKp3
	KeyKp4
	KeyKp5
	KeyKp6
	KeyKp7
	KeyKp8
	KeyKp9
	KeyKpSep
	KeyKpUp
	KeyKpDown
	KeyKpLeft
	KeyKpRight
	KeyKpPgUp
	KeyKpPgDown
	KeyKpHome
	KeyKpEnd
	KeyKpInsert
	KeyKpDelete
	KeyKpBegin
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24
	KeyF25
	KeyF26
	KeyF27
	KeyF28
	KeyF29
	KeyF30
	KeyF31
	KeyF32
	KeyF33
	KeyF34
	KeyF35
	KeyF36
	KeyF37
	KeyF38
	KeyF39
	KeyF40
	KeyF41
	KeyF42
	KeyF43
	KeyF44
	KeyF45
	KeyF46
	KeyF47
	KeyF48
	KeyF49
	KeyF50
	KeyF51
	KeyF52
	KeyF53
	KeyF54
	KeyF55
	KeyF56
	KeyF57
	KeyF58
	KeyF59
	KeyF60
	KeyF61
	KeyF62
	KeyF63
	KeyCapsLock
	KeyScrollLock
	KeyNumLock
	KeyPrintScreen
	KeyPause
	KeyMenu
	KeyMediaPlay
	KeyMediaPause
	KeyMediaPlayPause
	KeyMediaReverse
	KeyMediaStop
	KeyMediaFastForward
	KeyMediaRewind
	KeyMediaNext
	KeyMediaPrev
	KeyMediaRecord
	KeyLowerVol
	KeyRaiseVol
	KeyMute
	KeyLeftShift
	KeyLeftAlt
	KeyLeftCtrl
	KeyLeftSuper
	KeyLeftHyper
	KeyLeftMeta
	KeyRightShift
	KeyRightAlt
	KeyRightCtrl
	KeyRightSuper
	KeyRightHyper
	KeyRightMeta
	KeyIsoLevel3Shift
	KeyIsoLevel5Shift

	KeyBackspace = rune(0x7f)
	KeyTab       = rune(0x09)
	KeyEnter     = rune(0x0d)
	KeyReturn    = KeyEnter
	KeyEscape    = rune(0x1b)
	KeyEsc       = KeyEscape
	KeySpace     = rune(0x20)
)

var keyNames = map[rune]string{
	KeyEnter:      "enter",
	KeyTab:        "tab",
	KeyBackspace:  "backspace",
	KeyEscape:     "esc",
	KeySpace:      "space",
	KeyUp:         "up",
	KeyDown:       "down",
	KeyLeft:       "left",
	KeyRight:      "right",
	KeyBegin:      "begin",
	KeyFind:       "find",
	KeyInsert:     "insert",
	KeyDelete:     "delete",
	KeySelect:     "select",
	KeyPgUp:       "pgup",
	KeyPgDown:     "pgdown",
	KeyHome:       "home",
	KeyEnd:        "end",
	KeyKpEnter:    "enter",
	KeyKpEqual:    "equal",
	KeyKpMultiply: "mul",
	KeyKpPlus:     "plus",
	KeyKpComma:    "comma",
	KeyKpMinus:    "minus",
	KeyKpDecimal:  "period",
	KeyKpDivide:   "div",
	KeyKp0:        "0",
	KeyKp1:        "1",
	KeyKp2:        "2",
	KeyKp3:        "3",
	KeyKp4:        "4",
	KeyKp5:        "5",
	KeyKp6:        "6",
	KeyKp7:        "7",
	KeyKp8:        "8",
	KeyKp9:        "9",

	// Kitty keyboard extension
	KeyKpSep:    "sep",
	KeyKpUp:     "up",
	KeyKpDown:   "down",
	KeyKpLeft:   "left",
	KeyKpRight:  "right",
	KeyKpPgUp:   "pgup",
	KeyKpPgDown: "pgdown",
	KeyKpHome:   "home",
	KeyKpEnd:    "end",
	KeyKpInsert: "insert",
	KeyKpDelete: "delete",
	KeyKpBegin:  "begin",

	KeyF1:  "f1",
	KeyF2:  "f2",
	KeyF3:  "f3",
	KeyF4:  "f4",
	KeyF5:  "f5",
	KeyF6:  "f6",
	KeyF7:  "f7",
	KeyF8:  "f8",
	KeyF9:  "f9",
	KeyF10: "f10",
	KeyF11: "f11",
	KeyF12: "f12",
	KeyF13: "f13",
	KeyF14: "f14",
	KeyF15: "f15",
	KeyF16: "f16",
	KeyF17: "f17",
	KeyF18: "f18",
	KeyF19: "f19",
	KeyF20: "f20",
	KeyF21: "f21",
	KeyF22: "f22",
	KeyF23: "f23",
	KeyF24: "f24",
	KeyF25: "f25",
	KeyF26: "f26",
	KeyF27: "f27",
	KeyF28: "f28",
	KeyF29: "f29",
	KeyF30: "f30",
	KeyF31: "f31",
	KeyF32: "f32",
	KeyF33: "f33",
	KeyF34: "f34",
	KeyF35: "f35",
	KeyF36: "f36",
	KeyF37: "f37",
	KeyF38: "f38",
	KeyF39: "f39",
	KeyF40: "f40",
	KeyF41: "f41",
	KeyF42: "f42",
	KeyF43: "f43",
	KeyF44: "f44",
	KeyF45: "f45",
	KeyF46: "f46",
	KeyF47: "f47",
	KeyF48: "f48",
	KeyF49: "f49",
	KeyF50: "f50",
	KeyF51: "f51",
	KeyF52: "f52",
	KeyF53: "f53",
	KeyF54: "f54",
	KeyF55: "f55",
	KeyF56: "f56",
	KeyF57: "f57",
	KeyF58: "f58",
	KeyF59: "f59",
	KeyF60: "f60",
	KeyF61: "f61",
	KeyF62: "f62",
	KeyF63: "f63",

	// Kitty keyboard extension
	KeyCapsLock:         "capslock",
	KeyScrollLock:       "scrolllock",
	KeyNumLock:          "numlock",
	KeyPrintScreen:      "printscreen",
	KeyPause:            "pause",
	KeyMenu:             "menu",
	KeyMediaPlay:        "mediaplay",
	KeyMediaPause:       "mediapause",
	KeyMediaPlayPause:   "mediaplaypause",
	KeyMediaReverse:     "mediareverse",
	KeyMediaStop:        "mediastop",
	KeyMediaFastForward: "mediafastforward",
	KeyMediaRewind:      "mediarewind",
	KeyMediaNext:        "medianext",
	KeyMediaPrev:        "mediaprev",
	KeyMediaRecord:      "mediarecord",
	KeyLowerVol:         "lowervol",
	KeyRaiseVol:         "raisevol",
	KeyMute:             "mute",
	KeyLeftShift:        "leftshift",
	KeyLeftAlt:          "leftalt",
	KeyLeftCtrl:         "leftctrl",
	KeyLeftSuper:        "leftsuper",
	KeyLeftHyper:        "lefthyper",
	KeyLeftMeta:         "leftmeta",
	KeyRightShift:       "rightshift",
	KeyRightAlt:         "rightalt",
	KeyRightCtrl:        "rightctrl",
	KeyRightSuper:       "rightsuper",
	KeyRightHyper:       "righthyper",
	KeyRightMeta:        "rightmeta",
	KeyIsoLevel3Shift:   "isolevel3shift",
	KeyIsoLevel5Shift:   "isolevel5shift",
}

// String is the key in the form the keymaps use: "ctrl+s", "enter", "a", "?".
func (k Key) String() string {
	if len(k.Text) > 0 && k.Text != " " {
		return k.Text
	}
	return k.keystroke()
}

// keystroke names the key by its code, with the modifiers in the order ctrl,
// alt, shift, meta, hyper, super.
func (k Key) keystroke() string {
	var sb strings.Builder
	for _, m := range [...]struct {
		mod         Mod
		name        string
		left, right rune
	}{
		{ModCtrl, "ctrl+", KeyLeftCtrl, KeyRightCtrl},
		{ModAlt, "alt+", KeyLeftAlt, KeyRightAlt},
		{ModShift, "shift+", KeyLeftShift, KeyRightShift},
		{ModMeta, "meta+", KeyLeftMeta, KeyRightMeta},
		{ModHyper, "hyper+", KeyLeftHyper, KeyRightHyper},
		{ModSuper, "super+", KeyLeftSuper, KeyRightSuper},
	} {
		if k.Mod.Has(m.mod) && k.Code != m.left && k.Code != m.right {
			sb.WriteString(m.name)
		}
	}
	if name, ok := keyNames[k.Code]; ok {
		sb.WriteString(name)
		return sb.String()
	}
	code := k.Code
	if k.BaseCode != 0 {
		code = k.BaseCode // the key on a standard PC-101 layout
	}
	switch code {
	case KeySpace:
		sb.WriteString("space")
	case KeyExtended:
		sb.WriteString(k.Text)
	default:
		sb.WriteRune(code)
	}
	return sb.String()
}
