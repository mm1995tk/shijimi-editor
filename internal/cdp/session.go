// Package cdp keeps one headless Chromium running and talks to it over the
// Chrome DevTools Protocol (via chromedp). It streams the page as screencast
// frames and forwards mouse and keyboard input, which is what lets the
// browser view play video. One-shot screenshots stay in package render.
package cdp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/mm1995tk/gh-editor/internal/render"
)

// DefaultFPS is the most frames per second sent to OnFrame. A terminal has to
// receive every frame as PNG data, so the limit keeps CPU and bandwidth (SSH)
// bounded.
const DefaultFPS = 15

// State is what the UI shows about the page.
type State struct {
	URL, Title string
	Loading    bool
}

// Config describes a session. OnFrame and OnState are called from background
// goroutines.
type Config struct {
	Chromium      string // executable; "" = search PATH
	Width, Height int    // viewport in pixels
	FPS           int    // frame rate limit; 0 = DefaultFPS
	OnFrame       func(png []byte)
	OnState       func(State)
}

// Session is a running Chromium with one page.
type Session struct {
	cfg    Config
	dir    string
	cancel []context.CancelFunc
	ctx    context.Context
	ops    chan func(context.Context)
	wake   chan struct{}
	done   chan struct{}

	mu        sync.Mutex
	state     State
	mainFrame cdp.FrameID
	w, h      int
	streaming bool
	latest    []byte // newest frame not yet sent
	last      []byte // newest frame sent, for Redraw
	closeOnce sync.Once
}

// Start launches Chromium. The sandbox is never disabled; if Chromium cannot
// start with it, render.ErrSandbox is returned. Start blocks until the browser
// is up, so call it from a background goroutine.
func Start(cfg Config) (*Session, error) {
	bin, err := render.Chromium{Path: cfg.Chromium}.Find()
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 {
		cfg.Width = 1280
	}
	if cfg.Height <= 0 {
		cfg.Height = 800
	}
	if cfg.FPS <= 0 {
		cfg.FPS = DefaultFPS
	}
	dir, err := os.MkdirTemp("", "gh-editor-cdp-")
	if err != nil {
		return nil, err
	}
	s := &Session{
		cfg: cfg, dir: dir, w: cfg.Width, h: cfg.Height,
		ops:  make(chan func(context.Context), 256),
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	opts := append(append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...),
		chromedp.ExecPath(bin),
		chromedp.UserDataDir(filepath.Join(dir, "profile")),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("disable-crash-reporter", true),
		chromedp.Flag("autoplay-policy", "no-user-gesture-required"),
		chromedp.DisableGPU,
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	s.ctx = ctx
	s.cancel = []context.CancelFunc{cancelCtx, cancelAlloc}

	if err := chromedp.Do(ctx); err != nil { // starts the browser and opens the tab
		s.Close()
		return nil, classify(err)
	}
	// Subscribe before anything can happen on the page.
	listen(s, chromedp.Events(ctx, page.ScreencastFrame), s.onFrame)
	listen(s, chromedp.Events(ctx, page.FrameNavigated), s.onNavigated)
	listen(s, chromedp.Events(ctx, page.FrameStartedLoading), func(ev page.EventFrameStartedLoading) {
		if s.isMain(ev.FrameID) {
			s.update(func(st *State) { st.Loading = true })
		}
	})
	listen(s, chromedp.Events(ctx, page.FrameStoppedLoading), func(ev page.EventFrameStoppedLoading) {
		if s.isMain(ev.FrameID) {
			s.update(func(st *State) { st.Loading = false })
			s.refreshTitle()
		}
	})
	listen(s, chromedp.Events(ctx, page.NavigatedWithinDocument), func(ev page.EventNavigatedWithinDocument) {
		if s.isMain(ev.FrameID) {
			s.update(func(st *State) { st.URL = ev.URL })
			s.refreshTitle()
		}
	})
	listen(s, chromedp.BrowserEvents(ctx, target.TargetInfoChanged), func(ev target.EventTargetInfoChanged) {
		if ev.TargetInfo != nil && ev.TargetInfo.Type == "page" && ev.TargetInfo.TargetID == chromedp.FromContext(ctx).Target.TargetID {
			s.update(func(st *State) { st.Title = ev.TargetInfo.Title })
		}
	})
	if _, err := chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
		Width: int64(s.w), Height: int64(s.h), DeviceScaleFactor: 1,
	}); err != nil {
		s.Close()
		return nil, classify(err)
	}
	go s.worker()
	go s.pump()
	return s, nil
}

// listen calls fn for each event until the session ends.
func listen[E any](s *Session, events func(yield func(E, error) bool), fn func(E)) {
	go func() {
		for ev, err := range events {
			if err != nil {
				return
			}
			fn(ev)
		}
	}()
}

