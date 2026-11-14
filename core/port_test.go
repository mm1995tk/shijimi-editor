package core

import (
	"errors"
	"testing"
)

// The core hands a copy and a URL to its Port; it does not touch this machine.
func TestPortGetsClipboardAndURL(t *testing.T) {
	a := newTestApp(t, "x\n")
	f := a.port.(*testFront)
	old := OpenExternal
	OpenExternal = func(u string) error { t.Fatalf("opened %q in the core", u); return nil }
	defer func() { OpenExternal = old }()

	a.CopyText("hello")
	if f.clip != "hello" || a.Clipboard != "hello" {
		t.Fatalf("clip %q, register %q", f.clip, a.Clipboard)
	}
	if effs := a.openURL("https://a.example"); len(effs) != 0 {
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
	if a.MsgErr || a.Msg.IsZero() {
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
	f := a.port.(*testFront)
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
// URL opened some other way (a language server asks for it) that fails afterwards is an error.
func TestOpenPRURLCopiesOnFail(t *testing.T) {
	const url = "https://github.com/o/r/pull/7"
	for name, err := range map[string]error{"remote": ErrRemoteSession, "other": errors.New("boom")} {
		a := newTestApp(t, "x\n")
		f := a.port.(*testFront)
		a.Gd.Pr = &PrInfo{URL: url}
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
	a.Gd.Pr = &PrInfo{URL: url}
	a.openPRURL()
	a.openURL(url)
	a.Update(OpenURLResult{URL: url, Err: errors.New("boom")})
	if a.Clipboard != "" || !a.MsgErr {
		t.Fatalf("plain open copied: clip %q err %v", a.Clipboard, a.MsgErr)
	}
}

// The terminal panel is the UI's: the core only says what it wants of it.
func TestTerminalGoesToPort(t *testing.T) {
	a := newTestApp(t, "x\n")
	f := a.port.(*testFront)
	a.SetFocus(FocusTerminal)
	if !f.termFocused {
		t.Fatal("the terminal was not given the keys")
	}
	a.SetFocus(FocusEditor)
	if f.termFocused {
		t.Fatal("the terminal kept the keys")
	}
	a.ExecCommand("sh echo hi")
	if len(f.termRuns) != 1 || f.termRuns[0] != "echo hi" || !f.termFocused {
		t.Fatalf("runs %v focused %v", f.termRuns, f.termFocused)
	}
	a.Quit()
	if !f.termClosed {
		t.Fatal("quit left the shell open")
	}
}

// A UI whose terminal is a pane outside it (Layout.TermInPane) shows, hides and
// feeds it itself: the core keeps neither the panel nor the terminal focus.
func TestTerminalInPane(t *testing.T) {
	a := newTestApp(t, "x\n")
	f := a.port.(*testFront)
	f.inPane = true
	f.push()

	a.ToggleTerm()
	if f.termToggled != 1 || f.term || a.Focus != FocusEditor {
		t.Fatalf("toggled %d, panel %v, focus %v", f.termToggled, f.term, a.Focus)
	}
	a.ExecCommand("sh make")
	if len(f.termRuns) != 1 || f.termRuns[0] != "make" || f.term || f.termFocused {
		t.Fatalf("runs %v panel %v focused %v", f.termRuns, f.term, f.termFocused)
	}
	a.ExecCommand("sh hide")
	if len(f.termRuns) != 1 || f.termShows != "hide" {
		t.Fatalf("hide: runs %v shows %q", f.termRuns, f.termShows)
	}
	a.Ctx = &CtxPreview{Text: "ctx"}
	a.HandleContextKey(Key{Code: KeyEnter})
	if len(f.termPasted) != 1 || f.termPasted[0] != "ctx" || f.term {
		t.Fatalf("pasted %v panel %v", f.termPasted, f.term)
	}
}

// A URL that is not exactly one http or https URL is not handed to the opener.
func TestOpenURLRejectsBadURL(t *testing.T) {
	for _, bad := range []string{"ftp://x", "foo https://x", "--version https://x", "https://x https://y"} {
		a := newTestApp(t, "x\n")
		f := a.port.(*testFront)
		a.openURL(bad)
		if len(f.urls) != 0 || !a.MsgErr {
			t.Fatalf("%q must be rejected: urls %v err %v", bad, f.urls, a.MsgErr)
		}
	}
}
