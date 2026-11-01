package tui

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/term"
	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

// termOutputMsg says the shell printed something, termDoneMsg that it ended
// and termErrMsg that none could be started. The pump of the PTY
// (internal/term) sends the first two on terminal.ch; WaitTerm hands them to
// Update, which gives them to handleTermEvent.
type termOutputMsg struct{ id int }

type termDoneMsg struct {
	id  int
	err error
}

// termErrMsg reports a failure to start a shell.
type termErrMsg struct{ err error }

func (termOutputMsg) isEvent() {}
func (termDoneMsg) isEvent()   {}
func (termErrMsg) isEvent()    {}

// termTab is the shell running on its own PTY (internal/term), drawn through
// a VT emulator.
type termTab struct {
	id      int
	vt      *vt.Term
	sess    *term.Session
	cols    int
	rows    int
	title   string
	snap    vt.Snapshot // reused by every render
	scanned int         // line number the failure scan reads from (vt.LinesSince)
}

// terminal is the bottom panel: one PTY-backed login shell. When it exits,
// sh is nil until the panel is opened again. The PTY and the process belong
// to internal/term; this holds what the panel shows of it.
type terminal struct {
	sh     *termTab
	cwd    string
	nextID int
	ch     chan Event
	armed  bool // a WaitTerm is outstanding
	w, h   int  // size of the body last rendered
	grab   bool // a mouse press was sent to the program; its drag and release follow
	sel    termSel
}

// termPos is a cell of the terminal: row is numbered as in vt.Snapshot.Top,
// so it stays on the same text while the view scrolls.
type termPos struct{ row, col int }

func (p termPos) less(q termPos) bool {
	return p.row < q.row || p.row == q.row && p.col < q.col
}

// termSel is text of the shell picked with the mouse. a is where the
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
	return &terminal{cwd: cwd, ch: make(chan Event, 64), w: 80, h: 24}
}

func (t *terminal) cur() *termTab { return t.sh }

// focus makes sure a shell exists so the panel is never empty.
func (t *terminal) focus() ([]Effect, error) {
	if t.sh == nil {
		return t.start()
	}
	return t.wait(), nil
}

func (t *terminal) blur() {}

// wait asks for the next PTY event to be delivered. Only one is ever
// outstanding; handleTermEvent asks again after every event.
func (t *terminal) wait() []Effect {
	if t.armed {
		return nil
	}
	t.armed = true
	return []Effect{WaitTerm{From: t.ch}}
}

