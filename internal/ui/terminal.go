package ui

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

type termOutputMsg struct{ id int }

type termDoneMsg struct {
	id  int
	err error
}

// termTab is one shell running on its own PTY, drawn through a VT emulator.
type termTab struct {
	id      int
	n       int // number shown in the tab label
	vt      *vt.Term
	ptmx    *os.File
	cmd     *exec.Cmd
	cols    int
	rows    int
	title   string
	snap    vt.Snapshot // reused by every render
	scanned int         // line number the failure scan reads from (vt.LinesSince)
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
	grab   bool // a mouse press was sent to the program; its drag and release follow
	sel    termSel
	tabX   []int
}

// termPos is a cell of a terminal tab: row is numbered as in vt.Snapshot.Top,
// so it stays on the same text while the view scrolls.
type termPos struct{ row, col int }

func (p termPos) less(q termPos) bool {
	return p.row < q.row || p.row == q.row && p.col < q.col
}

// termSel is text of the active tab picked with the mouse. a is where the
// drag started and b where it is now; both cells are included.
type termSel struct {
	on       bool // something is selected and drawn
	dragging bool
	a, b     termPos
}

func (s termSel) ordered() (termPos, termPos) {
	if s.b.less(s.a) {
		return s.b, s.a
	}
	return s.a, s.b
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
	args, env := shellIntegration(shell())
	cmd := exec.Command(shell(), args...)
	cmd.Dir = t.cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	cmd.Env = append(cmd.Env, env...)
	cols, rows := max(1, t.w), max(1, t.h)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return func() tea.Msg { return termErrMsg{err} }
	}
	t.nextID++
	t.nextN++
	tab := &termTab{
		id: t.nextID, n: t.nextN, ptmx: ptmx, cmd: cmd, cols: cols, rows: rows,
		vt: vt.New(vt.Options{Cols: cols, Rows: rows, Reply: ptmx}),
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
	for {
		n, err := tab.ptmx.Read(buf)
		if n > 0 {
			tab.vt.Write(buf[:n])
			select {
			case t.ch <- termOutputMsg{id: tab.id}:
			default: // a notification is already queued
			}
		}
		if err != nil {
			break
		}
	}
	werr := tab.cmd.Wait()
	t.ch <- termDoneMsg{id: tab.id, err: werr}
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
	case tea.KeyPressMsg:
		t.sel = termSel{}
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
			tab.vt.ScrollToBottom()
			tab.ptmx.Write(termKeyBytes(msg, tab.vt.Modes().AppCursor))
		}
	}
	return nil
}

func (t *terminal) switchTab(d int) {
	t.sel = termSel{}
	if n := len(t.tabs); n > 0 {
		t.active = (t.active + d + n) % n
	}
}