// classify turns a start-up failure into one of render's sentinel errors when
// it recognises the cause.
func classify(err error) error {
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "sandbox") || strings.Contains(low, "failed to move to new namespace") {
		return fmt.Errorf("%w: %s", render.ErrSandbox, lastLine(err.Error()))
	}
	return fmt.Errorf("Chromium を起動できません: %w", err)
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// worker runs operations one at a time, in order, so that input events reach
// the page in the order they were made.
func (s *Session) worker() {
	for {
		select {
		case <-s.done:
			return
		case fn := <-s.ops:
			fn(s.ctx)
		}
	}
}

// do queues an operation. When the queue is full (Chromium is stuck) the
// operation is dropped rather than blocking the UI.
func (s *Session) do(fn func(context.Context)) {
	select {
	case s.ops <- fn:
	default:
	}
}

// send queues one protocol command and ignores its result.
func send[P, R any](s *Session, cmd cdp.Command[P, R], params P) {
	s.do(func(ctx context.Context) { _, _ = chromedp.Call(ctx, cmd, params) })
}

func (s *Session) onFrame(ev page.EventScreencastFrame) {
	sid := ev.SessionID
	go func() {
		_, _ = chromedp.Call(s.ctx, page.ScreencastFrameAck, page.ScreencastFrameAckParams{SessionID: sid})
	}()
	s.mu.Lock()
	s.latest = ev.Data
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Session) onNavigated(ev page.EventFrameNavigated) {
	if ev.Frame == nil || ev.Frame.ParentID != "" {
		return
	}
	s.mu.Lock()
	s.mainFrame = ev.Frame.ID
	s.mu.Unlock()
	s.update(func(st *State) { st.URL = ev.Frame.URL })
	s.restartScreencast()
}

func (s *Session) isMain(id cdp.FrameID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mainFrame == "" || s.mainFrame == id
}

func (s *Session) update(f func(*State)) {
	s.mu.Lock()
	old := s.state
	f(&s.state)
	st := s.state
	s.mu.Unlock()
	if st != old && s.cfg.OnState != nil {
		s.cfg.OnState(st)
	}
}

// State returns the current page state.
func (s *Session) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// pump hands the newest frame to OnFrame, at most FPS times a second. Frames
// that arrive while the previous one is still being sent are dropped, so a
// slow terminal makes the video skip instead of lag.
func (s *Session) pump() {
	interval := time.Second / time.Duration(s.cfg.FPS)
	var sent time.Time
	for {
		select {
		case <-s.done:
			return
		case <-s.wake:
		}
		if d := interval - time.Since(sent); d > 0 {
			select {
			case <-s.done:
				return
			case <-time.After(d):
			}
		}
		s.mu.Lock()
		f := s.latest
		s.latest = nil
		if f != nil {
			s.last = f
		}
		s.mu.Unlock()
		if f != nil && s.cfg.OnFrame != nil {
			s.cfg.OnFrame(f)
			sent = time.Now()
		}
	}
}

// Redraw sends the newest frame again, for example after the tab was hidden.
func (s *Session) Redraw() {
	s.mu.Lock()
	if s.latest == nil {
		s.latest = s.last
	}
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Navigate loads url without waiting for the page to finish.
func (s *Session) Navigate(url string) {
	s.update(func(st *State) { st.URL, st.Loading = url, true })
	send(s, page.Navigate, page.NavigateParams{URL: url})
}

// Reload reloads the page.
func (s *Session) Reload() { send(s, page.Reload, page.ReloadParams{}) }

// Back and Forward move through the navigation history.
func (s *Session) Back()    { s.history(-1) }
func (s *Session) Forward() { s.history(1) }

func (s *Session) history(delta int) {
	s.do(func(ctx context.Context) {
		h, err := chromedp.Call(ctx, page.GetNavigationHistory, cdp.Empty{})
		if err != nil {
			return
		}
		i := int(h.CurrentIndex) + delta
		if i < 0 || i >= len(h.Entries) {
			return
		}
		_, _ = chromedp.Call(ctx, page.NavigateToHistoryEntry, page.NavigateToHistoryEntryParams{EntryID: h.Entries[i].ID})
	})
}

// Size returns the viewport in pixels.
func (s *Session) Size() (w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w, s.h
}

// Resize changes the viewport.
func (s *Session) Resize(w, h int) {
	s.mu.Lock()
	if w == s.w && h == s.h || w <= 0 || h <= 0 {
		s.mu.Unlock()
		return
	}
	s.w, s.h = w, h
	streaming := s.streaming
	s.mu.Unlock()
	s.do(func(ctx context.Context) {
		_, _ = chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width: int64(w), Height: int64(h), DeviceScaleFactor: 1,
		})
		if streaming {
			_, _ = chromedp.Call(ctx, page.StopScreencast, cdp.Empty{})
			startScreencast(ctx, w, h)
		}
	})
}

