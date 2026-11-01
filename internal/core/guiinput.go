package core

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// guiinput.go is what the GUI of `shj serve` sends, turned into the core's
// Inputs (docs/design/gui-protocol.md, 3 章「キー・マウス・貼り付けの入力」):
// W3C key names into Keys, a pointer on a pane into the cursor, the text an
// input method is composing into an underlined run in the pane, and the
// file.open and command.run requests. A headless front end has no layout of
// its own, so the pointer is worked out here from the panes BuildScreen makes.

// w3cKeys is the named keys of W3C KeyboardEvent.key that the core knows.
var w3cKeys = map[string]rune{
	"Enter":       KeyEnter,
	"Tab":         KeyTab,
	"Backspace":   KeyBackspace,
	"Escape":      KeyEscape,
	"Delete":      KeyDelete,
	"Insert":      KeyInsert,
	"Home":        KeyHome,
	"End":         KeyEnd,
	"PageUp":      KeyPgUp,
	"PageDown":    KeyPgDown,
	"ArrowUp":     KeyUp,
	"ArrowDown":   KeyDown,
	"ArrowLeft":   KeyLeft,
	"ArrowRight":  KeyRight,
	"CapsLock":    KeyCapsLock,
	"ScrollLock":  KeyScrollLock,
	"NumLock":     KeyNumLock,
	"PrintScreen": KeyPrintScreen,
	"Pause":       KeyPause,
	"ContextMenu": KeyMenu,
}

// ModOf is the modifiers a GUI names ("ctrl", "shift", "alt", "super", and
// "meta"). Names it does not know are left out.
func ModOf(names []string) Mod {
	var m Mod
	for _, n := range names {
		switch n {
		case "ctrl":
			m |= ModCtrl
		case "shift":
			m |= ModShift
		case "alt":
			m |= ModAlt
		case "super":
			m |= ModSuper
		case "meta":
			m |= ModMeta
		}
	}
	return m
}

// KeyFromW3C is the Key of a GUI's input.key: key is a W3C KeyboardEvent.key
// ("a", "A", "Enter", "ArrowLeft", "F5", " "), text what typing it produces.
// ok is false for a key that is no input of its own: a modifier pressed
// alone, a dead key, or a name the core does not know with no text.
//
// It makes the Key the TUI would get for the same press: a letter typed with
// Shift has the lower case Code and the upper case ShiftedCode and Text, and a
// key held with Ctrl, Alt, Super or Meta types nothing ("ctrl+a").
func KeyFromW3C(key, text string, mod Mod) (k Key, ok bool) {
	k.Mod = mod
	types := !mod.Has(ModCtrl) && !mod.Has(ModAlt) && !mod.Has(ModSuper) && !mod.Has(ModMeta)
	if code, ok := w3cKeys[key]; ok {
		k.Code = code
		return k, true
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(key, "F")); err == nil && strings.HasPrefix(key, "F") && n >= 1 && n <= 63 {
		k.Code = KeyF1 + rune(n-1)
		return k, true
	}
	if utf8.RuneCountInString(key) == 1 {
		r, _ := utf8.DecodeRuneInString(key)
		k.Code = r
		if mod.Has(ModShift) && unicode.IsUpper(r) {
			k.Code, k.ShiftedCode = unicode.ToLower(r), r
		}
		if types {
			k.Text = text
			if k.Text == "" {
				k.Text = key
			}
		}
		return k, true
	}
	// "Process", "Unidentified" and the like: an input method committing text.
	if text == "" || !types {
		return Key{}, false
	}
	k.Text = text
	k.Code, _ = utf8.DecodeRuneInString(text)
	if utf8.RuneCountInString(text) > 1 {
		k.Code = KeyExtended
	}
	return k, true
}

// splicePreedit puts text, underlined, into the line of p at pos: where an
// input method's composition shows until it is committed or cancelled.
func splicePreedit(p *screen.Pane, pos screen.Pos, text string) {
	for i := range p.Lines {
		ln := &p.Lines[i]
		if ln.No != pos.Line || ln.Blank {
			continue
		}
		mark := screen.Span{Text: text, Underline: true}
		var out []screen.Span
		col, done := 0, false
		for _, s := range ln.Spans {
			n := utf8.RuneCountInString(s.Text)
			if !done && pos.Col <= col+n {
				rs := []rune(s.Text)
				cut := pos.Col - col
				if cut > 0 {
					head := s
					head.Text = string(rs[:cut])
					out = append(out, head)
				}
				out = append(out, mark)
				if cut < n {
					tail := s
					tail.Text = string(rs[cut:])
					out = append(out, tail)
				}
				done = true
			} else {
				out = append(out, s)
			}
			col += n
		}
		if !done {
			out = append(out, mark)
		}
		ln.Spans = out
		return
	}
}

