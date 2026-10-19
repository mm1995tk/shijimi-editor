package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newGoApp(t *testing.T, src string) (*App, *tab) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	os.WriteFile(p, []byte(src), 0o644)
	a := New(dir, p)
	a.helix = true
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a, a.curText()
}

// lexing returns the command lexing the tab. Update starts one whenever the
// colors fall behind, but tests drop the commands Update returns.
func lexing(tb *tab) tea.Cmd {
	tb.hl.running = false
	return tb.hl.lexCmd(tb.buf)
}

// lex runs the tab's lexing, as bubbletea would.
func lex(t *testing.T, a *App, tb *tab) {
	t.Helper()
	cmd := lexing(tb)
	if cmd == nil {
		t.Fatal("no lexing to run")
	}
	a.Update(cmd())
}

// Rendering never lexes: colors come from a lexing command.
func TestHighlightLexesOutsideRender(t *testing.T) {
	a, tb := newGoApp(t, "package main\n")
	if tb.hl.Colors(tb.buf) != nil {
		t.Fatal("colors before any lexing")
	}
	a.render()
	if tb.hl.Colors(tb.buf) != nil {
		t.Fatal("render lexed the buffer")
	}
	lex(t, a, tb)
	if c := tb.hl.Colors(tb.buf); len(c) == 0 || c[0][0] == "" {
		t.Fatalf("package keyword not colored: %v", c)
	}
}

// IME text arrives one rune per message. Edits during a lexing wait for it,
// then one more lexing catches up with all of them.
func TestHighlightCatchesUpAfterBurst(t *testing.T) {
	a, tb := newGoApp(t, "package main\n")
	lex(t, a, tb)
	keys(a, "o")
	running := lexing(tb)
	for _, r := range "あいう" {
		a.Update(textKey(string(r)))
		if tb.hl.lexCmd(tb.buf) != nil {
			t.Fatal("a second lexing started while one runs")
		}
	}
	a.Update(running()) // lexed before あいう
	if tb.hl.version == tb.buf.Version {
		t.Fatal("stale lexing claims the latest version")
	}
	lex(t, a, tb)
	if tb.hl.version != tb.buf.Version {
		t.Fatal("colors did not catch up")
	}
}

// Until an edit is lexed, unchanged lines keep their colors and edited ones
// have none, even when lines were inserted above.
func TestAlignColors(t *testing.T) {
	lexed := []string{"a", "b", "c"}
	colors := [][]string{{"1"}, {"2"}, {"3"}}
	got := alignColors(lexed, colors, [][]rune{[]rune("a"), []rune("x"), []rune("y"), []rune("c")})
	want := [][]string{{"1"}, nil, nil, {"3"}}
	for i := range want {
		if len(got[i]) != len(want[i]) || len(got[i]) > 0 && got[i][0] != want[i][0] {
			t.Fatalf("line %d = %v, want %v", i, got[i], want[i])
		}
	}
}
