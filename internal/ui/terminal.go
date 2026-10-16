package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/mattn/go-runewidth"
)

// vt10x keeps its glyph attribute bits private; these mirror them.
const (
	vtAttrUnderline = 1 << 1
	vtAttrBold      = 1 << 2
	vtAttrItalic    = 1 << 4
)

type termOutputMsg struct{ id int }

type termDoneMsg struct {
	id  int
	err error
}

// termTab is one shell running on its own PTY, drawn through a VT emulator.
type termTab struct {
	id    int
	n     int // number shown in the tab label
	vt    vt10x.Terminal
	ptmx  *os.File
	cmd   *exec.Cmd
	cols  int
	rows  int
	title string
}

// terminal is the bottom panel: a set of PTY-backed login shells.
type terminal struct {
	tabs   []*termTab
	active int
	cwd    string
	nextID int
	nextN  int
	ch     chan tea.Msg
	armed  bool // a wait() command is outstanding
	w, h   int  // size of the body last rendered
	tabX   []int
}

func newTerminal(cwd string) *terminal {
	return &terminal{cwd: cwd, ch: make(chan tea.Msg, 64), w: 80, h: 24}
}

func shell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

func (t *terminal) cur() *termTab {
	if t.active < 0 || t.active >= len(t.tabs) {
		return nil
	}
	return t.tabs[t.active]
}

// focus makes sure a shell exists so the panel is never empty.
func (t *terminal) focus() tea.Cmd {
	if len(t.tabs) == 0 {
		return t.newTab()
	}
	return t.wait()
}

func (t *terminal) blur() {}

// wait returns a command that delivers the next PTY event. Only one is ever
// outstanding; update re-arms it after every event.
func (t *terminal) wait() tea.Cmd {
	if t.armed {
		return nil
	}
	t.armed = true
	ch := t.ch
	return func() tea.Msg { return <-ch }
}

// newTab starts a login shell in the current directory on a fresh PTY.
func (t *terminal) newTab() tea.Cmd {
	cmd := exec.Command(shell(), "-l")
	cmd.Dir = t.cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	cols, rows := max(1, t.w), max(1, t.h)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return func() tea.Msg { return termErrMsg{err} }
	}
	t.nextID++
	t.nextN++
	tab := &termTab{
		id: t.nextID, n: t.nextN, ptmx: ptmx, cmd: cmd, cols: cols, rows: rows,
		vt: vt10x.New(vt10x.WithWriter(ptmx), vt10x.WithSize(cols, rows)),
	}
	t.tabs = append(t.tabs, tab)
	t.active = len(t.tabs) - 1
	go t.pump(tab)
	return t.wait()
}

// termErrMsg reports a failure to start a shell.
type termErrMsg struct{ err error }

// pump copies PTY output into the emulator and wakes the UI.
func (t *terminal) pump(tab *termTab) {
	buf := make([]byte, 32*1024)
	var pending []byte
	for {
		n, err := tab.ptmx.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			var done []byte
			done, pending = splitIncomplete(pending)
			if len(done) > 0 {
				tab.vt.Write(done)
				select {
				case t.ch <- termOutputMsg{id: tab.id}:
				default: // a notification is already queued
				}
			}
			pending = append([]byte(nil), pending...)
		}
		if err != nil {
			break
		}
	}
	werr := tab.cmd.Wait()
	t.ch <- termDoneMsg{id: tab.id, err: werr}
}

// splitIncomplete splits off a trailing partial UTF-8 sequence so that a
// multi-byte character cut by a read boundary is not corrupted.
func splitIncomplete(b []byte) (complete, rest []byte) {
	for i := 1; i <= 3 && i <= len(b); i++ {
		if utf8.RuneStart(b[len(b)-i]) {
			if !utf8.FullRune(b[len(b)-i:]) {
				return b[:len(b)-i], b[len(b)-i:]
			}
			break
		}
	}
	return b, nil
}

func (t *terminal) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case termOutputMsg:
		t.armed = false
		return t.wait()
	case termDoneMsg:
		t.armed = false
		for i, tab := range t.tabs {
			if tab.id == msg.id {
				tab.ptmx.Close()
				t.tabs = append(t.tabs[:i], t.tabs[i+1:]...)
				if t.active > i || t.active >= len(t.tabs) {
					t.active = max(0, t.active-1)
				}
				break
			}
		}
		return t.wait()
	case tea.KeyMsg:
		switch msg.String() {
		case "f2":
			return t.newTab()
		case "ctrl+pgdown":
			t.switchTab(1)
			return nil
		case "ctrl+pgup":
			t.switchTab(-1)
			return nil
		}
		if tab := t.cur(); tab != nil {
			tab.ptmx.Write(termKeyBytes(msg, tab.mode()&vt10x.ModeAppCursor != 0))
		}
	}
	return nil
}

func (t *terminal) switchTab(d int) {
	if n := len(t.tabs); n > 0 {
		t.active = (t.active + d + n) % n
	}
}

