package core

import (
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// handlePaste sends a bracketed paste to whatever has the keyboard, the same
// way handleKey routes keys. Overlays that take no text ignore it.
func (a *Core) handlePaste(msg Paste) []Effect {
	a.msg = ""
	switch {
	case a.search.open:
		return a.searchInput(msg)
	case a.pal.open:
		return a.paletteInput(msg)
	case a.composer != nil:
		a.composer.Ta.InsertString(strings.ReplaceAll(msg.Content, "\r\n", "\n"))
		return nil
	case a.confirm != nil:
		return nil
	case a.props.Len() > 0, a.ctx != nil, a.InResizeMode():
		return nil
	case a.cmdOpen:
		return a.cmdlineInput(msg)
	case a.focus == FocusTerminal:
		s := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r\n", "\r"), "\n", "\r")
		return a.TermPaste(s)
	case a.focus != FocusEditor:
		return nil
	}
	t := a.CurText()
	if t == nil {
		return nil
	}
	if a.findOpen {
		return a.findInputUpdate(msg)
	}
	if a.helix && a.mode != ModeInsert {
		return nil
	}
	if a.FullPreview(t) {
		a.InfoText(i18n.T("app.fullpreview"))
		return nil
	}
	s := strings.ReplaceAll(msg.Content, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	a.comp = nil
	a.follow = true
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
