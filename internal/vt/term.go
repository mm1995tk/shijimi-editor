package vt

import (
	"io"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// DefaultScrollback is the number of lines kept after they leave the screen.
const DefaultScrollback = 5000

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
	clip     *string // text the program asked to copy (OSC 52), not yet taken
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

// TakeClipboard returns the text the program last asked to copy with OSC 52,
// once.
func (t *Term) TakeClipboard() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.clip == nil {
		return "", false
	}
	s := *t.clip
	t.clip = nil
	return s, true
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
		t.sb = append(t.sb[:0], t.sb[drop:]...)
		t.view = t.clampView(t.view)
	}
}
