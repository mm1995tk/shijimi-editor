package core

import (
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// HandlePaste sends a bracketed paste to whatever has the keyboard, the same
// way HandleKey routes keys. Overlays that take no text ignore it.
func (a *Core) HandlePaste(in Paste) []Effect {
	a.Msg = msg.Msg{}
	switch {
	case a.Search.Open:
		return a.searchInput(in)
	case a.Pal.Open:
		return a.paletteInput(in)
	case a.Composer != nil:
		a.Composer.Ta.InsertString(in.Content)
		return nil
	case a.Confirm != nil:
		return nil
	case a.props.Len() > 0, a.Ctx != nil, a.InResizeMode():
		return nil
	case a.CmdOpen:
		return a.cmdlineInput(in)
	case a.Focus == FocusTerminal:
		s := strings.ReplaceAll(strings.ReplaceAll(in.Content, "\r\n", "\r"), "\n", "\r")
		a.port.TermPaste(s)
		return nil
	case a.Focus != FocusEditor:
		return nil
	}
	t := a.CurText()
	if t == nil {
		return nil
	}
	if a.FindOpen {
		return a.findInputUpdate(in)
	}
	if a.Helix && a.Mode != ModeInsert {
		return nil
	}
	if a.FullPreview(t) {
		a.InfoText(msg.T("app.fullpreview"))
		return nil
	}
	s := strings.ReplaceAll(in.Content, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	a.Comp = nil
	a.Follow = true
	t.Buf.ForEachCursor(func() { t.Buf.InsertBlock(s) })
	return nil
}

// Typed returns the text a key press types, or "" for keys that type nothing
// (named keys and Ctrl/Alt combinations).
func Typed(msg Key) string {
	if msg.Mod.Has(ModCtrl) || msg.Mod.Has(ModAlt) {
		return ""
	}
	return msg.Text
}

// lineInputUpdate passes a key or a paste to a one-line field. Ctrl+V pastes
// from the clipboard (the field itself does not read it).
func (a *Core) lineInputUpdate(l *InputField, in Input) {
	if k, ok := in.(Key); ok && k.String() == "ctrl+v" {
		if l.Focused() {
			l.Insert(a.pasteText())
		}
		return
	}
	l.Update(in)
}
