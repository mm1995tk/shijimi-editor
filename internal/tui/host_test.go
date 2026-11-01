package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"
)

// With a Host (shj serve) the copy and the URL go to the GUI, not to this machine.
func TestHostGetsClipboardAndURL(t *testing.T) {
	a := newTestApp(t, "x\n")
	h := &fakeHost{}
	a.SetHost(h)
	old := core.OpenExternal
	core.OpenExternal = func(u string) error { t.Fatalf("opened %q on the server", u); return nil }
	defer func() { core.OpenExternal = old }()

	a.CopyText("hello")
	if len(h.clip) != 1 || h.clip[0] != "hello" || a.Clipboard != "hello" {
		t.Fatalf("clip %v, register %q", h.clip, a.Clipboard)
	}
	runCmd(a, a.effectCmd(a.ExecCommand("open-url https://a.example")...))
	if len(h.url) != 1 || h.url[0] != "https://a.example" || a.MsgErr {
		t.Fatalf("url %v err %v", h.url, a.MsgErr)
	}
}