func startScreencast(ctx context.Context, w, h int) {
	_, _ = chromedp.Call(ctx, page.StartScreencast, page.StartScreencastParams{
		Format: page.StartScreencastFormatPng, MaxWidth: int64(w), MaxHeight: int64(h), EveryNthFrame: 1,
	})
}

// SetStreaming starts or stops the screencast. Stopping it while the view is
// hidden saves CPU; starting it again makes Chromium paint a fresh frame.
func (s *Session) SetStreaming(on bool) {
	s.mu.Lock()
	if s.streaming == on {
		s.mu.Unlock()
		return
	}
	s.streaming = on
	w, h := s.w, s.h
	s.mu.Unlock()
	s.do(func(ctx context.Context) {
		if on {
			startScreencast(ctx, w, h)
		} else {
			_, _ = chromedp.Call(ctx, page.StopScreencast, cdp.Empty{})
		}
	})
}

// MouseKind is the kind of a Mouse event.
type MouseKind int

const (
	MouseMove MouseKind = iota // moved with the left button held
	MousePress
	MouseRelease
)

// Mouse sends a left-button event at viewport pixel (x, y).
func (s *Session) Mouse(kind MouseKind, x, y float64) {
	p := input.DispatchMouseEventParams{X: x, Y: y}
	switch kind {
	case MousePress:
		p.Type, p.Button, p.Buttons, p.ClickCount = input.DispatchMouseEventTypeMousePressed, input.MouseButtonLeft, 1, 1
	case MouseRelease:
		p.Type, p.Button, p.ClickCount = input.DispatchMouseEventTypeMouseReleased, input.MouseButtonLeft, 1
	default:
		p.Type, p.Buttons = input.DispatchMouseEventTypeMouseMoved, 1
	}
	send(s, input.DispatchMouseEvent, p)
}

// Wheel scrolls the page at viewport pixel (x, y) by dy pixels (positive =
// down).
func (s *Session) Wheel(x, y, dy float64) {
	send(s, input.DispatchMouseEvent, input.DispatchMouseEventParams{
		Type: input.DispatchMouseEventTypeMouseWheel, X: x, Y: y, DeltaY: dy,
	})
}

// Key sends a key press. name is a bubbletea key string ("a", "enter",
// "backspace", "up", ...); keys that have no equivalent are ignored.
func (s *Session) Key(name string) {
	k, ok := KeyFor(name)
	if !ok {
		return
	}
	s.do(func(ctx context.Context) { _ = chromedp.Do(ctx, chromedp.KeyEvent(k)) })
}

// Text inserts text (a paste) into the focused element at once.
func (s *Session) Text(text string) {
	send(s, input.InsertText, input.InsertTextParams{Text: text})
}

var specialKeys = map[string]string{
	"enter": kb.Enter, "backspace": kb.Backspace, "tab": kb.Tab, "esc": kb.Escape,
	"delete": kb.Delete, "up": kb.ArrowUp, "down": kb.ArrowDown, "left": kb.ArrowLeft,
	"right": kb.ArrowRight, "home": kb.Home, "end": kb.End, "pgup": kb.PageUp,
	"pgdown": kb.PageDown, "space": " ",
}

// KeyFor maps a bubbletea key string to the text chromedp's KeyEvent takes.
func KeyFor(name string) (string, bool) {
	if k, ok := specialKeys[name]; ok {
		return k, true
	}
	if r := []rune(name); len(r) == 1 {
		return name, true
	}
	return "", false
}

// ScrollToEdge jumps to the top or the bottom of the page.
func (s *Session) ScrollToEdge(bottom bool) {
	js := "window.scrollTo(0,0)"
	if bottom {
		js = "window.scrollTo(0,document.documentElement.scrollHeight)"
	}
	send(s, runtime.Evaluate, runtime.EvaluateParams{Expression: js})
}

// Close shuts Chromium down and removes its profile.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		for _, c := range s.cancel {
			c()
		}
		_ = os.RemoveAll(s.dir)
	})
}

// refreshTitle reads document.title. The target-info event does not follow
// every title change, so the title is also read when a page finishes loading
// or navigates within the document.
func (s *Session) refreshTitle() {
	s.do(func(ctx context.Context) {
		if title, err := chromedp.Run(ctx, chromedp.Evaluate[string]("document.title")); err == nil {
			s.update(func(st *State) { st.Title = title })
		}
	})
}

// restartScreencast restarts the screencast when it is on. A screencast that
// was started while a navigation was under way can stay silent after the
// navigation commits; starting it again makes Chromium send a fresh frame.
func (s *Session) restartScreencast() {
	s.mu.Lock()
	on, w, h := s.streaming, s.w, s.h
	s.mu.Unlock()
	if !on {
		return
	}
	s.do(func(ctx context.Context) {
		_, _ = chromedp.Call(ctx, page.StopScreencast, cdp.Empty{})
		startScreencast(ctx, w, h)
	})
}
