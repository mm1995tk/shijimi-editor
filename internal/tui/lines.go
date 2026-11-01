package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
func (a *App) titleOf(t *tab) *screen.Title {
	if t == nil {
		return nil
	}
	name := t.title()
	if !a.single && t.path() != "" {
		name = a.rel(t.path())
	}
	return &screen.Title{Text: name, Dirty: t.dirty()}
}

// drawTitle draws title as a row of width w. current is whether the pane is
// the one with the focus.
func drawTitle(title *screen.Title, w int, current bool) string {
	if title == nil {
		return stTabBar.Render(strings.Repeat(" ", w))
	}
	mark := "  "
	if title.Dirty {
		mark = " ●"
	}
	st := stTabActive
	if !current {
		st = stTabInactive // the pane without the focus
	}
	s := st.Render(" " + title.Text + mark + " ")
	if n := ansi.StringWidth(s); n < w {
		s += stTabBar.Render(strings.Repeat(" ", w-n))
	}
	return fitStyled(s, w, stTabBar)
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
func (a *App) statusOf() screen.Status {
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
	if l := a.layout(); l.agentW == 0 {
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
func (a *App) confirmOverlay() *screen.Overlay {
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

// drawStatus draws s as a row of width w, with the confirmation ov (if any)
// in the place of the message.
func drawStatus(s screen.Status, ov *screen.Overlay, w int) string {
	var prefix string
	prefixW := 0
	if s.Mode != "" {
		label := " " + s.Mode + " "
		if s.Pending != "" {
			label = " " + s.Mode + " " + s.Pending + " "
		}
		prefix = modeStyleOf(s.Style).Render(label)
		prefixW = ansi.StringWidth(label)
	}
	var leftParts []string
	if s.Branch != "" {
		leftParts = append(leftParts, "⎇ "+s.Branch)
	}
	if s.Pane != "" {
		leftParts = append(leftParts, s.Pane)
	}
	if s.Errors+s.Warnings > 0 {
		leftParts = append(leftParts, fmt.Sprintf("● %d  ▲ %d", s.Errors, s.Warnings))
	}
	if s.Suggestions != "" {
		leftParts = append(leftParts, s.Suggestions)
	}
	left := " " + strings.Join(leftParts, "  ")
	right := ""
	if s.Position != "" {
		right = fmt.Sprintf("%s   %s   %s   ", s.Position, s.Indent, s.Language)
	}
	if s.Unsent != "" {
		right = s.Unsent + right
	}
	right += s.Version + " "
	st := stStatus
	switch {
	case ov != nil && ov.Kind == screen.OverlayConfirm:
		st = stStatusError
		if len(ov.Items) > 0 {
			right = "" // the question must not be cut off
		}
		left += "   " + ov.Lines[0].Spans[0].Text
	case s.Msg != "":
		if s.MsgErr {
			st = stStatusError
		}
		left += "   " + s.Msg
	}
	lw := w - prefixW - ansi.StringWidth(right)
	return prefix + st.Render(fit(left, lw)+right)
}

// modeStyleOf is the lipgloss style of the mode color st.
func modeStyleOf(st screen.Style) lipgloss.Style {
	switch st {
	case screen.StyleModeInsert:
		return modeStyles[modeInsert]
	case screen.StyleModeSelect:
		return modeStyles[modeSelect]
	}
	return modeStyles[modeNormal]
}

// cmdlineState is the ":" / "/" line when it is open.
func (a *App) cmdlineState() *screen.Cmdline {
	if !a.cmdOpen {
		return nil
	}
	return &screen.Cmdline{Prefix: a.cmdKind, Text: a.cmdInput.Value(), Cursor: a.cmdInput.Position()}
}

// findState is the find bar when it is open.
func (a *App) findState() *screen.Cmdline {
	if !a.findOpen {
		return nil
	}
	return &screen.Cmdline{
		Prefix: findLabel(), Text: a.findInput.Value(), Cursor: a.findInput.Position(),
		Hint: i18n.T("app.findhint"),
	}
}

// drawCmdline draws the ":" / "/" line of width w. input is the text field as
// the widget draws it (its scrolling and cursor), which stays until the
// widget is replaced by our own type (P4).
func drawCmdline(c screen.Cmdline, input string, w int) string {
	return stPanel.Render(c.Prefix) + fitStyled(input, w-1, stPanel)
}

// drawFind draws the find bar of width w, with the text field as the widget
// draws it (see drawCmdline).
func drawFind(c screen.Cmdline, input string, w int) string {
	label := stFindLabel.Render(c.Prefix)
	hint := stFind.Render(c.Hint)
	iw := findInputCells(w)
	return label + fitStyled(input, iw, stFind) + hint
}
