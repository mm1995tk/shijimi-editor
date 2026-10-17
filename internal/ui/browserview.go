package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/render"
)

// Web pages are captured as one tall screenshot; scrolling moves a window
// over it. Both values can be changed here without touching the rest.
const (
	browserPageW = 1280
	browserPageH = 3200
)

// browserView shows a web page rendered to an image, with history.
type browserView struct {
	history []string
	idx     int

	page       image.Image // full capture of history[idx]
	size       int64
	scroll     int // top of the window, in page pixels
	loading    bool
	err        error
	seq        int  // incremented per load; stale results are dropped
	keepScroll bool // the next load is a reload: keep the scroll position

	live *liveView // non-nil on a terminal with Kitty graphics

	shown *imageView // window currently displayed
	shW   int        // cache key for shown
	shH   int
	shScr int
}

func (v *browserView) url() string {
	if v.live != nil && v.live.sess != nil {
		if u := v.live.sess.State().URL; u != "" {
			return u
		}
	}
	if v.idx < len(v.history) {
		return v.history[v.idx]
	}
	return ""
}

// target is the URL the user asked for (the current history entry), as opposed
// to url(), which prefers the address the live page has reached.
func (v *browserView) target() string {
	if v.idx < len(v.history) {
		return v.history[v.idx]
	}
	return ""
}

func (v *browserView) name() string {
	if u, err := url.Parse(v.url()); err == nil && u.Host != "" {
		return u.Host
	}
	return "browser"
}

// windowH is the height of the visible window in page pixels for a view of
// w columns by body rows (one cell = 1 px wide, 2 px tall).
func (v *browserView) windowH(w, body int) int {
	if v.page == nil || w <= 0 {
		return 0
	}
	return max(1, v.page.Bounds().Dx()*body*2/w)
}

func (v *browserView) maxScroll(w, body int) int {
	if v.page == nil {
		return 0
	}
	return max(0, v.page.Bounds().Dy()-v.windowH(w, body))
}

func (v *browserView) render(w, h int) []string {
	if v.live != nil {
		return v.renderLive(w, h)
	}
	body := h - 1
	status := "  " + v.url()
	switch {
	case v.loading:
		status = "  読み込み中… " + v.url()
	case v.err != nil:
		status = "  読み込めません: " + v.err.Error()
	case v.page != nil:
		status += fmt.Sprintf("  (%d/%d)", v.idx+1, len(v.history))
	}
	out := []string{stPanelDim.Render(fit(status, w))}
	if v.page == nil || body <= 0 {
		blank := stEmpty.Render(strings.Repeat(" ", w))
		for len(out) < h {
			out = append(out, blank)
		}
		return out
	}
	v.scroll = clamp(v.scroll, 0, v.maxScroll(w, body))
	if v.shown == nil || v.shW != w || v.shH != h || v.shScr != v.scroll {
		b := v.page.Bounds()
		win := image.Rect(b.Min.X, b.Min.Y+v.scroll, b.Max.X, min(b.Max.Y, b.Min.Y+v.scroll+v.windowH(w, body)))
		sub := v.page
		if s, ok := v.page.(interface {
			SubImage(image.Rectangle) image.Image
		}); ok {
			sub = s.SubImage(win)
		}
		v.shown = &imageView{path: v.url(), img: sub, format: "png", size: v.size}
		v.shW, v.shH, v.shScr = w, h, v.scroll
	}
	// drop the image view's own info line; the status line replaces it
	return append(out, v.shown.render(w, h)[1:]...)
}

// browserLoadedMsg carries the result of one render.
type browserLoadedMsg struct {
	view       *browserView
	seq        int
	data       []byte
	err        error
	keepScroll bool
}

// load renders the current history entry in the background.
func (a *App) browserLoad(v *browserView) tea.Cmd {
	if a.gfx != nil {
		return a.liveLoad(v)
	}
	v.seq++
	v.loading, v.err = true, nil
	seq, target := v.seq, v.url()
	keep := v.keepScroll // consumed here so a failed or stale load cannot leak it
	v.keepScroll = false
	var r render.Renderer = render.Chromium{Path: a.cfg.Browser.Chromium} // read per load so :config-reload applies
	if a.renderer != nil {
		r = a.renderer
	}
	return func() tea.Msg {
		data, err := r.Render(context.Background(), target, render.Options{Width: browserPageW, Height: browserPageH})
		return browserLoadedMsg{view: v, seq: seq, data: data, err: err, keepScroll: keep}
	}
}

func (a *App) browserLoaded(msg browserLoadedMsg) {
	v := msg.view
	if msg.seq != v.seq {
		return
	}
	v.loading = false
	if msg.err != nil {
		v.err = msg.err
		a.fail(msg.err)
		return
	}
	img, _, err := image.Decode(bytes.NewReader(msg.data))
	if err != nil {
		v.err = err
		a.fail(err)
		return
	}
	v.page, v.size, v.shown = img, int64(len(msg.data)), nil
	if !msg.keepScroll {
		v.scroll = 0
	}
}

