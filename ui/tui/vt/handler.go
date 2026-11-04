package vt

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// ---- characters ----

// decGraphics maps the DEC special graphics set (ESC ( 0) from 0x5f to 0x7e.
var decGraphics = []rune(" ◆▒␉␌␍␊°±␤␋┘┐┌└┼⎺⎻─⎼⎽├┤┴┬│≤≥π≠£·")

// runeWidth is the number of cells r takes. It uses the same width rules as
// the rest of the UI (ansi.StringWidth) so that rows are as wide as they are
// measured.
func runeWidth(r rune) int {
	if r < 0x300 {
		if r < 0x20 || (r >= 0x7f && r < 0xa0) {
			return 0
		}
		return 1
	}
	return ansi.StringWidth(string(r))
}

func (t *Term) print(r rune) {
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return // control characters never reach the grid
	}
	if g := t.gl; t.dec[g] && r >= 0x5f && r <= 0x7e {
		r = decGraphics[r-0x5f]
	}
	if t.joinsPrevious(r) {
		t.combine(r)
		return
	}
	w := runeWidth(r)
	if w == 0 {
		if isMark(r) {
			t.combine(r)
		}
		return
	}
	if w > t.cols {
		return
	}
	if t.wrapNext && t.autowrap {
		t.wrapLine()
	}
	t.wrapNext = false
	if t.x+w > t.cols {
		if !t.autowrap {
			t.x = t.cols - w
		} else {
			t.clearCells(t.y, t.x, t.cols)
			t.wrapLine()
		}
	}
	line := t.grid()[t.y]
	if t.insert {
		copy(line[t.x+w:], line[t.x:])
	}
	line[t.x] = Cell{Rune: r, Width: int8(w), FG: t.pen.fg, BG: t.pen.bg, Attr: t.pen.attr}
	for i := 1; i < w; i++ {
		line[t.x+i] = Cell{Width: 0, FG: t.pen.fg, BG: t.pen.bg, Attr: t.pen.attr}
	}
	sanitize(line)
	t.lastRune = r
	t.x += w
	if t.x >= t.cols {
		t.x = t.cols - 1
		t.wrapNext = t.autowrap
	}
}

// isMark reports whether r is a combining character or a joiner that is
// drawn as part of the character before it.
func isMark(r rune) bool {
	return r == 0x200d || unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc)
}

// prevCell finds the cell the next combining character belongs to.
func (t *Term) prevCell() (line []Cell, x int, ok bool) {
	x = t.x
	if !t.wrapNext {
		x--
	}
	if x < 0 {
		return nil, 0, false
	}
	line = t.grid()[t.y]
	if x > 0 && line[x].Width == 0 {
		x--
	}
	return line, x, line[x].Rune != 0
}

// joinsPrevious reports whether r continues the grapheme cluster before the
// cursor: an emoji skin tone modifier, or a character after a zero-width joiner.
func (t *Term) joinsPrevious(r rune) bool {
	line, x, ok := t.prevCell()
	if !ok {
		return false
	}
	if r >= 0x1f3fb && r <= 0x1f3ff {
		return true
	}
	return strings.HasSuffix(line[x].Comb, "‍")
}

// combine attaches r to the cell before the cursor. A cluster such as an emoji
// with a variation selector can be wider than its first character, so the cell
// is widened to the width the host terminal will draw it with.
func (t *Term) combine(r rune) {
	line, x, ok := t.prevCell()
	if !ok {
		return
	}
	line[x].Comb += string(r)
	w := min(max(ansi.StringWidth(line[x].Text()), 1), 2)
	if line[x].Width == 1 && w == 2 && x+1 < t.cols {
		pc := line[x]
		line[x].Width = 2
		line[x+1] = Cell{FG: pc.FG, BG: pc.BG, Attr: pc.Attr}
		if !t.wrapNext && t.x == x+1 {
			t.x++
			if t.x >= t.cols {
				t.x = t.cols - 1
				t.wrapNext = t.autowrap
			}
		}
	}
}

func (t *Term) wrapLine() {
	t.x = 0
	t.wrapNext = false
	t.index()
}

func (t *Term) execute(b byte) {
	switch b {
	case 0x08: // BS
		t.x = max(0, t.x-1)
		t.wrapNext = false
	case 0x09: // HT
		t.x = t.nextTab(t.x, 1)
		t.wrapNext = false
	case 0x0a, 0x0b, 0x0c: // LF, VT, FF
		t.index()
		if t.lnm {
			t.x = 0
		}
		t.wrapNext = false
	case 0x0d: // CR
		t.x = 0
		t.wrapNext = false
	case 0x0e: // SO
		t.gl = 1
	case 0x0f: // SI
		t.gl = 0
	}
}

