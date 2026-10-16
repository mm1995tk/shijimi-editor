package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/gh-editor/internal/vt"
)

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

func TestRenderVTRowKeepsWideCharsAligned(t *testing.T) {
	v := vt.New(vt.Options{Cols: 12, Rows: 1})
	v.Write([]byte("日本語abc"))
	var sn vt.Snapshot
	v.Snapshot(&sn)
	for _, cx := range []int{-1, 0, 1, 9} {
		row := renderVTRow(sn.Cells[0], 12, cx)
		if got := ansi.StringWidth(row); got != 12 {
			t.Errorf("cursor %d: width = %d want 12", cx, got)
		}
		if got := strings.TrimRight(stripANSI(row), " "); got != "日本語abc" {
			t.Errorf("cursor %d: text = %q", cx, got)
		}
	}
}

// Output from a real shell on a real PTY can be scrolled back through.
func TestTerminalScrollback(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	tm := newTerminal(t.TempDir())
	tm.focus()
	defer tm.closeAll()
	tm.run("seq 1 100; echo done-マーカー")

	deadline := time.After(5 * time.Second)
	for {
		if strings.Contains(stripANSI(strings.Join(tm.render(40, 12, true), "\n")), "done-マーカー") {
			break
		}
		select {
		case <-deadline:
			t.Fatal("output not found")
		case <-time.After(20 * time.Millisecond):
		}
	}
	tab := tm.cur()
	if tab.vt.Scrollback() < 50 {
		t.Fatalf("scrollback = %d", tab.vt.Scrollback())
	}
	tm.wheel(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress}, 0, 0, -3)
	tm.render(40, 12, true)
	if tab.snap.Offset != 3 {
		t.Fatalf("offset = %d", tab.snap.Offset)
	}
}
