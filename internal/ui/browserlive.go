package ui

import (
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/cdp"
	"github.com/mm1995tk/gh-editor/internal/kitty"
)

// On a terminal that draws Kitty graphics the browser view is "live": a
// resident Chromium (package cdp) streams its screen, the frames are sent to
// the terminal as images, and the view itself only holds Unicode placeholder
// cells that the terminal replaces with the image. Mouse and keyboard input
// go back to the page. Other terminals keep the screenshot view.

const (
	// Cells assumed when the terminal does not report its pixel size.
	defaultCellW, defaultCellH = 10, 20
	// Upper bound of the page viewport. Every frame is sent to the terminal as
	// a PNG, so a huge window must not mean huge frames.
	maxViewportW, maxViewportH = 1600, 1000
)

// liveSession is the part of cdp.Session the UI uses (tests replace it).
type liveSession interface {
	Navigate(url string)
	Reload()
	Back()
	Forward()
	Resize(w, h int)
	Size() (w, h int)
	SetStreaming(on bool)
	Redraw()
	Mouse(kind cdp.MouseKind, x, y float64)
	Wheel(x, y, dy float64)
	Key(name string)
	Text(s string)
	ScrollToEdge(bottom bool)
	State() cdp.State
	Close()
}

// startSession launches the browser; tests replace it.
var startSession = func(cfg cdp.Config) (liveSession, error) {
	s, err := cdp.Start(cfg)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// kittyGfx is what the app needs to draw images: the terminal output shared
// with bubbletea, and a way to post messages to the program.
type kittyGfx struct {
	out     *kitty.Writer
	send    func(tea.Msg)
	cellW   int
	cellH   int
	nextID  uint32
	closing sync.WaitGroup // sessions being shut down in the background
}

// EnableKitty turns on the live browser view. out must be the writer given to
// tea.WithOutput, so images and screen updates are never interleaved; send is
// the program's Send.
func (a *App) EnableKitty(out *kitty.Writer, send func(tea.Msg)) {
	a.gfx = &kittyGfx{out: out, send: send, cellW: defaultCellW, cellH: defaultCellH}
}

// liveView is the live state of one browserView.
type liveView struct {
	gfx      *kittyGfx
	id       uint32
	sess     liveSession
	starting bool
	pending  string // URL to open once the session is up
	err      error
	input    bool // keys go to the page (Esc leaves)
	dragging bool
	shown    bool // the last sync found this view in front

	mu         sync.Mutex // guards cols, rows and closed (read by the frame goroutine)
	cols, rows int
	closed     bool
}

func (l *liveView) size() (cols, rows int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cols, l.rows
}

// onFrame runs on the session's goroutine: it sends the frame to the terminal.
func (l *liveView) onFrame(png []byte) {
	l.mu.Lock()
	cols, rows, closed := l.cols, l.rows, l.closed
	l.mu.Unlock()
	if closed || cols <= 0 || rows <= 0 {
		return
	}
	_ = l.gfx.out.WriteString(kitty.Transmit(l.id, png, cols, rows))
}

// liveMsg reports that the session started (sess) or failed (err).
type liveStartedMsg struct {
	view *browserView
	sess liveSession
	err  error
}

// liveStateMsg only makes the app redraw after the page's title or URL changed.
type liveStateMsg struct{}

func (a *App) newLive() *liveView {
	a.gfx.nextID++
	// Ids are carried in a 24-bit foreground colour; start away from 0.
	return &liveView{gfx: a.gfx, id: 0x100 + a.gfx.nextID}
}

// liveLoad opens the current history entry in the live session, starting it
// first when needed.
func (a *App) liveLoad(v *browserView) tea.Cmd {
	if v.live == nil {
		v.live = a.newLive()
	}
	l := v.live
	l.err = nil
	if l.sess != nil {
		l.sess.Navigate(v.target())
		return nil
	}
	l.pending = v.target()
	if l.starting {
		return nil
	}
	l.starting = true
	w, h := a.liveViewport()
	cfg := cdp.Config{
		Chromium: a.cfg.Browser.Chromium, // read per start so :config-reload applies
		Width:    w, Height: h,
		OnFrame: l.onFrame,
		// In a goroutine: Navigate reports its state change from inside Update, where
		// Send would wait for the event loop that is waiting for it.
		OnState: func(cdp.State) { go l.gfx.send(liveStateMsg{}) },
	}
	return func() tea.Msg {
		s, err := startSession(cfg)
		return liveStartedMsg{view: v, sess: s, err: err}
	}
}

func (a *App) liveStarted(msg liveStartedMsg) {
	v := msg.view
	l := v.live
	l.starting = false
	if !a.hasBrowserView(v) { // the tab was closed while Chromium started
		if msg.sess != nil {
			go msg.sess.Close()
		}
		return
	}
	if msg.err != nil {
		l.err = msg.err
		a.fail(msg.err)
		return
	}
	l.sess = msg.sess
	l.sess.Navigate(l.pending)
	l.sess.SetStreaming(a.browserTab() == v)
}

func (a *App) hasBrowserView(v *browserView) bool {
	for _, t := range a.tabs {
		if t.kind == tabBrowser && t.web == v {
			return true
		}
	}
	return false
}

// liveGeom returns the image area of the browser view in cells.
func (a *App) liveGeom() (cols, rows int) {
	l := a.layout()
	return min(l.rightW, kitty.MaxCells), min(max(0, l.editH-1), kitty.MaxCells)
}

// liveViewport returns the page viewport in pixels for the current layout. The
// aspect ratio follows the cell grid; the size is capped.
func (a *App) liveViewport() (w, h int) {
	cols, rows := a.liveGeom()
	w, h = max(1, cols)*a.gfx.cellW, max(1, rows)*a.gfx.cellH
	if f := min(1, float64(maxViewportW)/float64(w), float64(maxViewportH)/float64(h)); f < 1 {
		w, h = max(1, int(float64(w)*f)), max(1, int(float64(h)*f))
	}
	return w, h
}

// syncLive keeps every live view in line with the screen: the viewport follows
// the pane size, and only the visible tab streams. It runs after every update.
func (a *App) syncLive() {
	if a.gfx == nil {
		return
	}
	if w, h := kitty.CellSize(os.Stdout); w > 0 && h > 0 {
		a.gfx.cellW, a.gfx.cellH = w, h
	}
	if a.w <= 0 || a.h <= 0 {
		return
	}
	cur := a.browserTab()
	for _, t := range a.tabs {
		if t.kind != tabBrowser || t.web.live == nil {
			continue
		}
		l := t.web.live
		cols, rows := a.liveGeom()
		l.mu.Lock()
		changed := l.cols != cols || l.rows != rows
		l.cols, l.rows = cols, rows
		l.mu.Unlock()
		if l.sess == nil {
			continue
		}
		visible := t.web == cur
		if changed && cols > 0 && rows > 0 {
			l.sess.Resize(a.liveViewport())
		}
		l.sess.SetStreaming(visible)
		if visible && (changed || !l.shown) {
			l.sess.Redraw()
		}
		l.shown = visible
	}
}

// closeLive releases the browser and the terminal's copy of the image.
func (v *browserView) closeLive() {
	l := v.live
	if l == nil {
		return
	}
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	_ = l.gfx.out.WriteString(kitty.Delete(l.id))
	if l.sess != nil {
		l.gfx.closing.Add(1)
		go func() { // waits for Chromium to exit
			defer l.gfx.closing.Done()
			l.sess.Close()
		}()
	}
}

// closeLiveNow is closeLive for program exit: it waits for Chromium to stop.
func (v *browserView) closeLiveNow() {
	l := v.live
	if l == nil {
		return
	}
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	if l.sess != nil {
		l.sess.Close()
	}
}

// renderLive draws the status line and the placeholder grid.
func (v *browserView) renderLive(w, h int) []string {
	l := v.live
	st := cdp.State{}
	if l.sess != nil {
		st = l.sess.State()
	}
	status := "  " + v.url()
	switch {
	case l.err != nil:
		status = "  読み込めません: " + l.err.Error()
	case l.sess == nil:
		status = "  Chromium を起動しています… " + v.url()
	case st.Loading:
		status = "  読み込み中… " + v.url()
	default:
		if st.Title != "" {
			status += "  " + st.Title
		}
	}
	if l.input {
		status += "  [ページへ入力中: Esc で戻る]"
	}
	out := []string{stPanelDim.Render(fit(status, w))}
	blank := stEmpty.Render(strings.Repeat(" ", w))
	cols, rows := l.size()
	if l.sess == nil || cols <= 0 || rows <= 0 {
		for len(out) < h {
			out = append(out, blank)
		}
		return out
	}
	pad := stEmpty.Render(strings.Repeat(" ", max(0, w-cols)))
	for _, line := range kitty.Placeholders(l.id, cols, rows) {
		if len(out) >= h {
			break
		}
		out = append(out, line+pad)
	}
	for len(out) < h {
		out = append(out, blank)
	}
	return out
}

// pagePoint converts a screen position to a page pixel; ok is false outside
// the image area.
func (a *App) pagePoint(v *browserView, x, y int) (px, py float64, ok bool) {
	l := a.layout()
	cols, rows := v.live.size()
	vw, vh := v.live.sess.Size()
	cx, cy := x-l.rightX, y-(l.editY+1)
	if cx < 0 || cy < 0 || cx >= cols || cy >= rows {
		return 0, 0, false
	}
	return (float64(cx) + 0.5) / float64(cols) * float64(vw), (float64(cy) + 0.5) / float64(rows) * float64(vh), true
}

// liveMouse forwards a mouse event on the live view to the page. It reports
// whether the event was consumed.
func (a *App) liveMouse(v *browserView, msg tea.MouseMsg) (tea.Cmd, bool) {
	l := v.live
	if l == nil || l.sess == nil {
		return nil, false
	}
	px, py, in := a.pagePoint(v, msg.X, msg.Y)
	left := msg.Button == tea.MouseButtonLeft
	switch msg.Action {
	case tea.MouseActionPress:
		if !in || !left {
			return nil, false
		}
		l.dragging = true
		l.input = true // clicking a search box is followed by typing into it
		l.sess.Mouse(cdp.MousePress, px, py)
		return a.setFocus(focusEditor), true
	case tea.MouseActionRelease:
		if !l.dragging {
			return nil, false
		}
		l.dragging = false
		if !in { // released outside: end the press at the nearest edge
			vw, vh := l.sess.Size()
			px, py = clampF(px, 0, float64(vw-1)), clampF(py, 0, float64(vh-1))
		}
		l.sess.Mouse(cdp.MouseRelease, px, py)
		return nil, true
	case tea.MouseActionMotion:
		if !l.dragging {
			return nil, false
		}
		if in {
			l.sess.Mouse(cdp.MouseMove, px, py)
		}
		return nil, true
	}
	return nil, false
}

func clampF(v, lo, hi float64) float64 { return max(lo, min(hi, v)) }

// liveWheel scrolls the page; wheel is the number of rows (positive = down).
func (a *App) liveWheel(v *browserView, x, y, wheel int) {
	l := v.live
	if l == nil || l.sess == nil {
		return
	}
	px, py, ok := a.pagePoint(v, x, y)
	vw, vh := l.sess.Size()
	if !ok {
		px, py = float64(vw)/2, float64(vh)/2
	}
	// one wheel notch is 3 rows; a notch scrolls 120 px in a browser
	l.sess.Wheel(px, py, float64(wheel)*40)
}

// liveKey handles a key on the live view. Outside the input mode the keys
// scroll and navigate like the screenshot view; "i" / Enter or a click on the
// page switch to input.
func (a *App) liveKey(v *browserView, key string) tea.Cmd {
	l := v.live
	if l.input {
		if key == "esc" {
			l.input = false
		} else if l.sess != nil {
			l.sess.Key(key)
		}
		return nil
	}
	if l.sess == nil {
		return nil
	}
	vw, vh := l.sess.Size()
	cx, cy := float64(vw)/2, float64(vh)/2
	switch key {
	case "i", "enter":
		l.input = true
	case "j", "down":
		l.sess.Wheel(cx, cy, float64(vh)/4)
	case "k", "up":
		l.sess.Wheel(cx, cy, -float64(vh)/4)
	case "pgdown", "ctrl+d", "ctrl+f":
		l.sess.Wheel(cx, cy, float64(vh)/2)
	case "pgup", "ctrl+u", "ctrl+b":
		l.sess.Wheel(cx, cy, -float64(vh)/2)
	case "g", "home":
		l.sess.ScrollToEdge(false)
	case "G", "end":
		l.sess.ScrollToEdge(true)
	}
	return nil
}

// liveNav runs ":browser reload|back|next" on the live session.
func (v *browserView) liveNav(arg string) {
	if v.live == nil || v.live.sess == nil {
		return
	}
	switch arg {
	case "reload":
		v.live.sess.Reload()
	case "back":
		v.live.sess.Back()
	case "next":
		v.live.sess.Forward()
	}
}

// liveKeys are the keys liveKey handles outside the input mode; the others
// (q, :, o, r, [, ] and the Space leader) work as in the screenshot view.
var liveKeys = map[string]bool{
	"i": true, "enter": true, "j": true, "down": true, "k": true, "up": true,
	"pgdown": true, "ctrl+d": true, "ctrl+f": true, "pgup": true, "ctrl+u": true, "ctrl+b": true,
	"g": true, "G": true, "home": true, "end": true,
}