// start runs a login shell in the current directory on a fresh PTY.
func (t *terminal) start() ([]Effect, error) {
	sh := term.DefaultShell()
	args, env := core.ShellIntegration(sh)
	cols, rows := max(1, t.w), max(1, t.h)
	sess, err := term.Start(term.Options{Shell: sh, Args: args, Dir: t.cwd, Env: env, Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	t.nextID++
	tab := &termTab{
		id: t.nextID, sess: sess, cols: cols, rows: rows,
		vt: vt.New(vt.Options{Cols: cols, Rows: rows, Reply: sess}),
	}
	t.sh = tab
	go t.pump(tab)
	return t.wait(), nil
}

// pump copies PTY output into the emulator and wakes the UI.
func (t *terminal) pump(tab *termTab) {
	err := tab.sess.Pump(func(b []byte) {
		tab.vt.Write(b)
		select {
		case t.ch <- termOutputMsg{id: tab.id}:
		default: // a notification is already queued
		}
	})
	t.ch <- termDoneMsg{id: tab.id, err: err}
}

// update takes a PTY event off the channel; the caller asks for the next one.
func (t *terminal) update(ev Event) []Effect {
	switch ev := ev.(type) {
	case termOutputMsg:
		t.armed = false
		return t.wait()
	case termDoneMsg:
		t.armed = false
		if t.sh != nil && t.sh.id == ev.id {
			t.sh.close()
			t.sh = nil
			t.sel = termSel{}
		}
		return t.wait()
	}
	return nil
}

// key sends a key press to the shell.
func (t *terminal) key(msg Key) {
	t.sel = termSel{}
	if tab := t.cur(); tab != nil {
		tab.vt.ScrollToBottom()
		tab.write(termKeyBytes(msg, tab.vt.Modes().AppCursor))
	}
}

// run types a command line into the shell, as if the user entered it.
func (t *terminal) run(line string) ([]Effect, error) {
	var effs []Effect
	if t.sh == nil {
		var err error
		if effs, err = t.start(); err != nil {
			return nil, err
		}
	} else if t.sh.vt.Modes().AltScreen {
		// A full-screen program owns the only terminal; typing into it would
		// send the command to that program.
		return nil, errors.New(i18n.T("term.busy"))
	}
	if tab := t.cur(); tab != nil {
		tab.vt.ScrollToBottom()
		tab.write([]byte(line + "\r"))
	}
	return batch(effs, t.wait()), nil
}

// closeAll ends the shell. Closing the PTY master hangs up the session.
func (t *terminal) closeAll() {
	if tab := t.sh; tab != nil {
		tab.close()
	}
}

// label is the title the program set, or the shell's name.
func (t *termTab) label() string {
	if s := strings.TrimSpace(t.title); s != "" {
		return s
	}
	return filepath.Base(term.DefaultShell())
}

// fit brings the terminal up to date for a panel w x h cells: it remembers the
// size (for a shell started later), resizes the shell to the body of the panel
// and takes the snapshot that render draws.
func (t *terminal) fit(w, h int) {
	body := h - 2
	if body < 0 {
		return
	}
	t.w, t.h = w, body
	tab := t.cur()
	if tab == nil || w < 1 || body < 1 {
		return
	}
	if tab.cols != w || tab.rows != body {
		tab.cols, tab.rows = w, body
		tab.resize(w, body)
		tab.vt.Resize(w, body)
	}
	tab.vt.Snapshot(&tab.snap)
	tab.title = tab.snap.Title
}

func (t *terminal) render(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelBord.Render(strings.Repeat("─", w)))

	var bar strings.Builder
	bar.WriteString(stPanelTitle.Render(" ") + stPanelTab.Render("TERMINAL"))
	if tab := t.sh; tab != nil {
		bar.WriteString(stPanelCmd.Render(" " + tab.label() + " "))
	}
	out = append(out, fitStyled(bar.String(), w, stPanel))

	body := h - 2
	if body < 0 {
		return out[:h]
	}
	tab := t.cur()
	if tab == nil || w < 1 || body < 1 {
		for len(out) < h {
			out = append(out, stPanel.Render(fit("", w)))
		}
		return out[:h]
	}
	sn := &tab.snap // taken in fit
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
func termKeyBytes(msg Key, appCursor bool) []byte {
	shift := msg.Mod.Has(ModShift)
	alt := msg.Mod.Has(ModAlt)
	ctrl := msg.Mod.Has(ModCtrl)
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
	case c == KeySpace:
		if ctrl {
			return esc([]byte{0})
		}
		return esc([]byte{' '})
	case c == KeyBackspace:
		return esc([]byte{0x7f})
	case c == KeyTab && shift:
		return []byte("\x1b[Z")
	case c == KeyTab, c == KeyEnter, c == KeyEscape:
		return esc([]byte{byte(c)})
	case ctrl && c >= 'a' && c <= 'z':
		return esc([]byte{byte(c - 'a' + 1)})
	case ctrl && c >= '@' && c <= '_': // ctrl+@ [ \ ] ^ _
		return esc([]byte{byte(c - '@')})
	case ctrl && c == '?':
		return esc([]byte{0x7f})
	case !ctrl && c >= ' ' && c < KeyExtended && utf8.ValidRune(c): // alt+letter
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
	case KeyUp:
		return csi('A')
	case KeyDown:
		return csi('B')
	case KeyRight:
		return csi('C')
	case KeyLeft:
		return csi('D')
	case KeyHome:
		return csi('H')
	case KeyEnd:
		return csi('F')
	case KeyInsert:
		return tilde(2)
	case KeyDelete:
		return tilde(3)
	case KeyPgUp:
		return tilde(5)
	case KeyPgDown:
		return tilde(6)
	case KeyF1:
		return ss3('P')
	case KeyF2:
		return ss3('Q')
	case KeyF3:
		return ss3('R')
	case KeyF4:
		return ss3('S')
	case KeyF5:
		return tilde(15)
	case KeyF6:
		return tilde(17)
	case KeyF7:
		return tilde(18)
	case KeyF8:
		return tilde(19)
	case KeyF9:
		return tilde(20)
	case KeyF10:
		return tilde(21)
	case KeyF11:
		return tilde(23)
	case KeyF12:
		return tilde(24)
	}
	return nil
}

// paste sends text to the shell without a trailing Enter. Programs that
// enabled bracketed paste receive it wrapped, so it arrives as one paste.
func (t *terminal) paste(text string) ([]Effect, error) {
	var effs []Effect
	if t.sh == nil {
		var err error
		if effs, err = t.start(); err != nil {
			return nil, err
		}
	}
	if tab := t.cur(); tab != nil {
		tab.vt.ScrollToBottom()
		if tab.vt.Modes().Bracketed {
			text = "\x1b[200~" + text + "\x1b[201~"
		}
		tab.write([]byte(text))
	}
	return batch(effs, t.wait()), nil
}

// mouse sends a mouse event at cell (x, y) of the terminal body to the
// program. It reports whether anything was sent, which is false when the
// program has not asked for mouse input.
func (t *terminal) mouse(msg Mouse, x, y int) bool {
	tab := t.cur()
	if tab == nil {
		return false
	}
	ev := vt.MouseEvent{
		X: min(max(x, 0), tab.cols-1), Y: min(max(y, 0), tab.rows-1),
		Shift: msg.Shift, Alt: msg.Alt, Ctrl: msg.Ctrl,
	}
	switch msg.Button {
	case MouseNone:
		ev.Button = vt.ButtonNone
	case MouseLeft:
		ev.Button = vt.ButtonLeft
	case MouseMiddle:
		ev.Button = vt.ButtonMiddle
	case MouseRight:
		ev.Button = vt.ButtonRight
	case MouseWheelUp:
		ev.Button = vt.WheelUp
	case MouseWheelDown:
		ev.Button = vt.WheelDown
	case MouseWheelLeft:
		ev.Button = vt.WheelLeft
	case MouseWheelRight:
		ev.Button = vt.WheelRight
	default:
		return false
	}
	switch msg.Action {
	case MousePress:
		ev.Action = vt.MousePress
	case MouseRelease:
		ev.Action = vt.MouseRelease
	case MouseMotion:
		ev.Action = vt.MouseMotion
	}
	b := tab.vt.EncodeMouse(ev)
	if len(b) == 0 {
		return false
	}
	tab.write(b)
	return true
}

// wheel handles a wheel turn over the panel; lines is negative for up. A
// program that reads the mouse gets the event. Otherwise the alternate screen
// (vim, less) gets arrow keys, and a shell's output scrolls through the
// scrollback.
func (t *terminal) wheel(msg Mouse, x, y, lines int) {
	tab := t.cur()
	if tab == nil {
		return
	}
	if t.mouse(msg, x, y) {
		return
	}
	m := tab.vt.Modes()
	if m.AltScreen {
		key := KeyDown
		if lines < 0 {
			key = KeyUp
		}
		for range abs(lines) {
			tab.write(termKeyBytes(Key{Code: key}, m.AppCursor))
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

// write sends bytes to the shell. A tab with no session (a test's) takes them
// and drops them.
func (t *termTab) write(b []byte) {
	if t.sess != nil {
		t.sess.Write(b)
	}
}

func (t *termTab) resize(cols, rows int) {
	if t.sess != nil {
		t.sess.Resize(cols, rows)
	}
}

func (t *termTab) close() {
	if t.sess != nil {
		t.sess.Close()
	}
}
