package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// bubbles reverses the cursor cell, so the cursor colour must be the
// foreground of the cursor style for it to show up as the cell background.
func TestStyleInputCursorVisible(t *testing.T) {
	ti := textinput.New()
	styleInput(&ti, cPaletteBg)
	ti.Focus()
	ti.SetCursor(0)

	want := lipgloss.NewStyle().Foreground(lipgloss.Color(curTheme.Cursor)).Reverse(true).Render(" ")
	if view := ti.View(); !strings.Contains(view, want) {
		t.Fatalf("cursor cell should be %q, got %q", want, view)
	}
}
