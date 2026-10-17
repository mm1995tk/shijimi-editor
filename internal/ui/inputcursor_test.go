package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// bubbles reverses the cursor cell, so the cursor colour must be the
// foreground of the cursor style for it to show up as the cell background.
func TestStyleInputCursorVisible(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	ti := textinput.New()
	styleInput(&ti, cPaletteBg)
	ti.Focus()
	ti.SetCursor(0)

	fg := termenv.TrueColor.Color(curTheme.Cursor).Sequence(false)
	want := "\x1b[7;" + fg + "m"
	if view := ti.View(); !strings.Contains(view, want) {
		t.Fatalf("cursor cell should be reversed with fg %q, got %q", fg, view)
	}
}