func (t *Term) nextTab(x, n int) int {
	for ; n > 0; n-- {
		x++
		for x < t.cols-1 && !t.tabs[x] {
			x++
		}
	}
	return min(x, t.cols-1)
}

func (t *Term) prevTab(x, n int) int {
	for ; n > 0; n-- {
		x--
		for x > 0 && !t.tabs[x] {
			x--
		}
	}
	return max(x, 0)
}

// ---- screen operations ----

// index moves down one line, scrolling at the bottom of the region.
func (t *Term) index() {
	switch {
	case t.y == t.bot:
		t.scrollUp(1)
	case t.y < t.rows-1:
		t.y++
	}
}

func (t *Term) reverseIndex() {
	switch {
	case t.y == t.top:
		t.scrollDown(1)
	case t.y > 0:
		t.y--
	}
}

// scrollUp moves the lines of the region up by n. Lines that leave the top of
// a full-height region on the main screen go to the scrollback.
func (t *Term) scrollUp(n int) {
	g := t.grid()
	n = min(n, t.bot-t.top+1)
	for i := 0; i < n; i++ {
		if !t.altActive && t.top == 0 {
			t.pushScrollback(g[t.top])
		}
		copy(g[t.top:t.bot], g[t.top+1:t.bot+1])
		g[t.bot] = blankLine(t.cols, t.pen.bg)
	}
}

func (t *Term) scrollDown(n int) {
	g := t.grid()
	n = min(n, t.bot-t.top+1)
	for i := 0; i < n; i++ {
		copy(g[t.top+1:t.bot+1], g[t.top:t.bot])
		g[t.top] = blankLine(t.cols, t.pen.bg)
	}
}

// clearCells erases cells [x0, x1) of line y using the current background.
func (t *Term) clearCells(y, x0, x1 int) {
	line := t.grid()[y]
	x0, x1 = max(x0, 0), min(x1, t.cols)
	for x := x0; x < x1; x++ {
		line[x] = blank(t.pen.bg)
	}
	sanitize(line)
}

func (t *Term) moveTo(y, x int) {
	if t.origin {
		y = min(max(y+t.top, t.top), t.bot)
	} else {
		y = min(max(y, 0), t.rows-1)
	}
	t.y, t.x = y, min(max(x, 0), t.cols-1)
	t.wrapNext = false
}

func (t *Term) saveCursor() {
	t.saved = cursorState{t.x, t.y, t.pen, t.wrapNext, t.origin, t.gl, t.dec}
}

func (t *Term) restoreCursor() {
	s := t.saved
	t.x, t.y = min(s.x, t.cols-1), min(s.y, t.rows-1)
	t.pen, t.wrapNext, t.origin, t.gl, t.dec = s.pen, s.wrapNext, s.origin, s.gl, s.dec
}

func (t *Term) setAlt(on, save bool) {
	if on == t.altActive {
		return
	}
	if on {
		if save {
			t.saveCursor()
		}
		t.altActive = true
		for i := range t.alt {
			t.alt[i] = blankLine(t.cols, 0)
		}
		t.view = 0
		return
	}
	t.altActive = false
	if save {
		t.restoreCursor()
	}
}

func (t *Term) softReset() {
	t.pen = pen{}
	t.saved = cursorState{}
	t.dec, t.gl = [2]bool{}, 0
	t.modes.AppCursor = false
	t.autowrap, t.origin, t.insert, t.lnm, t.showCur = true, false, false, false, true
	t.top, t.bot = 0, t.rows-1
}

// ---- escape sequences ----

func (t *Term) esc(cmd ansi.Cmd) {
	switch cmd.Intermediate() {
	case 0:
		switch cmd.Final() {
		case '7':
			t.saveCursor()
		case '8':
			t.restoreCursor()
		case 'D':
			t.index()
		case 'E':
			t.x = 0
			t.index()
			t.wrapNext = false
		case 'M':
			t.reverseIndex()
		case 'H':
			t.tabs[t.x] = true
		case 'c':
			t.reset()
		}
	case '(', ')':
		i := int(cmd.Intermediate() - '(')
		switch cmd.Final() {
		case '0':
			t.dec[i] = true
		case 'B', 'A', 'U', 'K':
			t.dec[i] = false
		}
	case '#':
		if cmd.Final() == '8' { // DECALN
			for y := 0; y < t.rows; y++ {
				for x := 0; x < t.cols; x++ {
					t.grid()[y][x] = Cell{Rune: 'E', Width: 1}
				}
			}
		}
	}
}

