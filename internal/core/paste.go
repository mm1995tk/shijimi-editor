package core

import (
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// handlePaste sends a bracketed paste to whatever has the keyboard, the same
// way handleKey routes keys. Overlays that take no text ignore it.
func (a *Core) HandlePaste(msg Paste) []Effect {
	a.Msg = ""
	switch {
	case a.Search.Open:
		return a.searchInput(msg)
	case a.Pal.Open:
		return a.paletteInput(msg)
	case a.Composer != nil:
		a.Composer.Ta.InsertString(strings.ReplaceAll(msg.Content, "\r\n", "\n"))
		return nil
	case a.Confirm != nil:
		return nil
	case a.props.Len() > 0, a.Ctx != nil, a.InResizeMode():
		return nil
	case a.CmdOpen:
		return a.cmdlineInput(msg)
	case a.Focus == FocusTerminal:
		s := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r\n", "\r"), "\n", "\r")
		return a.TermPaste(s)
	case a.Focus != FocusEditor:
		return nil
	}
	t := a.CurText()
	if t == nil {
		return nil
	}
	if a.FindOpen {
		return a.findInputUpdate(msg)
	}
	if a.Helix && a.Mode != ModeInsert {
		return nil
	}
	if a.FullPreview(t) {
		a.InfoText(i18n.T("app.fullpreview"))
		return nil
	}
	s := strings.ReplaceAll(msg.Content, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	a.Comp = nil
	a.Follow = true
	t.Buf.ForEachCursor(func() { t.Buf.InsertBlock(s) })
	return nil
}

// typed returns the text a key press types, or "" for keys that type nothing
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
