package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// Rendering never lexes: colors come from a lexing command.
func TestHighlightLexesOutsideRender(t *testing.T) {
	a, tb := newGoApp(t, "package main\n")
	if tb.Hl.Colors(tb.Buf) != nil {
		t.Fatal("colors before any lexing")
	}
	a.render()
	if tb.Hl.Colors(tb.Buf) != nil {
		t.Fatal("render lexed the buffer")
	}
	lex(t, a, tb)
	if c := tb.Hl.Colors(tb.Buf); len(c) == 0 || c[0][0] == "" {
		t.Fatalf("package keyword not colored: %v", c)
	}
}

// lexing returns the command lexing the tab. Update starts one whenever the
// colors fall behind, but tests drop the commands Update returns.
func lexing(a *App, tb *core.Tab) tea.Cmd {
	tb.Hl.Running = false
	return cmdOf(a, tb.Hl.LexCmd(tb.Buf))
}

// lex runs the tab's lexing, as bubbletea would.
func lex(t *testing.T, a *App, tb *core.Tab) {
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
		if tb.Hl.LexCmd(tb.Buf) != nil {
			t.Fatal("a second lexing started while one runs")
		}
	}
	a.Update(running()) // lexed before あいう
	if tb.Hl.Version == tb.Buf.Version {
		t.Fatal("stale lexing claims the latest version")
	}
	lex(t, a, tb)
	if tb.Hl.Version != tb.Buf.Version {
		t.Fatal("colors did not catch up")
	}
}
