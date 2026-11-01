package tui

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
		a.composer.ta.InsertString(strings.ReplaceAll(msg.Content, "\r\n", "\n"))
		return nil
	case a.confirm != nil:
		return nil
	case a.props.Len() > 0, a.ctx != nil, a.inResizeMode():
		return nil
	case a.cmdOpen:
		return a.cmdlineInput(msg)
	case a.focus == focusTerminal:
		s := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r\n", "\r"), "\n", "\r")
		return a.termPaste(s)
	case a.focus != focusEditor:
		return nil
	}
	t := a.curText()
	if t == nil {
		return nil
	}
	if a.findOpen {
		return a.findInputUpdate(msg)
	}
	if a.helix && a.mode != modeInsert {
		return nil
	}
	if a.fullPreview(t) {
		a.infoText(i18n.T("app.fullpreview"))
		return nil
	}
	s := strings.ReplaceAll(msg.Content, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	a.comp = nil
	a.follow = true
	t.buf.ForEachCursor(func() { t.buf.InsertBlock(s) })
	return nil
}

// typed returns the text a key press types, or "" for keys that type nothing
// (named keys and Ctrl/Alt combinations).
func typed(msg Key) string {
	if msg.Mod.Has(ModCtrl) || msg.Mod.Has(ModAlt) {
		return ""
	}
	return msg.Text
}

// lineInputUpdate passes a key or a paste to a one-line field. Ctrl+V pastes
// from the clipboard (the field itself does not read it).
func (a *Core) lineInputUpdate(l *inputField, in Input) {
	if k, ok := in.(Key); ok && k.String() == "ctrl+v" {
		if l.Focused() {
			l.Insert(a.pasteText())
		}
		return
	}
	l.Update(in)
}