func (t *Term) osc(cmd int, data []byte) {
	switch cmd {
	case 0, 2:
		s := string(data)
		if _, after, ok := strings.Cut(s, ";"); ok {
			t.title = strings.Map(func(r rune) rune {
				if r < 0x20 || r == 0x7f {
					return -1
				}
				return r
			}, after)
		}
	case 133: // shell integration: 133;D;<exit code> ends a command
		parts := strings.Split(string(data), ";")
		if len(parts) >= 2 && parts[1] == "D" {
			code := 0
			if len(parts) >= 3 {
				code, _ = strconv.Atoi(parts[2])
			}
			t.ends = append(t.ends, code)
		}
	case 52: // clipboard: 52;Pc;Pd, where Pd is base64 or "?" for a query
		if len(data) >= maxOSCData {
			// The parser dropped the rest; a cut-off copy would be wrong.
			t.clip = &Clip{TooLarge: true}
			return
		}
		parts := strings.SplitN(string(data), ";", 3)
		if len(parts) != 3 || parts[2] == "?" {
			return
		}
		b, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			// Some programs leave out the padding.
			b, err = base64.RawStdEncoding.DecodeString(parts[2])
		}
		// An empty payload asks to clear the clipboard; it must not wipe the
		// editor's yank register.
		if err != nil || len(b) == 0 {
			return
		}
		t.clip = &Clip{Text: string(b)}
	}
}

// arg returns parameter i, or def when it is missing.
func arg(p ansi.Params, i, def int) int {
	v, _, ok := p.Param(i, def)
	if !ok {
		return def
	}
	return v
}

// count returns parameter i as a repeat count: missing and 0 mean 1.
func count(p ansi.Params, i int) int {
	return max(1, arg(p, i, 1))
}

