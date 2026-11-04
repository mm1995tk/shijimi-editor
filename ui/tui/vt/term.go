package vt

import (
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/parser"
)

// Programs write UTF-8, whose continuation bytes include 0x9C, the 8-bit
// string terminator, and the other C1 controls. The parser takes them as
// controls inside strings, so a title such as "✳ 検索" (✳ is E2 9C B3) ended
// early and the rest of it was printed on the screen. Treat 0x80-0xFF inside
// strings as text, as UTF-8 terminals do. As a result 0x9C never ends a string
// any more: only ESC \, BEL, CAN and SUB do.
//
// parser.Table is shared by the whole process, so this also fixes
// ansi.StringWidth, Strip and Truncate in the UI.
func init() {
	// Upstream already puts 0x80-0xFF in OSC and DCS strings, except 0x9C.
	for _, s := range []parser.State{parser.DcsStringState, parser.OscStringState} {
		parser.Table.AddOne(0x9C, s, parser.PutAction, s)
	}
	// In SOS, PM and APC strings 0x80-0xFF still follow the "anywhere" C1 and
	// UTF-8 transitions, which print the string on the screen.
	for _, s := range []parser.State{parser.SosStringState, parser.PmStringState, parser.ApcStringState} {
		parser.Table.AddRange(0x80, 0xFF, s, parser.PutAction, s)
	}
}

// DefaultScrollback is the number of lines kept after they leave the screen.
const DefaultScrollback = 5000

// maxOSCData is the most an OSC (or DCS, APC, …) payload may hold. It bounds
// OSC 52 copies at about 1.5 MB of text; the parser's default of 64 KB cut
// copies off at 48 KB.
const maxOSCData = 2 << 20

// Options configures a Term.
type Options struct {
	Cols, Rows int
	// Scrollback is the number of lines kept after they scroll off the top of
	// the main screen. Zero means DefaultScrollback; negative disables it.
	Scrollback int
	// Reply receives the answers a program asked for (device attributes,
	// cursor position reports). It may be nil.
	Reply io.Writer
}

// MouseMode is how much mouse input the running program asked for.
type MouseMode int

const (
	MouseOff   MouseMode = iota
	MouseClick           // button presses and releases (1000)
	MouseDrag            // plus motion while a button is held (1002)
	MouseAny             // plus all motion (1003)
)

// Modes is the part of the terminal state that decides how input is encoded.
type Modes struct {
	AppCursor   bool // cursor keys send SS3 sequences (DECCKM)
	AltScreen   bool
	Bracketed   bool // bracketed paste (2004)
	FocusEvents bool // focus in/out reports (1004)
	Mouse       MouseMode
	MouseSGR    bool // SGR mouse encoding (1006)
}

type pen struct {
	fg, bg Color
	attr   Attr
}

// cursorState is what DECSC saves and DECRC restores.
type cursorState struct {
	x, y     int
	pen      pen
	wrapNext bool
	origin   bool
	gl       int
	dec      [2]bool
}

// Term is a terminal screen. All methods are safe for concurrent use.
type Term struct {
	mu     sync.Mutex
	parser *ansi.Parser
	reply  io.Writer
	out    []byte // replies produced during Write

	cols, rows int
	main, alt  [][]Cell
	altActive  bool

	sb    [][]Cell
	sbMax int
	view  int // lines the view is scrolled back from the bottom

	x, y     int
	wrapNext bool
	pen      pen
	saved    cursorState
	top, bot int // scroll region, inclusive
	tabs     []bool

	// character sets: dec[i] is true when Gi is DEC special graphics.
	dec [2]bool
	gl  int

	modes    Modes
	autowrap bool
	origin   bool
	insert   bool
	lnm      bool
	showCur  bool

	title    string
	clip     *Clip // the program's last copy (OSC 52), not yet taken
	ends     []int // exit codes reported with OSC 133;D, not yet taken
	dropped  int   // lines trimmed from the front of the scrollback so far
	lastRune rune
}

