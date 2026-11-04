package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func TestOpenURLCommand(t *testing.T) {
	a := newTestApp(t, "go https://example.com/x now\n")
	oldSSH := core.IsRemoteSession
	core.IsRemoteSession = func() bool { return false }
	defer func() { core.IsRemoteSession = oldSSH }()
	var opened string
	old := core.OpenExternal
	core.OpenExternal = func(u string) error { opened = u; return nil }
	defer func() { core.OpenExternal = old }()

	runCmd(a, a.effectCmd(a.ExecCommand("open-url https://a.example")...))
	if opened != "https://a.example" || a.MsgErr {
		t.Fatalf("opened %q err %v", opened, a.MsgErr)
	}
	a.CurText().Buf.Cur.Col = 5
	runCmd(a, a.effectCmd(a.ExecCommand("open-url")...))
	if opened != "https://example.com/x" {
		t.Fatalf("cursor url: opened %q", opened)
	}
	opened = ""
	a.CurText().Buf.Cur.Col = 0
	runCmd(a, a.effectCmd(a.ExecCommand("open-url")...))
	if opened != "" || !a.MsgErr {
		t.Fatalf("expected error, opened %q err %v", opened, a.MsgErr)
	}
	for _, bad := range []string{"ftp://x", "foo https://x", "--version https://x", "https://x https://y"} {
		a.MsgErr = false
		runCmd(a, a.effectCmd(a.ExecCommand("open-url "+bad)...))
		if opened != "" || !a.MsgErr {
			t.Fatalf("%q must be rejected (opened %q)", bad, opened)
		}
	}
}

func TestOpenURLOpenerFailureShown(t *testing.T) {
	a := newTestApp(t, "x\n")
	oldSSH := core.IsRemoteSession
	core.IsRemoteSession = func() bool { return false }
	defer func() { core.IsRemoteSession = oldSSH }()
	old := core.OpenExternal
	core.OpenExternal = func(string) error { return errors.New("xdg-open が失敗しました") }
	defer func() { core.OpenExternal = old }()

	runCmd(a, a.effectCmd(a.ExecCommand("open-url https://a.example")...))
	if !a.MsgErr {
		t.Fatalf("opener failure must be shown as error, msg %q", a.Msg)
	}
}

// runCmd executes cmd and feeds its message back into the app.
func runCmd(a *App, cmd tea.Cmd) {
	if cmd != nil {
		a.Update(cmd())
	}
}

// Over SSH the TUI does not run the opener: the browser would not appear on the
// user's screen.
func TestOpenURLOverSSHNotOpened(t *testing.T) {
	a := newTestApp(t, "x\n")
	oldOpen, oldSSH := core.OpenExternal, core.IsRemoteSession
	defer func() { core.OpenExternal, core.IsRemoteSession = oldOpen, oldSSH }()
	core.OpenExternal = func(u string) error { t.Fatalf("opened %q over SSH", u); return nil }
	core.IsRemoteSession = func() bool { return true }

	runCmd(a, a.effectCmd(a.ExecCommand("open-url https://a.example")...))
	if !a.MsgErr {
		t.Fatalf("expected an error, msg %q", a.Msg)
	}
}
