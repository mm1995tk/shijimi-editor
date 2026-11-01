package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

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
	return prefix + st.Render(core.Fit(left, lw)+right)
}

// modeStyleOf is the lipgloss style of the mode color st.
func modeStyleOf(st screen.Style) lipgloss.Style {
	switch st {
	case screen.StyleModeInsert:
		return modeStyles[core.ModeInsert]
	case screen.StyleModeSelect:
		return modeStyles[core.ModeSelect]
	}
	return modeStyles[core.ModeNormal]
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
