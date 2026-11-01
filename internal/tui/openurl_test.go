package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUrlAt(t *testing.T) {
	line := []rune("see (https://example.com/a?b=1). ok")
	for col, want := range map[int]string{5: "https://example.com/a?b=1", 30: "", 0: ""} {
		got, ok := urlAt(line, col)
		if got != want || ok != (want != "") {
			t.Errorf("col %d: got %q,%v want %q", col, got, ok, want)
		}
	}
}

// runCmd executes cmd and feeds its message back into the app.
func runCmd(a *App, cmd tea.Cmd) {
	if cmd != nil {
		a.Update(cmd())
	}
}

func TestOpenURLCommand(t *testing.T) {
	a := newTestApp(t, "go https://example.com/x now\n")
	var opened string
	old := openExternal
	openExternal = func(u string) error { opened = u; return nil }
	defer func() { openExternal = old }()

	runCmd(a, a.execCommand("open-url https://a.example"))
	if opened != "https://a.example" || a.msgErr {
		t.Fatalf("opened %q err %v", opened, a.msgErr)
	}
	a.curText().buf.Cur.Col = 5
	runCmd(a, a.execCommand("open-url"))
	if opened != "https://example.com/x" {
		t.Fatalf("cursor url: opened %q", opened)
	}
	opened = ""
	a.curText().buf.Cur.Col = 0
	runCmd(a, a.execCommand("open-url"))
	if opened != "" || !a.msgErr {
		t.Fatalf("expected error, opened %q err %v", opened, a.msgErr)
	}
	for _, bad := range []string{"ftp://x", "foo https://x", "--version https://x", "https://x https://y"} {
		a.msgErr = false
		runCmd(a, a.execCommand("open-url "+bad))
		if opened != "" || !a.msgErr {
			t.Fatalf("%q must be rejected (opened %q)", bad, opened)
		}
	}
}

func TestOpenURLOpenerFailureShown(t *testing.T) {
	a := newTestApp(t, "x\n")
	old := openExternal
	openExternal = func(string) error { return errors.New("xdg-open が失敗しました") }
	defer func() { openExternal = old }()

	runCmd(a, a.execCommand("open-url https://a.example"))
	if !a.msgErr {
		t.Fatalf("opener failure must be shown as error, msg %q", a.msg)
	}
}

func TestOpenExternalMissingCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := openExternal("https://example.com"); err == nil {
		t.Fatal("expected error when opener is missing")
	}
}
