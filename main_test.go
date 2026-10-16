package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

// TestKittyGraphicsOnPTY runs the real program on a pseudo terminal that
// announces Kitty and checks that the browser view sends an image and draws
// placeholder cells. It needs Chromium (GH_EDITOR_TEST_CHROMIUM).
func TestKittyGraphicsOnPTY(t *testing.T) {
	chromium := os.Getenv("GH_EDITOR_TEST_CHROMIUM")
	if chromium == "" {
		t.Skip("GH_EDITOR_TEST_CHROMIUM が未設定です")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><title>pty</title><body style="background:#c33">hello`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	bin := filepath.Join(dir, "gh-editor")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, dir)
	// Chromium is found through PATH, as for a user without a [browser] setting.
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(chromium)+":"+os.Getenv("PATH"),
		"TERM=xterm-kitty", "KITTY_WINDOW_ID=1")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120, X: 1200, Y: 800})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Quit through the UI so that Chromium is stopped and its profile removed.
		io.WriteString(f, "q") // close the browser tab
		time.Sleep(300 * time.Millisecond)
		io.WriteString(f, ":q\r")
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		f.Close()
	}()

	var mu sync.Mutex
	var got bytes.Buffer
	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, err := f.Read(buf)
			mu.Lock()
			got.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	has := func(s string) bool {
		mu.Lock()
		defer mu.Unlock()
		return bytes.Contains(got.Bytes(), []byte(s))
	}
	wait := func(what, s string) {
		t.Helper()
		for deadline := time.Now().Add(30 * time.Second); !has(s); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				mu.Lock()
				defer mu.Unlock()
				t.Fatalf("タイムアウト: %s (出力 %d バイト)\n%s", what, got.Len(), ansi.Strip(got.String()))
			}
		}
	}

	wait("画面の描画", "\x1b[?1049h") // alternate screen: bubbletea saw a terminal
	time.Sleep(500 * time.Millisecond)
	io.WriteString(f, ":browser "+srv.URL+"\r")
	wait("画像の送信", "\x1b_Ga=T,U=1,f=100,i=")
	wait("プレースホルダー", "\U0010EEEE")
	// 120 columns minus the side bar, 38 rows minus tab bar, status line and status bar
	if !has(",c=") || !has(",r=") {
		t.Error("image size missing from the transmission")
	}
}
