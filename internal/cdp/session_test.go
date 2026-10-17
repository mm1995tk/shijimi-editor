package cdp

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mm1995tk/gh-editor/internal/render"
)

func TestKeyFor(t *testing.T) {
	for _, name := range []string{"a", "Z", "あ", "enter", "backspace", "up", "pgdown", "space"} {
		if _, ok := KeyFor(name); !ok {
			t.Errorf("KeyFor(%q) not mapped", name)
		}
	}
	for _, name := range []string{"ctrl+a", "f13", "alt+left", ""} {
		if _, ok := KeyFor(name); ok {
			t.Errorf("KeyFor(%q) should be unmapped", name)
		}
	}
}

// startForTest launches a real Chromium, or skips when there is none (or it
// cannot run with its sandbox in this environment).
func startForTest(t *testing.T, cfg Config) *Session {
	t.Helper()
	if _, err := (render.Chromium{Path: os.Getenv("GH_EDITOR_TEST_CHROMIUM")}).Find(); err != nil {
		t.Skip("Chromium がありません")
	}
	cfg.Chromium = os.Getenv("GH_EDITOR_TEST_CHROMIUM")
	s, err := Start(cfg)
	if errors.Is(err, render.ErrSandbox) {
		t.Skipf("この環境では sandbox 付きで起動できません: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("タイムアウト: %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

const testPage = `<!doctype html><title>start</title>
<body style="margin:0"><button id=b style="position:absolute;left:100px;top:100px;width:200px;height:100px"
 onclick="location.hash='clicked'">go</button>
<div style="height:5000px"></div>
<script>setInterval(()=>{document.body.style.background='hsl('+(Date.now()/10%360)+',80%,50%)'},30)</script>`

func TestSessionLive(t *testing.T) {
	var frames atomic.Int64
	var lastFrame atomic.Value
	s := startForTest(t, Config{
		Width: 640, Height: 400,
		OnFrame: func(jpeg []byte) { frames.Add(1); lastFrame.Store(jpeg) },
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testPage)
	}))
	defer srv.Close()

	s.SetStreaming(true)
	s.Navigate(srv.URL)
	waitFor(t, "title=start", func() bool { return s.State().Title == "start" })
	if st := s.State(); !strings.HasPrefix(st.URL, srv.URL) {
		t.Errorf("URL = %q", st.URL)
	}
	// the page repaints all the time, so frames keep coming
	waitFor(t, "frames", func() bool { return frames.Load() >= 3 })
	if f, _ := lastFrame.Load().([]byte); !bytes.HasPrefix(f, []byte("\xff\xd8")) {
		t.Errorf("frame is not a JPEG")
	}

	// a click is delivered to the page
	s.Mouse(MousePress, 150, 150)
	s.Mouse(MouseRelease, 150, 150)
	waitFor(t, "hash=clicked", func() bool { return strings.HasSuffix(s.State().URL, "#clicked") })

	// the frame rate limit holds
	before := frames.Load()
	time.Sleep(time.Second)
	if n := frames.Load() - before; n > int64(DefaultFPS)+3 {
		t.Errorf("%d frames in one second, limit is %d", n, DefaultFPS)
	}

	// stopping the screencast stops the frames
	s.SetStreaming(false)
	time.Sleep(500 * time.Millisecond)
	before = frames.Load()
	time.Sleep(500 * time.Millisecond)
	if n := frames.Load() - before; n > 1 {
		t.Errorf("%d frames after SetStreaming(false)", n)
	}
}