// run types a command line into the active shell, as if the user entered it.
func (t *terminal) run(line string) tea.Cmd {
	var cmd tea.Cmd
	if len(t.tabs) == 0 || t.cur().mode()&vt10x.ModeAltScreen != 0 {
		cmd = t.newTab()
	}
	if tab := t.cur(); tab != nil {
		tab.ptmx.Write([]byte(line + "\r"))
	}
	return tea.Batch(cmd, t.wait())
}

// closeAll ends every shell. Closing the PTY master hangs up the session.
func (t *terminal) closeAll() {
	for _, tab := range t.tabs {
		tab.ptmx.Close()
		if tab.cmd.Process != nil {
			tab.cmd.Process.Kill()
		}
	}
}

// clickTab selects the tab drawn at column x of the tab bar.
func (t *terminal) clickTab(x int) {
	for i := 0; i+1 < len(t.tabX) && i < len(t.tabs); i++ {
		if x >= t.tabX[i] && x < t.tabX[i+1] {
			t.active = i
			return
		}
	}
}

func (t *termTab) label() string {
	if s := strings.TrimSpace(t.title); s != "" {
		return s
	}
	return filepath.Base(shell()) + " " + strconv.Itoa(t.n)
}

func (t *terminal) render(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelBord.Render(strings.Repeat("─", w)))

	bar := stPanelTitle.Render(" ") + stPanelTab.Render("TERMINAL")
	x := 9
	t.tabX = t.tabX[:0]
	for i, tab := range t.tabs {
		t.tabX = append(t.tabX, x)
		s := " " + tab.label() + " "
		st := stPanelDim
		if i == t.active {
			st = stPanelCmd
		}
		bar += st.Render(s)
		x += ansi.StringWidth(s)
	}
	t.tabX = append(t.tabX, x)
	bar += stPanelDim.Render(" F2:新規")
	out = append(out, fitStyled(bar, w, stPanel))

	body := h - 2
	if body < 0 {
		return out[:h]
	}
	t.w, t.h = w, body
	tab := t.cur()
	if tab == nil || w < 1 || body < 1 {
		for len(out) < h {
			out = append(out, stPanel.Render(fit("", w)))
		}
		return out[:h]
	}
	if tab.cols != w || tab.rows != body {
		tab.cols, tab.rows = w, body
		pty.Setsize(tab.ptmx, &pty.Winsize{Rows: uint16(body), Cols: uint16(w)})
		tab.vt.Resize(w, body)
	}
	tab.vt.Lock()
	tab.title = tab.vt.Title()
	cur := tab.vt.Cursor()
	showCur := focused && tab.vt.CursorVisible()
	for y := 0; y < body; y++ {
		out = append(out, renderVTRow(tab.vt, y, w, showCur && cur.Y == y, cur.X))
	}
	tab.vt.Unlock()
	return out[:h]
}

// renderVTRow turns one emulator row into an SGR-styled string w cells wide.
func renderVTRow(v vt10x.View, y, w int, withCursor bool, curX int) string {
	var sb strings.Builder
	cols, _ := v.Size()
	type style struct {
		fg, bg vt10x.Color
		mode   int16
		cursor bool
	}
	var prev style
	first := true
	width := 0
	for x := 0; x < cols && x < w; x++ {
		g := v.Cell(x, y)
		s := style{g.FG, g.BG, g.Mode & (vtAttrBold | vtAttrUnderline | vtAttrItalic), withCursor && x == curX}
		if first || s != prev {
			sb.WriteString(sgr(s.fg, s.bg, s.mode, s.cursor))
			prev, first = s, false
		}
		c := g.Char
		if c < ' ' {
			c = ' '
		}
		sb.WriteRune(c)
		width += runewidth.RuneWidth(c)
	}
	if width > w {
		return ansi.Truncate(sb.String(), w, "") + "\x1b[0m"
	}
	sb.WriteString(sgr(vt10x.DefaultFG, vt10x.DefaultBG, 0, false))
	sb.WriteString(strings.Repeat(" ", w-width))
	sb.WriteString("\x1b[0m")
	return sb.String()
}

func sgr(fg, bg vt10x.Color, mode int16, cursor bool) string {
	p := []string{"0"}
	if mode&vtAttrBold != 0 {
		p = append(p, "1")
	}
	if mode&vtAttrItalic != 0 {
		p = append(p, "3")
	}
	if mode&vtAttrUnderline != 0 {
		p = append(p, "4")
	}
	fgp, bgp := sgrColor(fg, true), sgrColor(bg, false)
	if cursor {
		fgp, bgp = sgrColor(bg, true), sgrColor(fg, false)
	}
	p = append(p, fgp, bgp)
	return "\x1b[" + strings.Join(p, ";") + "m"
}

// Default colors of the panel, matching the editor.
var (
	defFG = hexRGB(cEditorFg)
	defBG = hexRGB(cEditorBg)
)

func hexRGB(h string) [3]int {
	v, _ := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	return [3]int{int(v >> 16 & 255), int(v >> 8 & 255), int(v & 255)}
}

