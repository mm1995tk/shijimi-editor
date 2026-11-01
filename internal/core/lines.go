package core

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
func (a *Core) TitleOf(t *Tab) *screen.Title {
	if t == nil {
		return nil
	}
	name := t.Title()
	if !a.single && t.Path() != "" {
		name = a.Rel(t.Path())
	}
	return &screen.Title{Text: name, Dirty: t.dirty()}
}

// modeStyleName is the name of the color of the mode m in the status line.
func modeStyleName(m EditMode) screen.Style {
	switch m {
	case ModeInsert:
		return screen.StyleModeInsert
	case ModeSelect:
		return screen.StyleModeSelect
	}
	return screen.StyleModeNormal
}

// statusOf is the status line as it is now.
func (a *Core) StatusOf() screen.Status {
	var s screen.Status
	if a.Helix && a.Focus == FocusEditor && a.CurText() != nil {
		s.Mode, s.Style = a.Mode.label(), modeStyleName(a.Mode)
		if p := a.Pending; p != "" {
			if p == " " {
				p = "space"
			}
			s.Pending = p
		}
	}
	s.Branch = a.Branch
	switch a.Focus {
	case FocusExplorer:
		s.Pane = i18n.T("app.pane.tree")
	case FocusChanges:
		s.Pane = i18n.T("app.pane.changes")
	case FocusTerminal:
		s.Pane = i18n.T("app.pane.terminal")
	case FocusAgent:
		s.Pane = i18n.T("app.pane.suggest")
	}
	if t := a.CurText(); t != nil {
		for _, d := range a.Lsp.diagnostics(t.Buf.Path) {
			if sevRank(d.Severity) == 1 {
				s.Errors++
			} else if d.Severity == 2 {
				s.Warnings++
			}
		}
	}
	if !a.SuggestionsShown() {
		if n := len(a.Suggestions()); n > 0 {
			s.Suggestions = fmt.Sprintf(i18n.T("app.suggestions"), n)
		}
	}
	if t := a.Cur(); t != nil && t.Kind == TabText {
		b := t.Buf
		s.Cursor = ToScreenPos(b.Cur)
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
		s.Language = t.Hl.Language()
		if st := a.lspStatus(); st != "" {
			s.Language += " [" + st + "]"
		}
	}
	if n := len(a.Gd.Rv.Pending); n > 0 && a.PrShown() {
		s.Unsent = fmt.Sprintf(i18n.T("app.pendingcomments"), n)
	}
	s.Version = VersionLabel()
	switch {
	case a.ConfirmOverlay() != nil:
		// the confirmation is an Overlay, drawn over the status line
	case a.Msg != "":
		s.Msg, s.MsgErr = a.Msg, a.MsgErr
	case a.PrShown() && a.Gd.Rv.Sync.text() != "":
		s.Msg = a.Gd.Rv.Sync.text()
	}
	return s
}

// confirmOverlay is the confirmation now asked, as a screen.Overlay, or nil.
// A y/n question (confirm) has the two answers as Items; "press again to
// discard" (confirmQuit, confirmClose) has none, as the same key answers it.
// It has no place in a pane. It changes nothing. Drawing it is drawStatus's work.
func (a *Core) ConfirmOverlay() *screen.Overlay {
	ov := screen.Overlay{Kind: screen.OverlayConfirm}
	switch {
	case a.Confirm != nil:
		ov.Lines = []screen.Line{{Spans: []screen.Span{{Text: a.Confirm.Prompt}}}}
		ov.Items = []screen.Item{{Label: "y"}, {Label: "n"}}
	case (a.confirmQuit || a.confirmClose) && a.Msg != "":
		ov.Lines = []screen.Line{{Spans: []screen.Span{{Text: a.Msg}}}}
	default:
		return nil
	}
	return &ov
}

// cmdlineState is the ":" / "/" line when it is open.
func (a *Core) CmdlineState() *screen.Cmdline {
	if !a.CmdOpen {
		return nil
	}
	return &screen.Cmdline{Prefix: a.cmdKind, Text: a.cmdInput.Value(), Cursor: a.cmdInput.Position()}
}

// findState is the find bar when it is open.
func (a *Core) FindState() *screen.Cmdline {
	if !a.FindOpen {
		return nil
	}
	return &screen.Cmdline{
		Prefix: FindLabel(), Text: a.FindInput.Value(), Cursor: a.FindInput.Position(),
		Hint: i18n.T("app.findhint"),
	}
}