// New returns an empty terminal.
func New(o Options) *Term {
	t := &Term{reply: o.Reply, sbMax: o.Scrollback}
	switch {
	case t.sbMax == 0:
		t.sbMax = DefaultScrollback
	case t.sbMax < 0:
		t.sbMax = 0
	}
	t.cols, t.rows = max(1, o.Cols), max(1, o.Rows)
	t.main = newGrid(t.cols, t.rows)
	t.alt = newGrid(t.cols, t.rows)
	t.parser = ansi.NewParser()
	t.parser.SetDataSize(maxOSCData)
	t.parser.SetHandler(ansi.Handler{
		Print:     t.print,
		Execute:   t.execute,
		HandleCsi: t.csi,
		HandleEsc: t.esc,
		HandleOsc: t.osc,
	})
	t.reset()
	return t
}

func newGrid(cols, rows int) [][]Cell {
	g := make([][]Cell, rows)
	for i := range g {
		g[i] = blankLine(cols, 0)
	}
	return g
}

// reset is a hard reset (RIS) that keeps the scrollback.
func (t *Term) reset() {
	for _, g := range [][][]Cell{t.main, t.alt} {
		for i := range g {
			g[i] = blankLine(t.cols, 0)
		}
	}
	t.altActive = false
	t.x, t.y, t.wrapNext = 0, 0, false
	t.pen = pen{}
	t.saved = cursorState{}
	t.dec, t.gl = [2]bool{}, 0
	t.modes = Modes{}
	t.autowrap, t.origin, t.insert, t.lnm, t.showCur = true, false, false, false, true
	t.top, t.bot = 0, t.rows-1
	t.tabs = make([]bool, t.cols)
	for i := 0; i < t.cols; i += 8 {
		t.tabs[i] = true
	}
	t.view = 0
}

func (t *Term) grid() [][]Cell {
	if t.altActive {
		return t.alt
	}
	return t.main
}

// Write feeds program output to the terminal.
func (t *Term) Write(p []byte) (int, error) {
	t.mu.Lock()
	for _, b := range p {
		t.parser.Advance(b)
	}
	out := t.out
	t.out = nil
	t.mu.Unlock()
	if len(out) > 0 && t.reply != nil {
		t.reply.Write(out)
	}
	return len(p), nil
}

// Size returns the screen size in cells.
func (t *Term) Size() (cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cols, t.rows
}

// Modes returns the input-related modes.
func (t *Term) Modes() Modes {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.modes
	m.AltScreen = t.altActive
	return m
}

// Title returns the window title set by the program.
func (t *Term) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

// Clip is a copy a program asked for with OSC 52.
type Clip struct {
	Text string
	// TooLarge means the copy did not fit in maxOSCData and was dropped.
	TooLarge bool
}

// TakeClipboard returns the copy the program last asked for with OSC 52,
// once. Only the last one since the previous call is kept.
func (t *Term) TakeClipboard() (Clip, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.clip == nil {
		return Clip{}, false
	}
	c := *t.clip
	t.clip = nil
	return c, true
}

// Scrollback returns the number of lines held above the screen.
func (t *Term) Scrollback() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.sb)
}

// ScrollView moves the view back (n > 0) or forward (n < 0) through the
// scrollback and returns how far back it now is. The alternate screen has no
// scrollback, so the view stays at the bottom there.
func (t *Term) ScrollView(n int) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.view = t.clampView(t.view + n)
	return t.view
}

// ScrollToBottom returns the view to the live screen.
func (t *Term) ScrollToBottom() {
	t.mu.Lock()
	t.view = 0
	t.mu.Unlock()
}

func (t *Term) clampView(v int) int {
	if t.altActive {
		return 0
	}
	return min(max(v, 0), len(t.sb))
}

// Snapshot is a copy of what the screen shows.
type Snapshot struct {
	Cols, Rows    int
	Cells         [][]Cell // Rows lines of Cols cells
	CursorX       int
	CursorY       int
	CursorVisible bool
	Offset        int // lines the view is scrolled back
	Scrollback    int
	Top           int // number of the first row shown, counted as in Text
	Title         string
	Modes         Modes
}