func sgrColor(c vt10x.Color, fg bool) string {
	base := 48
	if fg {
		base = 38
	}
	switch {
	case c >= vt10x.DefaultFG: // default fg / bg / cursor
		rgb := defBG
		if fg {
			rgb = defFG
		}
		return fmt.Sprintf("%d;2;%d;%d;%d", base, rgb[0], rgb[1], rgb[2])
	case c < 256:
		return fmt.Sprintf("%d;5;%d", base, c)
	default:
		return fmt.Sprintf("%d;2;%d;%d;%d", base, c>>16&255, c>>8&255, c&255)
	}
}

// fitStyled fits s into w cells and paints the padding with st.
func fitStyled(s string, w int, st interface{ Render(...string) string }) string {
	sw := ansi.StringWidth(s)
	if sw > w {
		return ansi.Truncate(s, w, "")
	}
	return s + st.Render(fmt.Sprintf("%*s", w-sw, ""))
}

// termKeyBytes encodes a key press the way an xterm would send it.
func termKeyBytes(msg tea.KeyMsg, appCursor bool) []byte {
	esc := func(b []byte) []byte {
		if msg.Alt {
			return append([]byte{0x1b}, b...)
		}
		return b
	}
	switch msg.Type {
	case tea.KeyRunes:
		s := string(msg.Runes)
		if msg.Paste {
			s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\r"), "\n", "\r")
		}
		return esc([]byte(s))
	case tea.KeySpace:
		return esc([]byte{' '})
	case tea.KeyBackspace:
		return esc([]byte{0x7f})
	case tea.KeyShiftTab:
		return []byte("\x1b[Z")
	}
	if msg.Type >= 0 && msg.Type < ' ' { // ctrl+letter, tab, enter, esc
		return esc([]byte{byte(msg.Type)})
	}

	mod := 1
	if msg.Alt {
		mod += 2
	}
	// csi builds a cursor-style sequence (ESC [ 1 ; mod F).
	csi := func(f byte, shift, ctrl bool) []byte {
		m := mod
		if shift {
			m++
		}
		if ctrl {
			m += 4
		}
		if m == 1 {
			if appCursor {
				return []byte{0x1b, 'O', f}
			}
			return []byte{0x1b, '[', f}
		}
		return []byte(fmt.Sprintf("\x1b[1;%d%c", m, f))
	}
	// tilde builds an ESC [ n ; mod ~ sequence.
	tilde := func(n int) []byte {
		if mod == 1 {
			return []byte(fmt.Sprintf("\x1b[%d~", n))
		}
		return []byte(fmt.Sprintf("\x1b[%d;%d~", n, mod))
	}
	switch msg.Type {
	case tea.KeyUp:
		return csi('A', false, false)
	case tea.KeyDown:
		return csi('B', false, false)
	case tea.KeyRight:
		return csi('C', false, false)
	case tea.KeyLeft:
		return csi('D', false, false)
	case tea.KeyShiftUp:
		return csi('A', true, false)
	case tea.KeyShiftDown:
		return csi('B', true, false)
	case tea.KeyShiftRight:
		return csi('C', true, false)
	case tea.KeyShiftLeft:
		return csi('D', true, false)
	case tea.KeyCtrlUp:
		return csi('A', false, true)
	case tea.KeyCtrlDown:
		return csi('B', false, true)
	case tea.KeyCtrlRight:
		return csi('C', false, true)
	case tea.KeyCtrlLeft:
		return csi('D', false, true)
	case tea.KeyCtrlShiftUp:
		return csi('A', true, true)
	case tea.KeyCtrlShiftDown:
		return csi('B', true, true)
	case tea.KeyCtrlShiftRight:
		return csi('C', true, true)
	case tea.KeyCtrlShiftLeft:
		return csi('D', true, true)
	case tea.KeyHome:
		return csi('H', false, false)
	case tea.KeyEnd:
		return csi('F', false, false)
	case tea.KeyShiftHome:
		return csi('H', true, false)
	case tea.KeyShiftEnd:
		return csi('F', true, false)
	case tea.KeyCtrlHome:
		return csi('H', false, true)
	case tea.KeyCtrlEnd:
		return csi('F', false, true)
	case tea.KeyInsert:
		return tilde(2)
	case tea.KeyDelete:
		return tilde(3)
	case tea.KeyPgUp:
		return tilde(5)
	case tea.KeyPgDown:
		return tilde(6)
	case tea.KeyF1:
		return []byte("\x1bOP")
	case tea.KeyF2:
		return []byte("\x1bOQ")
	case tea.KeyF3:
		return []byte("\x1bOR")
	case tea.KeyF4:
		return []byte("\x1bOS")
	case tea.KeyF5:
		return tilde(15)
	case tea.KeyF6:
		return tilde(17)
	case tea.KeyF7:
		return tilde(18)
	case tea.KeyF8:
		return tilde(19)
	case tea.KeyF9:
		return tilde(20)
	case tea.KeyF10:
		return tilde(21)
	case tea.KeyF11:
		return tilde(23)
	case tea.KeyF12:
		return tilde(24)
	}
	return nil
}

// mode reads the emulator mode under its lock (pump writes it concurrently).
func (t *termTab) mode() vt10x.ModeFlag {
	t.vt.Lock()
	defer t.vt.Unlock()
	return t.vt.Mode()
}
