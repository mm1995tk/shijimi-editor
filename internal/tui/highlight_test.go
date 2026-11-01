package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Rendering never lexes: colors come from a lexing command.
func TestHighlightLexesOutsideRender(t *testing.T) {
	a, tb := newGoApp(t, "package main\n")
	if tb.hl.Colors(tb.Buf) != nil {
		t.Fatal("colors before any lexing")
	}
	a.render()
	if tb.hl.Colors(tb.Buf) != nil {
		t.Fatal("render lexed the buffer")
	}
	lex(t, a, tb)
	if c := tb.hl.Colors(tb.Buf); len(c) == 0 || c[0][0] == "" {
		t.Fatalf("package keyword not colored: %v", c)
	}
}


// lexing returns the command lexing the tab. Update starts one whenever the
// colors fall behind, but tests drop the commands Update returns.
func lexing(a *App, tb *Tab) tea.Cmd {
	tb.hl.running = false
	return cmdOf(a, tb.hl.lexCmd(tb.Buf))
}


// lex runs the tab's lexing, as bubbletea would.
func lex(t *testing.T, a *App, tb *Tab) {
	t.Helper()
	cmd := lexing(a, tb)
	if cmd == nil {
		t.Fatal("no lexing to run")
	}
	a.Update(cmd())
}

// IME text arrives one rune per message. Edits during a lexing wait for it,
// then one more lexing catches up with all of them.
func TestHighlightCatchesUpAfterBurst(t *testing.T) {
	a, tb := newGoApp(t, "package main\n")
	lex(t, a, tb)
	keys(a, "o")
	running := lexing(a, tb)
	for _, r := range "あいう" {
		a.Update(textKey(string(r)))
		if tb.hl.lexCmd(tb.Buf) != nil {
			t.Fatal("a second lexing started while one runs")
		}
	}
	a.Update(running()) // lexed before あいう
	if tb.hl.version == tb.Buf.Version {
		t.Fatal("stale lexing claims the latest version")
	}
	lex(t, a, tb)
	if tb.hl.version != tb.Buf.Version {
		t.Fatal("colors did not catch up")
	}
}
