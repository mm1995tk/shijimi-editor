package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/screen"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

// titleCopyMin is the width the path keeps at least before the copy button is
// dropped from a narrow pane.
const titleCopyMin = 8

// titleCopyWidth is the width of the copy button at the right end of a title
// row of width w, or 0 when the title has none or the pane is too narrow.
func titleCopyWidth(title *screen.Title, w int) int {
	if title == nil || !title.Copy {
		return 0
	}
	bw := ansi.StringWidth(i18n.T("title.copy"))
	if w < bw+titleCopyMin {
		return 0
	}
	return bw
}

// copyButton is the copy button when bw (from titleCopyWidth) is not 0.
func copyButton(bw int) string {
	if bw == 0 {
		return ""
	}
	return stTabBar.Render(i18n.T("title.copy"))
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
	bw := titleCopyWidth(title, w)
	s := fitStyled(st.Render(" "+title.Text+mark+" "), w-bw, stTabBar) // the path is cut first
	if n := ansi.StringWidth(s); n < w-bw {
		s += stTabBar.Render(strings.Repeat(" ", w-bw-n))
	}
	return s + copyButton(bw)
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
	if !s.Pane.IsZero() {
		leftParts = append(leftParts, tr(s.Pane))
	}
	if s.Errors+s.Warnings > 0 {
		leftParts = append(leftParts, fmt.Sprintf("● %d  ▲ %d", s.Errors, s.Warnings))
	}
	if !s.Suggestions.IsZero() {
		leftParts = append(leftParts, tr(s.Suggestions))
	}
	left := " " + strings.Join(leftParts, "  ")
	right := ""
	if !s.Position.IsZero() {
		right = fmt.Sprintf("%s   %s   %s   ", tr(s.Position), tr(s.Indent), tr(s.Language))
	}
	if !s.Unsent.IsZero() {
		right = tr(s.Unsent) + right
	}
	right += s.Version + " "
	st := stStatus
	switch {
	case ov != nil && ov.Kind == screen.OverlayConfirm:
		st = stStatusError
		if len(ov.Items) > 0 {
			right = "" // the question must not be cut off
		}
		left += "   " + tr(ov.Title)
	case !s.Msg.IsZero():
		if s.MsgErr {
			st = stStatusError
		}
		left += "   " + tr(s.Msg)
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
	return stPanel.Render(tr(c.Prefix)) + fitStyled(input, w-1, stPanel)
}

// drawFind draws the find bar of width w, with the text field as the widget
// draws it (see drawCmdline).
func drawFind(c screen.Cmdline, input string, w int) string {
	label := stFindLabel.Render(tr(c.Prefix))
	hint := stFind.Render(tr(c.Hint))
	iw := findInputCells(w)
	return label + fitStyled(input, iw, stFind) + hint
}
