package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

func TestTermKeyBytes(t *testing.T) {
	cases := []struct {
		msg  tea.KeyPressMsg
		app  bool
		want string
	}{
		{textKey("a"), false, "a"},
		{altKey("a"), false, "\x1ba"},
		{tea.KeyPressMsg{Code: tea.KeyEnter}, false, "\r"},
		{tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, false, "\x03"},
		{tea.KeyPressMsg{Code: tea.KeyBackspace}, false, "\x7f"},
		{tea.KeyPressMsg{Code: tea.KeyUp}, false, "\x1b[A"},
		{tea.KeyPressMsg{Code: tea.KeyUp}, true, "\x1bOA"},
		{tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl}, false, "\x1b[1;5C"},
		{tea.KeyPressMsg{Code: tea.KeyDelete}, false, "\x1b[3~"},
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
	tm.run("seq 1 100; echo マーカー-$((1+1))")

	deadline := time.After(5 * time.Second)
	for {
		if strings.Contains(stripANSI(strings.Join(tm.render(40, 12, true), "\n")), "マーカー-2") {
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
	tm.wheel(mouseEvent{Button: tea.MouseWheelUp, Action: mousePress}, 0, 0, -3)
	tm.render(40, 12, true)
	if tab.snap.Offset != 3 {
		t.Fatalf("offset = %d", tab.snap.Offset)
	}
}

func TestRenderVTRowKeepsWidthWhenHostDrawsDifferently(t *testing.T) {
	cells := []vt.Cell{{Rune: 'a', Comb: "‍‍", Width: 1}, {Rune: 'b', Width: 1}}
	for _, w := range []int{2, 5} {
		if got := ansi.StringWidth(renderVTRow(cells, w, -1)); got != w {
			t.Errorf("w=%d: width = %d", w, got)
		}
	}
}

func TestRenderVTRowCursorOnDefaultCell(t *testing.T) {
	term := vt.New(vt.Options{Cols: 10, Rows: 1})
	term.Write([]byte("a"))
	var s vt.Snapshot
	term.Snapshot(&s)
	row := renderVTRow(s.Cells[0], 10, s.CursorX)
	if !strings.Contains(row, sgr(0, 0, 0, true)) {
		t.Fatalf("cursor cell not drawn: %q", row)
	}
	if sgr(0, 0, 0, true) == sgr(0, 0, 0, false) {
		t.Fatal("cursor on a default cell looks the same as without it")
	}
}

func TestTerminalHasOneShell(t *testing.T) {
	tm := newTerminal(t.TempDir())
	tm.sh = &termTab{id: 1, vt: vt.New(vt.Options{Cols: 20, Rows: 5})}
	// F2 and Ctrl+PgUp/PgDn no longer manage tabs; with no PTY they are only written.
	tm.sh.ptmx, _ = os.CreateTemp(t.TempDir(), "pty")
	for _, k := range []tea.KeyPressMsg{{Code: tea.KeyF2}, {Code: tea.KeyPgDown, Mod: tea.ModCtrl}} {
		tm.update(k)
	}
	if tm.sh == nil || tm.sh.id != 1 || tm.nextID != 0 {
		t.Fatal("a key started another shell")
	}
	// when the shell exits the panel has none, until it is focused again
	tm.update(termDoneMsg{id: 1})
	if tm.sh != nil {
		t.Fatal("exited shell kept")
	}
}
