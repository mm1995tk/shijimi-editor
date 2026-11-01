package tui

import (
	"fmt"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// The title row, the status line and the command line (and the find bar) are
// built as screen.Title, screen.Status and screen.Cmdline by the functions
// named ...Of / ...State below, which change nothing, and drawn as cells by
// drawTitle / drawStatus / drawCmdline, which read only what they are given
// (and the colors of the theme).

// titleOf is the title row of the pane showing t: the path of the file being
// edited relative to the root (just the name in single-file mode), and ● when
// it is unsaved. It is nil when no file is open.
func (a *Core) titleOf(t *tab) *screen.Title {
	if t == nil {
		return nil
	}
	name := t.title()
	if !a.single && t.path() != "" {
		name = a.rel(t.path())
	}
	return &screen.Title{Text: name, Dirty: t.dirty()}
}

// modeStyleName is the name of the color of the mode m in the status line.
func modeStyleName(m editMode) screen.Style {
	switch m {
	case modeInsert:
		return screen.StyleModeInsert
	case modeSelect:
		return screen.StyleModeSelect
	}
	return screen.StyleModeNormal
}

// statusOf is the status line as it is now.
func (a *Core) statusOf() screen.Status {
	var s screen.Status
	if a.helix && a.focus == focusEditor && a.curText() != nil {
		s.Mode, s.Style = a.mode.label(), modeStyleName(a.mode)
		if p := a.pending; p != "" {
			if p == " " {
				p = "space"
			}
			s.Pending = p
		}
	}
	s.Branch = a.branch
	switch a.focus {
	case focusExplorer:
		s.Pane = i18n.T("app.pane.tree")
	case focusChanges:
		s.Pane = i18n.T("app.pane.changes")
	case focusTerminal:
		s.Pane = i18n.T("app.pane.terminal")
	case focusAgent:
		s.Pane = i18n.T("app.pane.suggest")
	}
	if t := a.curText(); t != nil {
		for _, d := range a.lsp.diagnostics(t.buf.Path) {
			if sevRank(d.Severity) == 1 {
				s.Errors++
			} else if d.Severity == 2 {
				s.Warnings++
			}
		}
	}
	if !a.suggestionsShown() {
		if n := len(a.suggestions()); n > 0 {
			s.Suggestions = fmt.Sprintf(i18n.T("app.suggestions"), n)
		}
	}
	if t := a.cur(); t != nil && t.kind == tabText {
		b := t.buf
		s.Cursor = toScreenPos(b.Cur)
		s.Indent = fmt.Sprintf(i18n.T("app.spaces"), len(b.IndentUnit()))
		if b.IndentUnit() == "\t" {
			s.Indent = i18n.T("app.tab")
		}
		s.Position = fmt.Sprintf("%d:%d", b.Cur.Row+1, buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)+1)
		if b.HasExtra() {
			s.Position = fmt.Sprintf(i18n.T("app.cursors"), len(b.Extra())+1) + s.Position
		}
		if sel := b.SelectedText(); sel != "" {
			s.Position += fmt.Sprintf(i18n.T("app.selected"), len([]rune(sel)))
		}
		s.Language = t.hl.Language()
		if st := a.lspStatus(); st != "" {
			s.Language += " [" + st + "]"
		}
	}
	if n := len(a.gd.rv.pending); n > 0 && a.prShown() {
		s.Unsent = fmt.Sprintf(i18n.T("app.pendingcomments"), n)
	}
	s.Version = versionLabel()
	switch {
	case a.confirmOverlay() != nil:
		// the confirmation is an Overlay, drawn over the status line
	case a.msg != "":
		s.Msg, s.MsgErr = a.msg, a.msgErr
	case a.prShown() && a.gd.rv.sync.text() != "":
		s.Msg = a.gd.rv.sync.text()
	}
	return s
}

// confirmOverlay is the confirmation now asked, as a screen.Overlay, or nil.
// A y/n question (confirm) has the two answers as Items; "press again to
// discard" (confirmQuit, confirmClose) has none, as the same key answers it.
// It has no place in a pane. It changes nothing. Drawing it is drawStatus's work.
func (a *Core) confirmOverlay() *screen.Overlay {
	ov := screen.Overlay{Kind: screen.OverlayConfirm}
	switch {
	case a.confirm != nil:
		ov.Lines = []screen.Line{{Spans: []screen.Span{{Text: a.confirm.prompt}}}}
		ov.Items = []screen.Item{{Label: "y"}, {Label: "n"}}
	case (a.confirmQuit || a.confirmClose) && a.msg != "":
		ov.Lines = []screen.Line{{Spans: []screen.Span{{Text: a.msg}}}}
	default:
		return nil
	}
	return &ov
}

// cmdlineState is the ":" / "/" line when it is open.
func (a *Core) cmdlineState() *screen.Cmdline {
	if !a.cmdOpen {
		return nil
	}
	return &screen.Cmdline{Prefix: a.cmdKind, Text: a.cmdInput.Value(), Cursor: a.cmdInput.Position()}
}

// findState is the find bar when it is open.
func (a *Core) findState() *screen.Cmdline {
	if !a.findOpen {
		return nil
	}
	return &screen.Cmdline{
		Prefix: findLabel(), Text: a.findInput.Value(), Cursor: a.findInput.Position(),
		Hint: i18n.T("app.findhint"),
	}
}
