package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestRenderWaitsForScripts(t *testing.T) {
	bin := fakeChromium(t, `ok=; for a in "$@"; do [ "$a" = --virtual-time-budget=5000 ] && ok=1; done
[ -n "$ok" ] || { echo "no virtual-time-budget" >&2; exit 1; }
for a in "$@"; do case "$a" in --screenshot=*) printf '\211PNG\r\n\032\nxx' > "${a#--screenshot=}";; esac; done`)
	if _, err := (Chromium{Path: bin}).Render(context.Background(), "https://youtube.com", Options{}); err != nil {
		t.Fatal(err)
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

func TestTimeoutWithLingeringChild(t *testing.T) {
	// the child inherits stderr and outlives the parent, like Chromium's helpers
	bin := fakeChromium(t, "sleep 20 &\nsleep 20")
	start := time.Now()
	_, err := Chromium{Path: bin}.Render(context.Background(), "http://x", Options{Timeout: 300 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "タイムアウト") {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("Render took %s", d)
	}
}

func TestNoSandboxFlagAndSandboxError(t *testing.T) {
	bin := fakeChromium(t, `for a in "$@"; do [ "$a" = --no-sandbox ] && { echo "no-sandbox given" >&2; exit 2; }; done
echo "No usable sandbox!" >&2; exit 1`)
	_, err := Chromium{Path: bin}.Render(context.Background(), "http://x", Options{})
	if !errors.Is(err, ErrSandbox) || !strings.Contains(err.Error(), ":open-url") || strings.Contains(err.Error(), "no-sandbox given") {
		t.Fatalf("err = %v", err)
	}
}

func TestSandboxErrorNamespaceMessage(t *testing.T) {
	bin := fakeChromium(t, `echo "Failed to move to new namespace: PID namespaces supported, Network namespace supported, but failed: errno = Operation not permitted" >&2; exit 1`)
	_, err := Chromium{Path: bin}.Render(context.Background(), "http://x", Options{})
	if !errors.Is(err, ErrSandbox) {
		t.Fatalf("err = %v", err)
	}
}
