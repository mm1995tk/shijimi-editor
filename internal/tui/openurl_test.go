package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOpenURLCommand(t *testing.T) {
	a := newTestApp(t, "go https://example.com/x now\n")
	var opened string
	old := openExternal
	openExternal = func(u string) error { opened = u; return nil }
	defer func() { openExternal = old }()

	runCmd(a, a.effectCmd(a.execCommand("open-url https://a.example")...))
	if opened != "https://a.example" || a.msgErr {
		t.Fatalf("opened %q err %v", opened, a.msgErr)
	}
	a.CurText().Buf.Cur.Col = 5
	runCmd(a, a.effectCmd(a.execCommand("open-url")...))
	if opened != "https://example.com/x" {
		t.Fatalf("cursor url: opened %q", opened)
	}
	opened = ""
	a.CurText().Buf.Cur.Col = 0
	runCmd(a, a.effectCmd(a.execCommand("open-url")...))
	if opened != "" || !a.msgErr {
		t.Fatalf("expected error, opened %q err %v", opened, a.msgErr)
	}
	for _, bad := range []string{"ftp://x", "foo https://x", "--version https://x", "https://x https://y"} {
		a.msgErr = false
		runCmd(a, a.effectCmd(a.execCommand("open-url "+bad)...))
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

	runCmd(a, a.effectCmd(a.execCommand("open-url https://a.example")...))
	if !a.msgErr {
		t.Fatalf("opener failure must be shown as error, msg %q", a.msg)
	}
}