func (t *Term) csi(cmd ansi.Cmd, p ansi.Params) {
	prefix, inter := cmd.Prefix(), cmd.Intermediate()
	final := cmd.Final()
	switch {
	case prefix == '?' && inter == 0:
		switch final {
		case 'h', 'l':
			for i := range p {
				t.setPrivateMode(arg(p, i, 0), final == 'h')
			}
		}
		return
	case prefix == '>':
		if final == 'c' {
			t.out = append(t.out, "\x1b[>0;0;0c"...)
		}
		return
	case prefix != 0:
		return
	}
	switch inter {
	case 0:
	case '!':
		if final == 'p' {
			t.softReset()
		}
		return
	default:
		return // cursor style (DECSCUSR) and the like
	}

	switch final {
	case '@': // ICH
		t.editLine(func(line []Cell) {
			n := min(count(p, 0), t.cols-t.x)
			copy(line[t.x+n:], line[t.x:])
			for i := range n {
				line[t.x+i] = blank(t.pen.bg)
			}
		})
	case 'A', 'k':
		n := count(p, 0)
		top := 0
		if t.y >= t.top {
			top = t.top
		}
		t.y, t.wrapNext = max(top, t.y-n), false
	case 'B', 'e':
		n := count(p, 0)
		bot := t.rows - 1
		if t.y <= t.bot {
			bot = t.bot
		}
		t.y, t.wrapNext = min(bot, t.y+n), false
	case 'C', 'a':
		t.x, t.wrapNext = min(t.cols-1, t.x+count(p, 0)), false
	case 'D':
		t.x, t.wrapNext = max(0, t.x-count(p, 0)), false
	case 'E':
		t.moveRel(count(p, 0))
	case 'F':
		t.moveRel(-count(p, 0))
	case 'G', '`':
		t.x, t.wrapNext = min(max(count(p, 0)-1, 0), t.cols-1), false
	case 'H', 'f':
		t.moveTo(count(p, 0)-1, count(p, 1)-1)
	case 'I':
		t.x, t.wrapNext = t.nextTab(t.x, count(p, 0)), false
	case 'Z':
		t.x, t.wrapNext = t.prevTab(t.x, count(p, 0)), false
	case 'J':
		t.eraseDisplay(arg(p, 0, 0))
	case 'K':
		switch arg(p, 0, 0) {
		case 0:
			t.clearCells(t.y, t.x, t.cols)
		case 1:
			t.clearCells(t.y, 0, t.x+1)
		case 2:
			t.clearCells(t.y, 0, t.cols)
		}
	case 'L': // IL
		if t.y >= t.top && t.y <= t.bot {
			g := t.grid()
			n := min(count(p, 0), t.bot-t.y+1)
			copy(g[t.y+n:t.bot+1], g[t.y:t.bot+1])
			for i := range n {
				g[t.y+i] = blankLine(t.cols, t.pen.bg)
			}
			t.x, t.wrapNext = 0, false
		}
	case 'M': // DL
		if t.y >= t.top && t.y <= t.bot {
			g := t.grid()
			n := min(count(p, 0), t.bot-t.y+1)
			copy(g[t.y:t.bot+1-n], g[t.y+n:t.bot+1])
			for i := range n {
				g[t.bot-i] = blankLine(t.cols, t.pen.bg)
			}
			t.x, t.wrapNext = 0, false
		}
	case 'P': // DCH
		t.editLine(func(line []Cell) {
			n := min(count(p, 0), t.cols-t.x)
			copy(line[t.x:], line[t.x+n:])
			for i := t.cols - n; i < t.cols; i++ {
				line[i] = blank(t.pen.bg)
			}
		})
	case 'S':
		t.scrollUp(count(p, 0))
	case 'T':
		t.scrollDown(count(p, 0))
	case 'X': // ECH
		t.clearCells(t.y, t.x, t.x+count(p, 0))
	case 'b': // REP
		if t.lastRune != 0 {
			for i := count(p, 0); i > 0; i-- {
				t.print(t.lastRune)
			}
		}
	case 'c':
		t.out = append(t.out, "\x1b[?62;22c"...)
	case 'd':
		t.moveTo(count(p, 0)-1, t.x)
	case 'g':
		switch arg(p, 0, 0) {
		case 0:
			t.tabs[t.x] = false
		case 3:
			clear(t.tabs)
		}
	case 'h', 'l':
		on := final == 'h'
		for i := range p {
			switch arg(p, i, 0) {
			case 4:
				t.insert = on
			case 20:
				t.lnm = on
			}
		}
	case 'm':
		t.sgr(p)
	case 'n':
		switch arg(p, 0, 0) {
		case 5:
			t.out = append(t.out, "\x1b[0n"...)
		case 6:
			y := t.y
			if t.origin {
				y -= t.top
			}
			t.out = fmt.Appendf(t.out, "\x1b[%d;%dR", y+1, t.x+1)
		}
	case 'r':
		top, bot := count(p, 0), arg(p, 1, t.rows)
		if bot == 0 {
			bot = t.rows
		}
		bot = min(bot, t.rows)
		if top < bot {
			t.top, t.bot = top-1, bot-1
			t.moveTo(0, 0)
		}
	case 's':
		t.saveCursor()
	case 'u':
		t.restoreCursor()
	case 't':
		if arg(p, 0, 0) == 18 {
			t.out = fmt.Appendf(t.out, "\x1b[8;%d;%dt", t.rows, t.cols)
		}
	}
}

// moveRel moves n lines (negative is up) and to the first column.
func (t *Term) moveRel(n int) {
	y := t.y + n
	if n > 0 {
		bot := t.rows - 1
		if t.y <= t.bot {
			bot = t.bot
		}
		y = min(y, bot)
	} else {
		top := 0
		if t.y >= t.top {
			top = t.top
		}
		y = max(y, top)
	}
	t.y, t.x, t.wrapNext = y, 0, false
}

// editLine runs f on the cursor line and repairs wide characters afterwards.
func (t *Term) editLine(f func(line []Cell)) {
	line := t.grid()[t.y]
	f(line)
	sanitize(line)
	t.wrapNext = false
}

func (t *Term) eraseDisplay(mode int) {
	switch mode {
	case 0:
		t.clearCells(t.y, t.x, t.cols)
		for y := t.y + 1; y < t.rows; y++ {
			t.clearCells(y, 0, t.cols)
		}
	case 1:
		for y := 0; y < t.y; y++ {
			t.clearCells(y, 0, t.cols)
		}
		t.clearCells(t.y, 0, t.x+1)
	case 2:
		for y := 0; y < t.rows; y++ {
			t.clearCells(y, 0, t.cols)
		}
	case 3:
		t.dropped += len(t.sb)
		t.sb = nil
		t.view = 0
	}
}

