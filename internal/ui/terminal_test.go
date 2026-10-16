package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSplitIncomplete(t *testing.T) {
	b := []byte("a\xe3\x81")
	done, rest := splitIncomplete(b)
	if string(done) != "a" || len(rest) != 2 {
		t.Fatalf("got %q %q", done, rest)
	}
	done, rest = splitIncomplete([]byte("あ"))
	if string(done) != "あ" || len(rest) != 0 {
		t.Fatalf("got %q %q", done, rest)
	}
}

func TestTermKeyBytes(t *testing.T) {
	cases := []struct {
		msg  tea.KeyMsg
		app  bool
		want string
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, false, "a"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a"), Alt: true}, false, "\x1ba"},
		{tea.KeyMsg{Type: tea.KeyEnter}, false, "\r"},
		{tea.KeyMsg{Type: tea.KeyCtrlC}, false, "\x03"},
		{tea.KeyMsg{Type: tea.KeyBackspace}, false, "\x7f"},
		{tea.KeyMsg{Type: tea.KeyUp}, false, "\x1b[A"},
		{tea.KeyMsg{Type: tea.KeyUp}, true, "\x1bOA"},
		{tea.KeyMsg{Type: tea.KeyCtrlRight}, false, "\x1b[1;5C"},
		{tea.KeyMsg{Type: tea.KeyDelete}, false, "\x1b[3~"},
	}
	for _, c := range cases {
		if got := string(termKeyBytes(c.msg, c.app)); got != c.want {
			t.Errorf("%v: got %q want %q", c.msg, got, c.want)
		}
	}
}

// A real shell on a real PTY: login-shell startup files must run.
func TestTerminalRunsLoginShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "")
	if err := writeFile(home+"/.profile", "export FROM_PROFILE=dotfile-ok\n"); err != nil {
		t.Fatal(err)
	}
	tm := newTerminal(home)
	tm.focus()
	defer tm.closeAll()
	tm.run("echo $FROM_PROFILE")

	deadline := time.After(5 * time.Second)
	for {
		rows := tm.render(60, 12, true)
		if strings.Contains(stripANSI(strings.Join(rows, "\n")), "dotfile-ok") {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("output not found:\n%s", stripANSI(strings.Join(rows, "\n")))
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func writeFile(p, s string) error { return os.WriteFile(p, []byte(s), 0o644) }

func stripANSI(s string) string { return ansi.Strip(s) }
