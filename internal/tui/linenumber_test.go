package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// firstLineNumber is the number in the gutter of the first line: 1 when
// absolute, the cursor row when relative.
func firstLineNumber(a *App) string {
	rows := renderEditor(a.CurText(), 40, 5, a.EditorOpts(a.CurText(), true))
	return strings.Fields(ansi.Strip(rows[0]))[0]
}


func TestLineFlipWhileHeld(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 20))
	keys(a, "5", "g")
	a.Update(tea.KeyboardEnhancementsMsg{Flags: ansi.KittyReportEventTypes})
	if got := firstLineNumber(a); got != "1" {
		t.Fatalf("absolute: %q, want 1", got)
	}
	a.Update(backslash)
	repeat := backslash
	repeat.Repeat = true
	a.Update(repeat) // a repeat keeps it flipped
	if got := firstLineNumber(a); got != "4" || !a.lineFlip {
		t.Fatalf("held: %q flip=%v, want relative", got, a.lineFlip)
	}
	a.Update(tea.KeyReleaseMsg{Code: '\\'})
	if a.lineFlip {
		t.Fatal("still flipped after the release")
	}

	// another key ends it, as its release may never come
	a.Update(backslash)
	keys(a, "j")
	if a.lineFlip {
		t.Fatal("still flipped after another key")
	}
}

func TestLineFlipTogglesWithoutKeyReleases(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 20))
	a.cfg.Editor.LineNumber = "relative"
	keys(a, "5", "g")
	if got := firstLineNumber(a); got != "4" {
		t.Fatalf("relative: %q, want 4", got)
	}
	a.Update(backslash)
	if got := firstLineNumber(a); got != "1" {
		t.Fatalf("after \\: %q, want 1", got)
	}
	keys(a, "j", "k")
	if !a.lineFlip {
		t.Fatal("other keys ended the switch")
	}
	a.Update(backslash)
	if a.lineFlip {
		t.Fatal("second \\ did not switch back")
	}
}