// guiPaneAt is the pane id of the files region as BuildScreen last laid it
// out, with the slot it is in. ok is false for an id not on the screen. The
// panes are made with no rows: only their ids, kinds and slots are wanted, so
// no line is built for every pointer event.
func (a *Core) guiPaneAt(id string) (p placedPane, ok bool) {
	for _, p := range a.placePanes(a.screenSlots(max(1, a.ScreenW())), 0) {
		if p.ID == id {
			return p, true
		}
	}
	return p, false
}

// paneMouse is a GUI's pointer on a pane (Mouse.Pane): Row and Col count
// from the pane's text. A press on the other pane of a split moves the focus
// there first, as in the TUI. A left press puts the cursor on the character
// under it (the core maps the cell to the rune: wide characters take two
// cells) and a drag selects; the wheel scrolls. Clicks in a diff, an image or
// a preview only move the focus.
func (a *Core) paneMouse(m Mouse) []Effect {
	if a.SearchOpen() {
		return nil
	}
	if a.PaletteOpen() {
		if m.Action == MousePress && m.Button == MouseLeft {
			a.ClosePalette()
		}
		return nil
	}
	switch m.Action {
	case MouseRelease:
		a.SetDragging(false)
		return nil
	case MouseMotion:
		if a.IsDragging() {
			a.paneCursor(m.Row, m.Col, true)
		}
		return nil
	}
	p, ok := a.guiPaneAt(m.Pane)
	if !ok {
		return nil
	}
	var effs []Effect
	if !p.slot.mine {
		if m.Button != MouseLeft && m.Button != MouseWheelUp && m.Button != MouseWheelDown {
			return nil
		}
		effs = a.FocusPane(a.OtherPane())
	}
	a.PointerMoved()
	switch m.Button {
	case MouseWheelUp, MouseWheelDown:
		d := 1
		if m.Button == MouseWheelUp {
			d = -1
		}
		if t := a.CurText(); t != nil && m.Shift {
			a.ScrollCols(t, d)
		} else {
			a.WheelEditor(d)
		}
		return effs
	case MouseWheelLeft, MouseWheelRight:
		if t := a.CurText(); t != nil {
			d := 1
			if m.Button == MouseWheelLeft {
				d = -1
			}
			a.ScrollCols(t, d)
		}
		return effs
	case MouseLeft:
	default:
		return effs
	}
	t := a.CurText()
	if t == nil || !p.editable || p.Kind != screen.KindText {
		if t != nil {
			effs = Batch(effs, a.SetFocus(FocusEditor))
		}
		return effs
	}
	a.CloseCompletion()
	a.paneCursor(m.Row, m.Col, false)
	a.SetDragging(true)
	return Batch(effs, a.SetFocus(FocusEditor))
}

// paneCursor puts the cursor of the current text on the cell row, col of its
// pane's text (extend: a drag, which selects). Past the top or the bottom it
// moves a line that way, so that dragging there scrolls.
func (a *Core) paneCursor(row, col int, extend bool) {
	t := a.CurText()
	if t == nil {
		return
	}
	b := t.Buf
	rows := max(1, a.ScreenH()-1)
	if row < 0 {
		b.MoveLines(-1, extend)
	} else if row >= rows {
		b.MoveLines(1, extend)
	}
	r := b.ScrollRow + Clamp(row, 0, rows-1)
	b.SetCursor(r, max(0, col)+b.ScrollCol, extend)
	a.FollowCursor()
}

// OpenAt is the GUI's file.open: it shows the file at path, with the cursor at
// at when it is not nil (0-based; Col counts runes), in the pane named pane
// when that is the other pane of a split. Without at, a file already open
// keeps its cursor. ok is false when the file cannot be shown
// (a.Msg says why).
func (a *Core) OpenAt(path string, at *screen.Pos, pane string) (effs []Effect, ok bool) {
	if pane != "" {
		if p, found := a.guiPaneAt(pane); found && !p.slot.mine {
			effs = a.FocusPane(a.OtherPane())
		}
	}
	if !a.OpenFile(path) {
		return effs, false
	}
	if t := a.CurText(); t != nil && at != nil {
		b := t.Buf
		r := Clamp(at.Line, 0, len(b.Lines)-1)
		c := Clamp(at.Col, 0, len(b.Lines[r]))
		b.Anchor = nil
		b.SetCursor(r, buffer.VisualCol(b.Lines[r], c), false)
		a.Follow = true
	}
	return Batch(effs, a.SetFocus(FocusEditor)), true
}

// FocusFiles is the GUI's input.focus on the files region: the keys go to the
// editor, when a file is open.
func (a *Core) FocusFiles() []Effect {
	if a.Cur() == nil {
		return nil
	}
	return a.SetFocus(FocusEditor)
}

// Exec is the GUI's command.run: the ":" command line, run as the control
// socket's exec runs it.
func (a *Core) Exec(line string) (ctl.Response, []Effect) { return a.ctlExec(line) }
