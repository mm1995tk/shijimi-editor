package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// handlePaste sends a bracketed paste to whatever has the keyboard, the same
// way handleKey routes keys. Overlays that take no text ignore it.
func (a *App) handlePaste(msg tea.PasteMsg) tea.Cmd {
	a.msg = ""
	switch {
	case a.search.open:
		return a.searchInput(msg)
	case a.pal.open:
		return a.paletteInput(msg)
	case len(a.props) > 0, a.ctx != nil, a.resizeMode:
		return nil
	case a.cmdOpen:
		return a.cmdlineInput(msg)
	case a.focus == focusTerminal:
		s := strings.ReplaceAll(strings.ReplaceAll(msg.Content, "\r\n", "\r"), "\n", "\r")
		return a.term.paste(s)
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
		a.info("全幅プレビュー中は編集できません")
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
func typed(msg tea.KeyPressMsg) string {
	if msg.Mod.Contains(tea.ModCtrl) || msg.Mod.Contains(tea.ModAlt) {
		return ""
	}
	return msg.Text
}