// run types a command line into the active shell, as if the user entered it.
func (t *terminal) run(line string) tea.Cmd {
	var cmd tea.Cmd
	if len(t.tabs) == 0 || t.cur().vt.Modes().AltScreen {
		cmd = t.newTab()
	}
	if tab := t.cur(); tab != nil {
		tab.vt.ScrollToBottom()
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
			if i != t.active {
				t.sel = termSel{}
			}
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

	var bar strings.Builder
	bar.WriteString(stPanelTitle.Render(" ") + stPanelTab.Render("TERMINAL"))
	x := 9
	t.tabX = t.tabX[:0]
	for i, tab := range t.tabs {
		t.tabX = append(t.tabX, x)
		s := " " + tab.label() + " "
		st := stPanelDim
		if i == t.active {
			st = stPanelCmd
		}
		bar.WriteString(st.Render(s))
		x += ansi.StringWidth(s)
	}
	t.tabX = append(t.tabX, x)
	bar.WriteString(stPanelDim.Render(i18n.T("term.new")))
	out = append(out, fitStyled(bar.String(), w, stPanel))

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
	sn := &tab.snap
	tab.vt.Snapshot(sn)
	tab.title = sn.Title
	for y := range body {
		cx := -1
		if focused && sn.CursorVisible && sn.CursorY == y {
			cx = sn.CursorX
		}
		s0, s1 := t.selCols(sn.Top + y)
		out = append(out, renderVTRowSel(sn.Cells[y], w, cx, s0, s1))
	}
	return out[:h]
}

// selCols returns the columns [s0, s1) of row (numbered as in
// vt.Snapshot.Top) that are selected; s0 == s1 when none are.
func (t *terminal) selCols(row int) (s0, s1 int) {
	if !t.sel.on {
		return 0, 0
	}
	a, b := t.sel.ordered()
	if row < a.row || row > b.row {
		return 0, 0
	}
	s0, s1 = 0, math.MaxInt
	if row == a.row {
		s0 = a.col
	}
	if row == b.row {
		s1 = b.col + 1
	}
	return s0, s1
}

// selectAt starts a selection at cell (x, y) of the body.
func (t *terminal) selectAt(x, y int) {
	tab := t.cur()
	if tab == nil {
		return
	}
	p := termPos{tab.snap.Top + clamp(y, 0, tab.rows-1), clamp(x, 0, tab.cols-1)}
	t.sel = termSel{dragging: true, a: p, b: p}
}

// dragTo moves the end of the selection to cell (x, y) of the body. Above or
// below the body, the view scrolls one line.
func (t *terminal) dragTo(x, y int) {
	tab := t.cur()
	if tab == nil {
		return
	}
	switch {
	case y < 0:
		tab.vt.ScrollView(1)
	case y >= tab.rows:
		tab.vt.ScrollView(-1)
	}
	tab.vt.Snapshot(&tab.snap)
	t.sel.b = termPos{tab.snap.Top + clamp(y, 0, tab.rows-1), clamp(x, 0, tab.cols-1)}
	t.sel.on = t.sel.a != t.sel.b
}

// endSelect finishes the drag and returns the selected text, or "" when the
// press did not move.
func (t *terminal) endSelect() string {
	t.sel.dragging = false
	tab := t.cur()
	if tab == nil || !t.sel.on {
		return ""
	}
	a, b := t.sel.ordered()
	return tab.vt.Text(a.row, a.col, b.row, b.col+1)
}

// renderVTRow turns one emulator row into an SGR-styled string w cells wide.
// cursorX is the column of the cursor, or -1.
func renderVTRow(cells []vt.Cell, w, cursorX int) string {
	return renderVTRowSel(cells, w, cursorX, 0, 0)
}

// renderVTRowSel is renderVTRow with the columns [s0, s1) drawn reversed, as
// selected.
func renderVTRowSel(cells []vt.Cell, w, cursorX, s0, s1 int) string {
	var sb strings.Builder
	type style struct {
		fg, bg vt.Color
		attr   vt.Attr
		cursor bool
	}
	var prev style
	first := true
	width := 0
	for x := 0; x < len(cells) && width < w; x++ {
		c := cells[x]
		if c.Width == 0 {
			continue
		}
		// The cursor sits on the right half of a wide character: draw it on the whole.
		onCursor := cursorX == x || (c.Width == 2 && cursorX == x+1)
		inSel := x+int(c.Width) > s0 && x < s1
		s := style{c.FG, c.BG, c.Attr, onCursor != inSel}
		if first || s != prev {
			sb.WriteString(sgr(s.fg, s.bg, s.attr, s.cursor))
			prev, first = s, false
		}
		if c.Rune == 0 {
			sb.WriteByte(' ')
		} else {
			sb.WriteString(c.Text())
		}
		width += int(c.Width)
	}
	if width < w {
		sb.WriteString(sgr(0, 0, 0, false))
		sb.WriteString(strings.Repeat(" ", w-width))
	}
	sb.WriteString("\x1b[0m")
	row := sb.String()
	// The host terminal may draw a cluster with another width than the one
	// counted; keep the row exactly w cells so the panel layout holds.
	switch sw := ansi.StringWidth(row); {
	case sw > w:
		return ansi.Truncate(row, w, "") + "\x1b[0m"
	case sw < w:
		return row + sgr(0, 0, 0, false) + strings.Repeat(" ", w-sw) + "\x1b[0m"
	}
	return row
}

func sgr(fg, bg vt.Color, attr vt.Attr, cursor bool) string {
	p := []string{"0"}
	if attr&vt.AttrBold != 0 {
		p = append(p, "1")
	}
	if attr&vt.AttrFaint != 0 {
		p = append(p, "2")
	}
	if attr&vt.AttrItalic != 0 {
		p = append(p, "3")
	}
	if attr&vt.AttrUnderline != 0 {
		p = append(p, "4")
	}
	if attr&vt.AttrStrike != 0 {
		p = append(p, "9")
	}
	fgp, bgp := sgrColor(fg, true, true), sgrColor(bg, false, false)
	if (attr&vt.AttrReverse != 0) != cursor {
		fgp, bgp = sgrColor(bg, true, false), sgrColor(fg, false, true)
	}
	p = append(p, fgp, bgp)
	return "\x1b[" + strings.Join(p, ";") + "m"
}

func hexRGB(h string) [3]int {
	v, _ := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	return [3]int{int(v >> 16 & 255), int(v >> 8 & 255), int(v & 255)}
}

// sgrColor renders c as a foreground (fg) or background parameter. A default
// color is the editor foreground when isFG, otherwise the editor background,
// so a reversed default cell still shows the opposite color.
func sgrColor(c vt.Color, fg, isFG bool) string {
	base := 48
	if fg {
		base = 38
	}
	if i, ok := c.Index(); ok {
		return fmt.Sprintf("%d;5;%d", base, i)
	}
	if r, g, b, ok := c.Components(); ok {
		return fmt.Sprintf("%d;2;%d;%d;%d", base, r, g, b)
	}
	// Default colors match the editor and follow the current theme.
	rgb := hexRGB(cEditorBg)
	if isFG {
		rgb = hexRGB(cEditorFg)
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", base, rgb[0], rgb[1], rgb[2])
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
func termKeyBytes(msg tea.KeyPressMsg, appCursor bool) []byte {
	shift := msg.Mod.Contains(tea.ModShift)
	alt := msg.Mod.Contains(tea.ModAlt)
	ctrl := msg.Mod.Contains(tea.ModCtrl)
	esc := func(b []byte) []byte {
		if alt {
			return append([]byte{0x1b}, b...)
		}
		return b
	}
	if s := typed(msg); s != "" {
		return []byte(s)
	}
	switch c := msg.Code; {
	case c == tea.KeySpace:
		if ctrl {
			return esc([]byte{0})
		}
		return esc([]byte{' '})
	case c == tea.KeyBackspace:
		return esc([]byte{0x7f})
	case c == tea.KeyTab && shift:
		return []byte("\x1b[Z")
	case c == tea.KeyTab, c == tea.KeyEnter, c == tea.KeyEscape:
		return esc([]byte{byte(c)})
	case ctrl && c >= 'a' && c <= 'z':
		return esc([]byte{byte(c - 'a' + 1)})
	case ctrl && c >= '@' && c <= '_': // ctrl+@ [ \ ] ^ _
		return esc([]byte{byte(c - '@')})
	case ctrl && c == '?':
		return esc([]byte{0x7f})
	case !ctrl && c >= ' ' && c < tea.KeyExtended && utf8.ValidRune(c): // alt+letter
		if shift && msg.ShiftedCode != 0 {
			c = msg.ShiftedCode
		}
		return esc([]byte(string(c)))
	}

	mod := 1
	if shift {
		mod++
	}
	if alt {
		mod += 2
	}
	if ctrl {
		mod += 4
	}
	// csi builds a cursor-style sequence (ESC [ 1 ; mod F).
	csi := func(f byte) []byte {
		if mod == 1 {
			if appCursor {
				return []byte{0x1b, 'O', f}
			}
			return []byte{0x1b, '[', f}
		}
		return []byte(fmt.Sprintf("\x1b[1;%d%c", mod, f))
	}
	// ss3 builds F1-F4, which drop to CSI form once a modifier is held.
	ss3 := func(f byte) []byte {
		if mod == 1 {
			return []byte{0x1b, 'O', f}
		}
		return []byte(fmt.Sprintf("\x1b[1;%d%c", mod, f))
	}
	// tilde builds an ESC [ n ; mod ~ sequence.
	tilde := func(n int) []byte {
		if mod == 1 {
			return []byte(fmt.Sprintf("\x1b[%d~", n))
		}
		return []byte(fmt.Sprintf("\x1b[%d;%d~", n, mod))
	}
	switch msg.Code {
	case tea.KeyUp:
		return csi('A')
	case tea.KeyDown:
		return csi('B')
	case tea.KeyRight:
		return csi('C')
	case tea.KeyLeft:
		return csi('D')
	case tea.KeyHome:
		return csi('H')
	case tea.KeyEnd:
		return csi('F')
	case tea.KeyInsert:
		return tilde(2)
	case tea.KeyDelete:
		return tilde(3)
	case tea.KeyPgUp:
		return tilde(5)
	case tea.KeyPgDown:
		return tilde(6)
	case tea.KeyF1:
		return ss3('P')
	case tea.KeyF2:
		return ss3('Q')
	case tea.KeyF3:
		return ss3('R')
	case tea.KeyF4:
		return ss3('S')
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

// paste sends text to the active shell without a trailing Enter. Programs that
// enabled bracketed paste receive it wrapped, so it arrives as one paste.
func (t *terminal) paste(text string) tea.Cmd {
	var cmd tea.Cmd
	if len(t.tabs) == 0 {
		cmd = t.newTab()
	}
	if tab := t.cur(); tab != nil {
		tab.vt.ScrollToBottom()
		if tab.vt.Modes().Bracketed {
			text = "\x1b[200~" + text + "\x1b[201~"
		}
		tab.ptmx.Write([]byte(text))
	}
	return tea.Batch(cmd, t.wait())
}

// mouse sends a mouse event at cell (x, y) of the terminal body to the
// program. It reports whether anything was sent, which is false when the
// program has not asked for mouse input.
func (t *terminal) mouse(msg mouseEvent, x, y int) bool {
	tab := t.cur()
	if tab == nil {
		return false
	}
	ev := vt.MouseEvent{
		X: min(max(x, 0), tab.cols-1), Y: min(max(y, 0), tab.rows-1),
		Shift: msg.Shift, Alt: msg.Alt, Ctrl: msg.Ctrl,
	}
	switch msg.Button {
	case tea.MouseNone:
		ev.Button = vt.ButtonNone
	case tea.MouseLeft:
		ev.Button = vt.ButtonLeft
	case tea.MouseMiddle:
		ev.Button = vt.ButtonMiddle
	case tea.MouseRight:
		ev.Button = vt.ButtonRight
	case tea.MouseWheelUp:
		ev.Button = vt.WheelUp
	case tea.MouseWheelDown:
		ev.Button = vt.WheelDown
	case tea.MouseWheelLeft:
		ev.Button = vt.WheelLeft
	case tea.MouseWheelRight:
		ev.Button = vt.WheelRight
	default:
		return false
	}
	switch msg.Action {
	case mousePress:
		ev.Action = vt.MousePress
	case mouseRelease:
		ev.Action = vt.MouseRelease
	case mouseMotion:
		ev.Action = vt.MouseMotion
	}
	b := tab.vt.EncodeMouse(ev)
	if len(b) == 0 {
		return false
	}
	tab.ptmx.Write(b)
	return true
}

// wheel handles a wheel turn over the panel; lines is negative for up. A
// program that reads the mouse gets the event. Otherwise the alternate screen
// (vim, less) gets arrow keys, and a shell's output scrolls through the
// scrollback.
func (t *terminal) wheel(msg mouseEvent, x, y, lines int) {
	tab := t.cur()
	if tab == nil {
		return
	}
	if t.mouse(msg, x, y) {
		return
	}
	m := tab.vt.Modes()
	if m.AltScreen {
		key := tea.KeyDown
		if lines < 0 {
			key = tea.KeyUp
		}
		for range abs(lines) {
			tab.ptmx.Write(termKeyBytes(tea.KeyPressMsg{Code: key}, m.AppCursor))
		}
		return
	}
	tab.vt.ScrollView(-lines)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
