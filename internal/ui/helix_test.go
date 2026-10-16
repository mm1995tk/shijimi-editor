package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keys(a *App, ks ...string) {
	for _, k := range ks {
		var m tea.KeyMsg
		switch k {
		case "esc":
			m = tea.KeyMsg{Type: tea.KeyEscape}
		case "enter":
			m = tea.KeyMsg{Type: tea.KeyEnter}
		case "space":
			m = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		default:
			m = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		a.Update(m)
	}
}

func newTestApp(t *testing.T, content string) *App {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte(content), 0o644)
	a := New(dir, p)
	a.helix = true
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a
}

func TestHelixLineOps(t *testing.T) {
	a := newTestApp(t, "one\ntwo\nthree\n")
	b := a.curText().buf
	keys(a, "2", "g")
	if b.Cur.Row != 1 {
		t.Fatalf("2g -> row %d", b.Cur.Row)
	}
	keys(a, "x", "d")
	if got := b.Text(); got != "one\nthree\n" {
		t.Fatalf("xd -> %q", got)
	}
	keys(a, "g", "g", "P")
	if got := b.Text(); got != "two\none\nthree\n" {
		t.Fatalf("ggP -> %q", got)
	}
	keys(a, "u", "u")
	if got := b.Text(); got != "one\ntwo\nthree\n" {
		t.Fatalf("uu -> %q", got)
	}
}

func TestHelixWordChangeAndInsert(t *testing.T) {
	a := newTestApp(t, "foo bar\n")
	b := a.curText().buf
	keys(a, "w")
	if got := b.SelectedText(); got != "foo " {
		t.Fatalf("w selects %q", got)
	}
	keys(a, ";", "g", "h", "e", "c")
	keys(a, "b", "a", "z", "esc")
	if got := b.Text(); got != "baz bar\n" {
		t.Fatalf("ec -> %q", got)
	}
	keys(a, "A", "!", "esc", "o", "x", "esc")
	if got := b.Text(); got != "baz bar!\nx\n" {
		t.Fatalf("A/o -> %q", got)
	}
}

func TestCmdline(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\n")
	b := a.curText().buf
	keys(a, ":", "3", "enter")
	if b.Cur.Row != 2 {
		t.Fatalf(":3 -> row %d", b.Cur.Row)
	}
	keys(a, "/", "b", "enter")
	if b.Cur.Row != 1 {
		t.Fatalf("/b -> %+v", b.Cur)
	}
	keys(a, "i", "X", "esc", ":", "w", "enter")
	data, _ := os.ReadFile(b.Path)
	if string(data) != "a\nXb\nc\n" {
		t.Fatalf("saved %q", data)
	}
}

func TestGroupedRunes(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\n")
	b := a.curText().buf
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3gx")})
	if b.SelectedText() != "c\n" {
		t.Fatalf("grouped runes -> %+v %q", b.Cur, b.SelectedText())
	}
}
