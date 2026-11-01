package core

import "testing"

type fakeHost struct{ clip, url []string }

func (h *fakeHost) WriteClipboard(s string) { h.clip = append(h.clip, s) }

func (h *fakeHost) OpenURL(u string) { h.url = append(h.url, u) }

// With a Host (shj serve) the copy and the URL go to the GUI, not to this machine.
func TestHostGetsClipboardAndURL(t *testing.T) {
	a := newTestApp(t, "x\n")
	h := &fakeHost{}
	a.SetHost(h)
	old := OpenExternal
	OpenExternal = func(u string) error { t.Fatalf("opened %q on the server", u); return nil }
	defer func() { OpenExternal = old }()

	a.CopyText("hello")
	if len(h.clip) != 1 || h.clip[0] != "hello" || a.Clipboard != "hello" {
		t.Fatalf("clip %v, register %q", h.clip, a.Clipboard)
	}
	a.Update(runOf(t, a.ExecCommand("open-url https://a.example")))
	if len(h.url) != 1 || h.url[0] != "https://a.example" || a.MsgErr {
		t.Fatalf("url %v err %v", h.url, a.MsgErr)
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
