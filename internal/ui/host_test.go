package ui

import "testing"

type fakeHost struct{ clip, url []string }

func (h *fakeHost) WriteClipboard(s string) { h.clip = append(h.clip, s) }
func (h *fakeHost) OpenURL(u string)        { h.url = append(h.url, u) }

// With a Host (shj serve) the copy and the URL go to the GUI, not to this machine.
func TestHostGetsClipboardAndURL(t *testing.T) {
	a := newTestApp(t, "x\n")
	h := &fakeHost{}
	a.SetHost(h)
	old := openExternal
	openExternal = func(u string) error { t.Fatalf("opened %q on the server", u); return nil }
	defer func() { openExternal = old }()

	a.copyText("hello")
	if len(h.clip) != 1 || h.clip[0] != "hello" || a.clipboard != "hello" {
		t.Fatalf("clip %v, register %q", h.clip, a.clipboard)
	}
	runCmd(a, a.execCommand("open-url https://a.example"))
	if len(h.url) != 1 || h.url[0] != "https://a.example" || a.msgErr {
		t.Fatalf("url %v err %v", h.url, a.msgErr)
	}
}