func (t *Term) setPrivateMode(mode int, on bool) {
	switch mode {
	case 1:
		t.modes.AppCursor = on
	case 6:
		t.origin = on
		t.moveTo(0, 0)
	case 7:
		t.autowrap = on
		if !on {
			t.wrapNext = false
		}
	case 25:
		t.showCur = on
	case 47, 1047:
		t.setAlt(on, false)
	case 1048:
		if on {
			t.saveCursor()
		} else {
			t.restoreCursor()
		}
	case 1049:
		t.setAlt(on, true)
	case 1000, 1002, 1003:
		want := map[int]MouseMode{1000: MouseClick, 1002: MouseDrag, 1003: MouseAny}[mode]
		switch {
		case on:
			t.modes.Mouse = want
		case t.modes.Mouse == want:
			t.modes.Mouse = MouseOff
		}
	case 1004:
		t.modes.FocusEvents = on
	case 1006:
		t.modes.MouseSGR = on
	case 2004:
		t.modes.Bracketed = on
	}
}

// ---- SGR ----

func (t *Term) sgr(p ansi.Params) {
	if len(p) == 0 {
		t.pen = pen{}
		return
	}
	for i := 0; i < len(p); i++ {
		v, more, _ := p.Param(i, 0)
		if more { // colon form: code:sub:sub...
			sub := []int{v}
			for more && i+1 < len(p) {
				i++
				v, more, _ = p.Param(i, 0)
				sub = append(sub, v)
			}
			t.sgrColon(sub)
			continue
		}
		switch {
		case v == 0:
			t.pen = pen{}
		case v == 1:
			t.pen.attr |= AttrBold
		case v == 2:
			t.pen.attr |= AttrFaint
		case v == 3:
			t.pen.attr |= AttrItalic
		case v == 4 || v == 21:
			t.pen.attr |= AttrUnderline
		case v == 7:
			t.pen.attr |= AttrReverse
		case v == 9:
			t.pen.attr |= AttrStrike
		case v == 22:
			t.pen.attr &^= AttrBold | AttrFaint
		case v == 23:
			t.pen.attr &^= AttrItalic
		case v == 24:
			t.pen.attr &^= AttrUnderline
		case v == 27:
			t.pen.attr &^= AttrReverse
		case v == 29:
			t.pen.attr &^= AttrStrike
		case v >= 30 && v <= 37:
			t.pen.fg = Indexed(v - 30)
		case v == 39:
			t.pen.fg = 0
		case v >= 40 && v <= 47:
			t.pen.bg = Indexed(v - 40)
		case v == 49:
			t.pen.bg = 0
		case v >= 90 && v <= 97:
			t.pen.fg = Indexed(v - 90 + 8)
		case v >= 100 && v <= 107:
			t.pen.bg = Indexed(v - 100 + 8)
		case v == 38 || v == 48 || v == 58:
			// semicolon form: 38;5;n or 38;2;r;g;b
			var c Color
			var ok bool
			switch arg(p, i+1, -1) {
			case 5:
				c, ok = Indexed(arg(p, i+2, 0)), true
				i += 2
			case 2:
				c, ok = RGB(uint8(arg(p, i+2, 0)), uint8(arg(p, i+3, 0)), uint8(arg(p, i+4, 0))), true
				i += 4
			}
			t.setExtColor(v, c, ok)
		}
	}
}

func (t *Term) sgrColon(sub []int) {
	at := func(i int) int {
		if i < len(sub) {
			return sub[i]
		}
		return 0
	}
	switch sub[0] {
	case 4: // underline style; 0 turns it off
		if at(1) == 0 && len(sub) > 1 {
			t.pen.attr &^= AttrUnderline
		} else {
			t.pen.attr |= AttrUnderline
		}
	case 38, 48, 58:
		switch at(1) {
		case 5:
			t.setExtColor(sub[0], Indexed(at(2)), true)
		case 2:
			// 38:2::r:g:b carries an empty color-space id; 38:2:r:g:b does not.
			o := 2
			if len(sub) >= 6 {
				o = 3
			}
			t.setExtColor(sub[0], RGB(uint8(at(o)), uint8(at(o+1)), uint8(at(o+2))), true)
		}
	}
}

func (t *Term) setExtColor(code int, c Color, ok bool) {
	if !ok {
		return
	}
	switch code {
	case 38:
		t.pen.fg = c
	case 48:
		t.pen.bg = c
	} // 58 (underline color) is parsed so its arguments are skipped, then dropped
}
