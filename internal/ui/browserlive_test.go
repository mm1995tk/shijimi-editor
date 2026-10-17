package ui

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"

	"fmt"
	"github.com/mm1995tk/gh-editor/internal/render"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/cdp"
	"github.com/mm1995tk/gh-editor/internal/kitty"
)

type fakeLive struct {
	calls     []string
	streaming bool
	w, h      int
	state     cdp.State
	closed    chan struct{}
	mouse     []mouseCall
	wheel     [][3]float64
}

type mouseCall struct {
	kind cdp.MouseKind
	x, y float64
}

func (f *fakeLive) rec(s string)         { f.calls = append(f.calls, s) }
func (f *fakeLive) Navigate(u string)    { f.rec("navigate " + u); f.state.URL = u }
func (f *fakeLive) Reload()              { f.rec("reload") }
func (f *fakeLive) Back()                { f.rec("back") }
func (f *fakeLive) Forward()             { f.rec("forward") }
func (f *fakeLive) Resize(w, h int)      { f.w, f.h = w, h }
func (f *fakeLive) Size() (int, int)     { return f.w, f.h }
func (f *fakeLive) SetStreaming(on bool) { f.streaming = on }
func (f *fakeLive) Redraw()              { f.rec("redraw") }
func (f *fakeLive) Key(name string)      { f.rec("key " + name) }
func (f *fakeLive) Text(s string)        { f.rec("text " + s) }
func (f *fakeLive) ScrollToEdge(b bool) {
	f.rec("edge " + map[bool]string{false: "top", true: "bottom"}[b])
}
func (f *fakeLive) State() cdp.State { return f.state }
func (f *fakeLive) Close()           { close(f.closed) }
func (f *fakeLive) Mouse(k cdp.MouseKind, x, y float64) {
	f.mouse = append(f.mouse, mouseCall{k, x, y})
}
func (f *fakeLive) Wheel(x, y, dy float64) { f.wheel = append(f.wheel, [3]float64{x, y, dy}) }

// liveApp returns an app on a "Kitty terminal" whose output goes to a file.
func liveApp(t *testing.T) (*App, *fakeLive, string) {
	t.Helper()
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	out, err := os.Create(filepath.Join(t.TempDir(), "tty"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { out.Close() })
	a.EnableKitty(kitty.NewWriter(out), func(tea.Msg) {})
	f := &fakeLive{w: 1, h: 1, closed: make(chan struct{})}
	old := startSession
	startSession = func(cdp.Config) (liveSession, error) { return f, nil }
	t.Cleanup(func() { startSession = old })
	return a, f, out.Name()
}

func TestLiveBrowserOpens(t *testing.T) {
	a, f, _ := liveApp(t)
	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	if v == nil || v.live == nil || v.live.sess == nil {
		t.Fatalf("live session not started: %+v", v)
	}
	if f.calls[0] != "navigate https://a.example" || !f.streaming {
		t.Errorf("calls %v streaming %v", f.calls, f.streaming)
	}
	// the viewport follows the pane
	l := a.layout()
	cols, rows := l.rightW, l.editH-1
	if c, r := v.live.size(); c != cols || r != rows {
		t.Errorf("grid %dx%d, want %dx%d", c, r, cols, rows)
	}
	if f.w <= 0 || f.h <= 0 || f.w > maxViewportW || f.h > maxViewportH {
		t.Errorf("viewport %dx%d", f.w, f.h)
	}
	if f.w*rows != f.h*cols*a.gfx.cellW/a.gfx.cellH && f.w < maxViewportW && f.h < maxViewportH {
		t.Errorf("viewport %dx%d does not follow the %dx%d cell grid", f.w, f.h, cols, rows)
	}

	// the screen holds placeholders, one per cell of the grid
	view := a.View()
	if n := strings.Count(view, "\U0010EEEE"); n != cols*rows {
		t.Errorf("%d placeholders, want %d", n, cols*rows)
	}
	for i, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > a.w {
			t.Errorf("line %d is %d wide (screen %d)", i, w, a.w)
		}
	}
}