// Snapshot copies the visible screen into dst, reusing its storage.
func (t *Term) Snapshot(dst *Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	dst.Cols, dst.Rows = t.cols, t.rows
	if len(dst.Cells) != t.rows {
		dst.Cells = make([][]Cell, t.rows)
	}
	g := t.grid()
	for y := 0; y < t.rows; y++ {
		if len(dst.Cells[y]) != t.cols {
			dst.Cells[y] = make([]Cell, t.cols)
		}
		var src []Cell
		if i := len(t.sb) - t.view + y; i < len(t.sb) {
			src = t.sb[i]
		} else {
			src = g[i-len(t.sb)]
		}
		n := copy(dst.Cells[y], src)
		for x := n; x < t.cols; x++ {
			dst.Cells[y][x] = blank(0)
		}
		// A line from before a resize may cut a wide character in two.
		sanitize(dst.Cells[y])
	}
	dst.CursorX, dst.CursorY = t.x, t.y
	dst.CursorVisible = t.showCur && t.view == 0
	dst.Offset, dst.Scrollback = t.view, len(t.sb)
	dst.Top = t.dropped + len(t.sb) - t.view
	dst.Title = t.title
	dst.Modes = t.modes
	dst.Modes.AltScreen = t.altActive
}

// Resize changes the screen size. Content is not reflowed: lines are cut or
// padded, and when the screen gets shorter the lines above the cursor move
// into the scrollback.
func (t *Term) Resize(cols, rows int) {
	cols, rows = max(1, cols), max(1, rows)
	t.mu.Lock()
	defer t.mu.Unlock()
	if cols == t.cols && rows == t.rows {
		return
	}
	if rows < t.rows {
		drop := 0
		if !t.altActive {
			drop = max(0, t.y-(rows-1))
		}
		for i := 0; i < drop; i++ {
			t.pushScrollback(t.main[i])
		}
		t.main = t.main[drop : rows+drop]
		t.y -= drop
		t.alt = t.alt[:rows]
	} else if rows > t.rows {
		extra := rows - t.rows
		pull := 0
		if !t.altActive {
			pull = min(len(t.sb), extra)
		}
		pulled := make([][]Cell, 0, extra)
		for i := len(t.sb) - pull; i < len(t.sb); i++ {
			pulled = append(pulled, t.sb[i])
		}
		t.sb = t.sb[:len(t.sb)-pull]
		t.y += pull
		t.main = append(pulled, t.main...)
		for len(t.main) < rows {
			t.main = append(t.main, blankLine(t.cols, 0))
		}
		for len(t.alt) < rows {
			t.alt = append(t.alt, blankLine(t.cols, 0))
		}
	}
	for _, g := range [][][]Cell{t.main, t.alt} {
		for i, l := range g {
			g[i] = fitLine(l, cols)
		}
	}
	for i := t.cols; i < cols; i++ {
		t.tabs = append(t.tabs, i%8 == 0)
	}
	t.tabs = t.tabs[:cols]
	t.cols, t.rows = cols, rows
	t.top, t.bot = 0, rows-1
	t.x, t.y = min(t.x, cols-1), min(max(t.y, 0), rows-1)
	t.wrapNext = false
	t.view = t.clampView(t.view)
}

// fitLine cuts or pads l to n cells.
func fitLine(l []Cell, n int) []Cell {
	if len(l) >= n {
		l = l[:n]
	} else {
		l = append(l, blankLine(n-len(l), 0)...)
	}
	sanitize(l)
	return l
}

// sanitize blanks wide-character halves that lost their other half.
func sanitize(l []Cell) {
	for x := range l {
		switch l[x].Width {
		case 2:
			if x+1 >= len(l) || l[x+1].Width != 0 {
				l[x] = blank(l[x].BG)
			}
		case 0:
			if x == 0 || l[x-1].Width != 2 {
				l[x] = blank(l[x].BG)
			}
		}
	}
}

