package ui

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"

	"github.com/mm1995tk/gh-editor/internal/render"
)

type fakeRenderer struct{ urls []string }

func (f *fakeRenderer) Render(_ context.Context, url string, _ render.Options) ([]byte, error) {
	f.urls = append(f.urls, url)
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 100, 1000)))
	return buf.Bytes(), nil
}

func TestBrowserNavigation(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	fr := &fakeRenderer{}
	a.renderer = fr

	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	if v == nil || v.page == nil || v.url() != "https://a.example" {
		t.Fatalf("not loaded: %+v", v)
	}
	runCmd(a, a.execCommand("browser b.example"))
	if v.url() != "https://b.example" || len(v.history) != 2 {
		t.Fatalf("history %v idx %d", v.history, v.idx)
	}
	runCmd(a, a.execCommand("browser back"))
	if v.url() != "https://a.example" {
		t.Fatalf("back: %s", v.url())
	}
	runCmd(a, a.execCommand("browser next"))
	runCmd(a, a.execCommand("browser reload"))
	if len(fr.urls) != 5 || fr.urls[4] != "https://b.example" {
		t.Fatalf("renders %v", fr.urls)
	}
	// new navigation after back drops the forward entries
	runCmd(a, a.execCommand("browser back"))
	runCmd(a, a.execCommand("browser https://c.example"))
	if len(v.history) != 2 || v.url() != "https://c.example" {
		t.Fatalf("history %v", v.history)
	}
	if len(a.tabs) != 2 {
		t.Fatalf("tabs %d", len(a.tabs))
	}
}

func TestBrowserScroll(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	a.renderer = &fakeRenderer{}
	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	if len(v.render(60, 20)) != 20 {
		t.Fatal("render height")
	}
	a.handleBrowserKey(a.cur(), "j")
	if v.scroll <= 0 {
		t.Fatalf("scroll %d", v.scroll)
	}
	a.handleBrowserKey(a.cur(), "G")
	if v.scroll != v.maxScroll(a.layout().rightW, a.layout().editH-1) || v.scroll == 0 {
		t.Fatalf("bottom %d", v.scroll)
	}
	a.handleBrowserKey(a.cur(), "g")
	if v.scroll != 0 {
		t.Fatal("top")
	}
}

func TestBrowserRejectsBadInput(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.renderer = &fakeRenderer{}
	for _, bad := range []string{"--version", "a b", "browser"} {
		a.msgErr = false
		if bad == "browser" {
			runCmd(a, a.execCommand("browser reload"))
		} else {
			runCmd(a, a.execCommand("browser "+bad))
		}
		if !a.msgErr || a.browserTab() != nil {
			t.Fatalf("%q must fail", bad)
		}
	}
}

func TestBrowserScheme(t *testing.T) {
	for in, want := range map[string]string{
		"localhost:3000":      "http://",
		"localhost":           "http://",
		"127.0.0.1:8080/x":    "http://",
		"[::1]:3000":          "http://",
		"example.com":         "https://",
		"localhost.evil.test": "https://",
	} {
		if got := browserScheme(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestBrowserReusesTabFromOtherTab(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	a.renderer = &fakeRenderer{}
	runCmd(a, a.execCommand("browser localhost:3000"))
	if a.browserTab().url() != "http://localhost:3000" {
		t.Fatalf("url %s", a.browserTab().url())
	}
	a.active = 0 // text tab
	runCmd(a, a.execCommand("browser https://b.example"))
	if len(a.tabs) != 2 || a.browserTab() == nil || len(a.browserTab().history) != 2 {
		t.Fatalf("tabs %d", len(a.tabs))
	}
	a.active = 0
	runCmd(a, a.execCommand("browser reload"))
	if a.msgErr {
		t.Fatal("reload from text tab failed")
	}
}

func TestBrowserReloadKeepsScroll(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	a.renderer = &fakeRenderer{}
	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	a.handleBrowserKey(a.cur(), "G")
	want := v.scroll
	runCmd(a, a.execCommand("browser reload"))
	if v.scroll != want {
		t.Fatalf("scroll %d, want %d", v.scroll, want)
	}
	a.browserWheel(v, -1)
	if v.scroll >= want {
		t.Fatal("wheel did not scroll")
	}
}

func TestBrowserStaleAndError(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	a.renderer = &fakeRenderer{}
	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	old := v.page
	v.seq, v.loading = 5, true
	a.browserLoaded(browserLoadedMsg{view: v, seq: 4, err: errors.New("stale")})
	if !v.loading || v.err != nil || v.page != old || a.msgErr {
		t.Fatal("stale result must be ignored")
	}
	a.browserLoaded(browserLoadedMsg{view: v, seq: 5, err: errors.New("boom")})
	if v.loading || v.err == nil || !a.msgErr {
		t.Fatal("error not shown")
	}
}