func TestLiveBrowserSendsFrames(t *testing.T) {
	a, _, ttyPath := liveApp(t)
	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	var jpg bytes.Buffer
	_ = jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil)
	v.live.onFrame(jpg.Bytes())
	data, _ := os.ReadFile(ttyPath)
	cols, rows := v.live.size()
	want := fmt.Sprintf("\x1b_Ga=T,U=1,f=24,o=z,s=8,v=8,i=%d,c=%d,r=%d", v.live.id, cols, rows)
	if !strings.Contains(string(data), want) {
		t.Errorf("frame header %q not in output %q", want, data)
	}
}

func TestLiveBrowserInput(t *testing.T) {
	a, f, _ := liveApp(t)
	runCmd(a, a.execCommand("browser https://a.example"))
	v := a.browserTab()
	l := a.layout()
	f.w, f.h = 1000, 500 // as if Chromium reported its viewport

	// a click lands on the pixel under the cell, scaled to the viewport
	cols, rows := v.live.size()
	x, y := l.rightX+cols/2, l.editY+1+rows/2
	a.handleMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	a.handleMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	if len(f.mouse) != 2 || f.mouse[0].kind != cdp.MousePress || f.mouse[1].kind != cdp.MouseRelease {
		t.Fatalf("mouse %+v", f.mouse)
	}
	// the click enters the input mode, so typing and pasting reach the page
	if !v.live.input {
		t.Fatal("a click on the page did not enter the input mode")
	}
	n := len(f.calls)
	keys(a, "j")
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("天気"), Paste: true})
	if got := strings.Join(f.calls[n:], ","); got != "key j,text 天気" {
		t.Errorf("calls after the click %q", got)
	}
	keys(a, "esc")
	wantX := (float64(cols/2) + 0.5) / float64(cols) * 1000
	wantY := (float64(rows/2) + 0.5) / float64(rows) * 500
	if d := f.mouse[0].x - wantX; d > 0.01 || d < -0.01 {
		t.Errorf("x = %v, want %v", f.mouse[0].x, wantX)
	}
	if d := f.mouse[0].y - wantY; d > 0.01 || d < -0.01 {
		t.Errorf("y = %v, want %v", f.mouse[0].y, wantY)
	}

	// a press on the tab bar is not a click on the page
	a.handleMouse(tea.MouseMsg{X: l.rightX + 2, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if len(f.mouse) != 2 {
		t.Errorf("tab bar click reached the page: %+v", f.mouse)
	}

	// the wheel scrolls the page
	a.handleMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if len(f.wheel) != 1 || f.wheel[0][2] <= 0 {
		t.Errorf("wheel %v", f.wheel)
	}

	// j scrolls; i enters the input mode where keys go to the page
	keys(a, "j")
	if len(f.wheel) != 2 {
		t.Errorf("j did not scroll: %v", f.wheel)
	}
	n = len(f.calls)
	keys(a, "i", "a", "enter", "q")
	if got := strings.Join(f.calls[n:], ","); got != "key a,key enter,key q" {
		t.Errorf("calls %q", got)
	}
	if len(a.tabs) != 2 {
		t.Errorf("q in the input mode closed the tab")
	}
	keys(a, "esc")
	if v.live.input {
		t.Errorf("esc did not leave the input mode")
	}
	n = len(f.calls)
	keys(a, "G", "g", "r", "[", "]")
	if got := strings.Join(f.calls[n:], ","); got != "edge bottom,edge top,reload,back,forward" {
		t.Errorf("calls %q", got)
	}
}

func TestLiveBrowserStreamsOnlyWhenVisible(t *testing.T) {
	a, f, _ := liveApp(t)
	runCmd(a, a.execCommand("browser https://a.example"))
	if !f.streaming {
		t.Fatal("not streaming while visible")
	}
	a.active = 0 // back to the text tab
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if f.streaming {
		t.Error("still streaming while another tab is in front")
	}
	a.active = 1
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if !f.streaming {
		t.Error("not streaming after coming back")
	}
}

func TestLiveBrowserClose(t *testing.T) {
	a, f, ttyPath := liveApp(t)
	runCmd(a, a.execCommand("browser https://a.example"))
	id := a.browserTab().live.id
	keys(a, "q")
	select {
	case <-f.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("session not closed")
	}
	data, _ := os.ReadFile(ttyPath)
	if !strings.Contains(string(data), kitty.Delete(id)) {
		t.Errorf("image not deleted: %q", data)
	}
}

func TestStaticBrowserWithoutKitty(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	a.renderer = &fakeRenderer{}
	runCmd(a, a.execCommand("browser https://a.example"))
	if v := a.browserTab(); v == nil || v.live != nil || v.page == nil {
		t.Fatalf("screenshot view expected: %+v", v)
	}
	if strings.Contains(a.View(), "\U0010EEEE") {
		t.Error("placeholders without Kitty graphics")
	}
}

// TestLiveBrowserWithChromium runs the whole path against a real Chromium:
// open a page, receive image data on the terminal output, click a link.
func TestLiveBrowserWithChromium(t *testing.T) {
	bin := os.Getenv("GH_EDITOR_TEST_CHROMIUM")
	if bin == "" {
		t.Skip("GH_EDITOR_TEST_CHROMIUM が未設定です")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><title>t</title><body style="margin:0">
<a id=l href="/next" style="position:absolute;left:0;top:0;width:100%;height:100%;background:#369">link</a>
<script>setInterval(()=>{document.title=String(Date.now()%1000)},50)</script>`)
	}))
	defer srv.Close()

	a := newTestApp(t, "x\n")
	a.w, a.h = 100, 40
	a.cfg.Browser.Chromium = bin
	out, err := os.Create(filepath.Join(t.TempDir(), "tty"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	var posted []tea.Msg
	var mu sync.Mutex
	a.EnableKitty(kitty.NewWriter(out), func(m tea.Msg) { mu.Lock(); posted = append(posted, m); mu.Unlock() })
	defer a.Close()

	runCmd(a, a.execCommand("browser "+srv.URL))
	v := a.browserTab()
	if v == nil || v.live == nil || v.live.sess == nil {
		if v != nil && v.live != nil && v.live.err != nil && errors.Is(v.live.err, render.ErrSandbox) {
			t.Skipf("この環境では sandbox 付きで起動できません: %v", v.live.err)
		}
		t.Fatalf("session not started: %+v", v)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		data, _ := os.ReadFile(out.Name())
		if bytes.Contains(data, []byte("\x1b_Ga=T,U=1,f=24,o=z,s=")) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no image data on the terminal (%d bytes)", len(data))
		}
		time.Sleep(50 * time.Millisecond)
	}

	// click in the middle of the page: the link navigates to /next
	l := a.layout()
	cols, rows := v.live.size()
	x, y := l.rightX+cols/2, l.editY+1+rows/2
	a.handleMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	a.handleMouse(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	for !strings.HasSuffix(v.url(), "/next") {
		if time.Now().After(deadline) {
			t.Fatalf("click did not navigate: %s", v.url())
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(posted) == 0 {
		t.Error("the UI was never told about a state change")
	}
}

func TestLiveBrowserOpensSecondURL(t *testing.T) {
	a, f, _ := liveApp(t)
	runCmd(a, a.execCommand("browser https://a.example"))
	runCmd(a, a.execCommand("browser b.example"))
	var navs []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "navigate ") {
			navs = append(navs, c)
		}
	}
	if got := strings.Join(navs, ","); got != "navigate https://a.example,navigate https://b.example" {
		t.Errorf("navigations %q", got)
	}
}
