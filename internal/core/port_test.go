package core

import (
	"errors"
	"testing"
)

// The core hands a copy and a URL to its Port; it does not touch this machine.
func TestPortGetsClipboardAndURL(t *testing.T) {
	a := newTestApp(t, "x\n")
	f := a.FrontEnd.(*testFront)
	old := OpenExternal
	OpenExternal = func(u string) error { t.Fatalf("opened %q in the core", u); return nil }
	defer func() { OpenExternal = old }()

	a.CopyText("hello")
	if f.clip != "hello" || a.Clipboard != "hello" {
		t.Fatalf("clip %q, register %q", f.clip, a.Clipboard)
	}
	if effs := a.ExecCommand("open-url https://a.example"); len(effs) != 0 {
		t.Fatalf("effects %#v", effs)
	}
	if len(f.urls) != 1 || f.urls[0] != "https://a.example" || a.MsgErr {
		t.Fatalf("url %v err %v", f.urls, a.MsgErr)
	}
}

// How opening went comes back as an event.
func TestOpenURLResult(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Update(OpenURLResult{URL: "https://a.example"})
	if a.MsgErr || a.Msg == "" {
		t.Fatalf("opened: msg %q err %v", a.Msg, a.MsgErr)
	}
	a.Update(OpenURLResult{URL: "https://a.example", Err: errors.New("boom")})
	if !a.MsgErr {
		t.Fatalf("failure not shown: %q", a.Msg)
	}
	a.Update(OpenURLResult{URL: "https://a.example", Err: ErrRemoteSession})
	if !a.MsgErr {
		t.Fatalf("remote not shown as error: %q", a.Msg)
	}
}

// Quit goes to the Port, with no Effect.
func TestExitGoesToPort(t *testing.T) {
	a := newTestApp(t, "x\n")
	f := a.FrontEnd.(*testFront)
	if effs := a.Quit(); len(effs) != 0 || !f.exited {
		t.Fatalf("effects %#v exited %v", effs, f.exited)
	}
}

// runOf runs the one Run in effs, as the front end would, and returns its event.
func runOf(t *testing.T, effs []Effect) Event {
	t.Helper()
	for _, e := range effs {
		if r, ok := e.(Run); ok {
			return r.Run()
		}
	}
	t.Fatalf("no Run in %#v", effs)
	return nil
}

// The pull request's URL is copied when it could not be opened; a plain
// :open-url that fails afterwards is an error.
func TestOpenPRURLCopiesOnFail(t *testing.T) {
	const url = "https://github.com/o/r/pull/7"
	for name, err := range map[string]error{"remote": ErrRemoteSession, "other": errors.New("boom")} {
		a := newTestApp(t, "x\n")
		f := a.FrontEnd.(*testFront)
		a.Gd.Pr.URL = url
		a.openPRURL()
		if len(f.urls) != 1 || f.urls[0] != url {
			t.Fatalf("%s: urls %v", name, f.urls)
		}
		a.Update(OpenURLResult{URL: url, Err: err})
		if a.Clipboard != url || a.MsgErr {
			t.Fatalf("%s: clip %q err %v msg %q", name, a.Clipboard, a.MsgErr, a.Msg)
		}
	}
	a := newTestApp(t, "x\n")
	a.Gd.Pr.URL = url
	a.openPRURL()
	a.ExecCommand("open-url " + url)
	a.Update(OpenURLResult{URL: url, Err: errors.New("boom")})
	if a.Clipboard != "" || !a.MsgErr {
		t.Fatalf("plain open copied: clip %q err %v", a.Clipboard, a.MsgErr)
	}
}
