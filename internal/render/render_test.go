package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeChromium writes a script that copies a PNG header to the --screenshot path.
func fakeChromium(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake-chromium")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRenderPNG(t *testing.T) {
	bin := fakeChromium(t, `for a in "$@"; do case "$a" in --screenshot=*) printf '\211PNG\r\n\032\nxx' > "${a#--screenshot=}";; esac; done`)
	var r Renderer = Chromium{Path: bin}
	data, err := r.Render(context.Background(), "http://localhost:3000", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "\x89PNG") {
		t.Fatalf("data = %q", data)
	}
}

func TestNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Chromium{}.Render(context.Background(), "http://x", Options{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	_, err = Chromium{Path: "/nonexistent/chromium"}.Render(context.Background(), "http://x", Options{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("configured path: err = %v", err)
	}
}

func TestFailureAndNonPNG(t *testing.T) {
	bin := fakeChromium(t, "echo boom >&2; exit 1")
	if _, err := (Chromium{Path: bin}).Render(context.Background(), "http://x", Options{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	bin = fakeChromium(t, `for a in "$@"; do case "$a" in --screenshot=*) echo hi > "${a#--screenshot=}";; esac; done`)
	if _, err := (Chromium{Path: bin}).Render(context.Background(), "http://x", Options{}); err == nil {
		t.Fatal("non-PNG output should fail")
	}
}