func (t *Term) pushScrollback(line []Cell) {
	if t.sbMax == 0 {
		return
	}
	n := len(line)
	for n > 0 && line[n-1].Rune == 0 && line[n-1].BG == 0 && line[n-1].Width != 0 {
		n--
	}
	t.sb = append(t.sb, line[:n:n])
	if t.view > 0 {
		t.view = min(t.view+1, len(t.sb))
	}
	// Trim in batches so the copy is amortized.
	if len(t.sb) > t.sbMax+t.sbMax/4+1 {
		drop := len(t.sb) - t.sbMax
		t.dropped += drop
		t.sb = append(t.sb[:0], t.sb[drop:]...)
		t.view = t.clampView(t.view)
	}
}

// line returns line n, numbered from the first one ever written, or nil when
// it has left the scrollback or is past the screen.
func (t *Term) line(n int) []Cell {
	i := n - t.dropped
	if i < 0 {
		return nil
	}
	if i < len(t.sb) {
		return t.sb[i]
	}
	if g := t.grid(); i-len(t.sb) < len(g) {
		return g[i-len(t.sb)]
	}
	return nil
}

// Text returns the characters from column c0 of line r0 up to, but not
// including, column c1 of line r1. Lines are numbered as in Snapshot.Top, and
// each line loses its trailing blanks.
func (t *Term) Text(r0, c0, r1, c1 int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var lines []string
	for n := r0; n <= r1; n++ {
		l := t.line(n)
		from, to := 0, len(l)
		if n == r0 {
			from = min(max(c0, 0), len(l))
			// Starting on the right half of a wide character takes all of it.
			for from > 0 && from < len(l) && l[from].Width == 0 {
				from--
			}
		}
		if n == r1 {
			to = min(max(c1, from), len(l))
		}
		lines = append(lines, lineText(l[from:to]))
	}
	return strings.Join(lines, "\n")
}

// TakeCommandEnds returns the exit codes the shell reported with OSC 133;D
// (shell integration) since the last call, oldest first.
func (t *Term) TakeCommandEnds() []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.ends
	t.ends = nil
	return e
}

// lineText is the characters of a line with trailing blanks removed.
func lineText(l []Cell) string {
	var sb strings.Builder
	for _, c := range l {
		switch {
		case c.Width == 0:
		case c.Rune == 0:
			sb.WriteByte(' ')
		default:
			sb.WriteString(c.Text())
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

// LinesSince returns the text of the main screen's finished lines, those
// above the cursor, from line number from on, and the number of the cursor
// line. Lines are numbered from the first one ever written, so a caller
// passes the returned number back to read only what came after. Lines that
// have left the scrollback are skipped. When the number went back (the
// screen was cleared and the cursor moved up), the screen is read from its
// top. Nothing is returned while the alternate screen is shown.
func (t *Term) LinesSince(from int) (lines []string, next int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.altActive {
		return nil, from
	}
	next = t.dropped + len(t.sb) + t.y
	if from > next {
		from = t.dropped + len(t.sb)
	}
	for n := max(from, t.dropped); n < next; n++ {
		if i := n - t.dropped; i < len(t.sb) {
			lines = append(lines, lineText(t.sb[i]))
		} else {
			lines = append(lines, lineText(t.main[i-len(t.sb)]))
		}
	}
	return lines, next
}

// Tail returns the last n lines of the main screen up to the cursor line,
// with trailing empty lines removed.
func (t *Term) Tail(n int) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var all []string
	end := len(t.sb) + t.y
	if t.altActive {
		end = len(t.sb) + len(t.main) - 1
	}
	for i := max(0, end-n+1); i <= end; i++ {
		if i < len(t.sb) {
			all = append(all, lineText(t.sb[i]))
		} else if i-len(t.sb) < len(t.main) {
			all = append(all, lineText(t.main[i-len(t.sb)]))
		}
	}
	for len(all) > 0 && all[len(all)-1] == "" {
		all = all[:len(all)-1]
	}
	return all
}