var errNoBrowserTab = errors.New("ブラウザのタブがありません（:browser <URL> で開けます）")

// browserScheme is the scheme added to a URL typed without one: dev servers
// on the local machine speak plain HTTP, everything else HTTPS.
func browserScheme(arg string) string {
	host := arg
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.HasSuffix(host, "]") {
		host = host[:i]
	}
	switch host {
	case "localhost", "127.0.0.1", "[::1]":
		return "http://"
	}
	return "https://"
}

// browserTab returns the active browser tab's view, if any.
func (a *App) browserTab() *browserView {
	if t := a.cur(); t != nil && t.kind == tabBrowser {
		return t.web
	}
	return nil
}

// findBrowserTab returns the browser view, activating its tab if another tab
// is in front.
func (a *App) findBrowserTab() *browserView {
	if v := a.browserTab(); v != nil {
		return v
	}
	for i, t := range a.tabs {
		if t.kind == tabBrowser {
			a.active = i
			a.follow = true
			return t.web
		}
	}
	return nil
}

// browserCmd handles ":browser <url|reload|back|next>".
func (a *App) browserCmd(arg string) tea.Cmd {
	arg = strings.TrimSpace(arg)
	switch arg {
	case "":
		a.fail(errors.New("URL を指定してください（:browser <URL>）"))
		return nil
	case "reload", "back", "next":
		v := a.findBrowserTab()
		if v == nil {
			a.fail(errNoBrowserTab)
			return nil
		}
		if a.gfx != nil {
			v.liveNav(arg)
			return nil
		}
		switch arg {
		case "reload":
			v.keepScroll = true
		case "back":
			if v.idx == 0 {
				a.info("これ以上戻れません")
				return nil
			}
			v.idx--
		case "next":
			if v.idx+1 >= len(v.history) {
				a.info("これ以上進めません")
				return nil
			}
			v.idx++
		}
		return a.browserLoad(v)
	}
	if !urlFullRe.MatchString(arg) {
		if strings.ContainsAny(arg, " \t") || strings.HasPrefix(arg, "-") {
			a.fail(fmt.Errorf("http:// または https:// で始まる URL を 1 つだけ指定してください: %s", arg))
			return nil
		}
		arg = browserScheme(arg) + arg
	}
	v := a.findBrowserTab()
	if v == nil {
		v = &browserView{}
		a.addTab(&tab{kind: tabBrowser, web: v})
		v.history = []string{arg}
	} else {
		v.history = append(v.history[:v.idx+1], arg)
		v.idx++
	}
	return a.browserLoad(v)
}

// browserWheel scrolls by wheel * 3 rows of the view.
func (a *App) browserWheel(v *browserView, x, y, wheel int) {
	if v.live != nil {
		a.liveWheel(v, x, y, wheel)
		return
	}
	l := a.layout()
	body := l.editH - 1
	if body <= 0 {
		return
	}
	v.scroll = clamp(v.scroll+wheel*3*v.windowH(l.rightW, body)/body, 0, v.maxScroll(l.rightW, body))
}

// editBrowserURL opens the command line as ":browser <current URL>".
func (a *App) editBrowserURL(v *browserView) tea.Cmd {
	if v.live != nil {
		v.live.input = false
	}
	return a.openCmdline(":browser " + v.url())
}

func (a *App) handleBrowserKey(t *tab, key string) tea.Cmd {
	v := t.web
	if key == "ctrl+l" { // works in the page input mode too, like a browser's address bar
		a.pending = ""
		return a.editBrowserURL(v)
	}
	if v.live != nil && (v.live.input || a.pending != " " && liveKeys[key]) {
		return a.liveKey(v, key)
	}
	l := a.layout()
	w, body := l.rightW, l.editH-1
	page := max(1, v.windowH(w, body))
	switch {
	case a.pending == " ":
		a.pending = ""
		return a.spaceMode(key)
	case key == "q" || key == "esc":
		a.closeTab(true)
	case key == ":":
		return a.openCmdline(":")
	case key == " ":
		a.pending = " "
	case key == "o":
		return a.openCmdline(":browser ")
	case key == "O":
		return a.editBrowserURL(v)
	case key == "r":
		return a.browserCmd("reload")
	case key == "[" || key == "alt+left":
		return a.browserCmd("back")
	case key == "]" || key == "alt+right":
		return a.browserCmd("next")
	case key == "j" || key == "down":
		v.scroll += page / 4
	case key == "k" || key == "up":
		v.scroll -= page / 4
	case key == "pgdown" || key == "ctrl+d" || key == "ctrl+f":
		v.scroll += page / 2
	case key == "pgup" || key == "ctrl+u" || key == "ctrl+b":
		v.scroll -= page / 2
	case key == "g" || key == "home":
		v.scroll = 0
	case key == "G" || key == "end":
		v.scroll = v.maxScroll(w, body)
	}
	v.scroll = clamp(v.scroll, 0, v.maxScroll(w, body))
	return nil
}
