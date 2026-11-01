package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// The caret cell is reversed, so the cursor colour must be the foreground of
// the cursor style for it to show up as the cell background.
func TestInputCursorVisible(t *testing.T) {
	ti := core.NewInputField(0)
	ti.Focus()
	ti.SetCursor(0)

	want := lipgloss.NewStyle().Foreground(lipgloss.Color(core.CurTheme.Cursor)).Reverse(true).Render(" ")
	if view := viewField(&ti, inputStyleOn(cPaletteBg)); !strings.Contains(view, want) {
		t.Fatalf("cursor cell should be %q, got %q", want, view)
	}
}
