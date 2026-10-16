package ui

import (
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

type editMode int

const (
	modeNormal editMode = iota
	modeInsert
	modeSelect
)

func (m editMode) label() string {
	switch m {
	case modeInsert:
		return "INS"
	case modeSelect:
		return "SEL"
	}
	return "NOR"
}

// handleHelixKey implements Helix-style modal editing for text tabs.
func (a *App) handleHelixKey(msg tea.KeyMsg) tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	if a.mode == modeInsert {
		return a.handleInsertKey(msg)
	}
	b := t.buf
	key := msg.String()
	a.follow = true
	ext := a.mode == modeSelect

	// pending multi-key sequences
	if a.pending != "" {
		p := a.pending
		a.pending = ""
		if b.HasExtra() && (strings.Contains("ftFTr", p) || p == "g" && strings.Contains("hls", key)) {
			var cmd tea.Cmd
			b.ForEachCursor(func() { cmd = a.helixPending(p, key, msg) })
			return cmd
		}
		return a.helixPending(p, key, msg)
	}

	// counts
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || a.count > 0) {
		a.count = a.count*10 + int(key[0]-'0')
		return nil
	}
	hadCount := a.count > 0
	count := max(1, a.count)
	a.count = 0
	repeat := func(f func()) {
		for i := 0; i < count; i++ {
			f()
		}
	}

	switch key {
	case "C", "alt+C":
		dir := 1
		if key == "alt+C" {
			dir = -1
		}
		repeat(func() { b.CopySelection(dir) })
		return nil
	case ",":
		b.ClearExtra()
		return nil
	case "s":
		if _, _, ok := b.Selection(); !ok && !b.HasExtra() {
			a.info("範囲を選択してから s を押してください")
			return nil
		}
		return a.openCmdline("s")
	}

	if b.HasExtra() && helixMultiKeys[key] {
		var cmd tea.Cmd
		b.ForEachCursor(func() { cmd = a.helixNormalKey(b, key, ext, count, hadCount, repeat) })
		return cmd
	}
	return a.helixNormalKey(b, key, ext, count, hadCount, repeat)
}

// helixMultiKeys are the normal-mode keys applied to every cursor.
var helixMultiKeys = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Fields("h l j k left right down up w W b B e E home end x X ; alt+; esc d alt+d c alt+c y p P R i a I A o O ~ ` alt+` f t F T r") {
		m[k] = true
	}
	return m
}()

func (a *App) helixNormalKey(b *buffer.Buffer, key string, ext bool, count int, hadCount bool, repeat func(func())) tea.Cmd {
	switch key {
	// --- movement ---
	case "h", "left":
		repeat(func() { b.MoveCursor(buffer.Left, ext) })
	case "l", "right":
		repeat(func() { b.MoveCursor(buffer.Right, ext) })
	case "j", "down":
		repeat(func() { b.MoveCursor(buffer.Down, ext) })
	case "k", "up":
		repeat(func() { b.MoveCursor(buffer.Up, ext) })
	case "w", "W":
		repeat(func() { b.SelectWord(true, ext) })
	case "b", "B":
		repeat(func() { b.SelectWord(false, ext) })
	case "e", "E":
		if !ext {
			b.Anchor = nil
		}
		repeat(func() { b.WordEnd(true) })
	case "home":
		b.MoveCursor(buffer.Home, ext)
	case "end":
		b.MoveCursor(buffer.End, ext)
	case "pgup", "ctrl+b":
		b.MoveLines(-a.pageSize(), ext)
	case "pgdown", "ctrl+f":
		b.MoveLines(a.pageSize(), ext)
	case "ctrl+u":
		b.MoveLines(-a.pageSize()/2, ext)
	case "ctrl+d":
		b.MoveLines(a.pageSize()/2, ext)
	case "G":
		if hadCount {
			b.GotoLine(count)
		} else {
			b.MoveCursor(buffer.DocEnd, ext)
		}

	// --- selection ---
	case "x", "X":
		repeat(b.SelectLine)
	case "%":
		b.SelectAll()
	case ";":
		b.ClearSelection()
	case "alt+;":
		if b.Anchor != nil {
			a, c := *b.Anchor, b.Cur
			b.Anchor, b.Cur = &c, a
		}
	case "v":
		if a.mode == modeSelect {
			a.mode = modeNormal
		} else {
			a.mode = modeSelect
		}
	case "esc":
		a.mode = modeNormal
		b.ClearSelection()

	// --- editing ---
	case "d", "alt+d":
		s, e := b.SelectionOrChar()
		text := b.DeleteRange(s, e)
		if key == "d" {
			a.clipboard = text
		}
		a.mode = modeNormal
	case "c", "alt+c":
		s, e := b.SelectionOrChar()
		if _, _, ok := b.Selection(); ok {
			text := b.DeleteRange(s, e)
			if key == "c" {
				a.clipboard = text
			}
		} else {
			a.clipboard = b.DeleteRange(s, e)
		}
		return a.enterInsert()
	case "y":
		s, e := b.SelectionOrChar()
		a.clipboard = b.TextRange(s, e)
		a.info("%d 文字をヤンクしました", len([]rune(a.clipboard)))
	case "p", "P":
		a.paste(b, a.clipboard, key == "p")
	case "R":
		if s, e, ok := b.Selection(); ok {
			b.DeleteRange(s, e)
			b.InsertBlock(a.clipboard)
		}
	case "u":
		repeat(func() { b.Undo() })
	case "U":
		repeat(func() { b.Redo() })
	case "i":
		if s, _, ok := b.Selection(); ok {
			b.Cur = s
		}
		b.ClearSelection()
		return a.enterInsert()
	case "a":
		if _, e, ok := b.Selection(); ok {
			b.Cur = e
		} else if b.Cur.Col < len(b.Lines[b.Cur.Row]) {
			b.Cur.Col++
		}
		b.ClearSelection()
		return a.enterInsert()
	case "I":
		b.ClearSelection()
		b.Cur.Col = 0
		b.MoveCursor(buffer.Home, false)
		return a.enterInsert()
	case "A":
		b.ClearSelection()
		b.MoveCursor(buffer.End, false)
		return a.enterInsert()
	case "o":
		b.OpenLine(true)
		return a.enterInsert()
	case "O":
		b.OpenLine(false)
		return a.enterInsert()
	case ">":
		repeat(a.indentLines(b, true))
	case "<":
		repeat(a.indentLines(b, false))
	case "J":
		b.JoinLines()
	case "~", "`", "alt+`":
		a.changeCase(b, key)
	case "ctrl+c":
		a.toggleComment()

	// --- search ---
	case "/", "?":
		return a.openCmdline(key)
	case "n":
		a.searchNext(false)
	case "N":
		a.searchNext(true)
	case "*":
		if s := b.SelectedText(); s != "" && !strings.Contains(s, "\n") {
			a.lastSearch = s
			a.info("検索: %s", s)
		}

	// --- prefixes ---
	case "g", "m", "]", "[", "f", "t", "F", "T", "r", " ", "space":
		a.pending = key
		if key == "space" {
			a.pending = " "
		}
		if hadCount && key == "g" {
			b.GotoLine(count)
			a.pending = ""
		}
		return nil
	case ":":
		return a.openCmdline(":")
	case "K":
		return a.hover()
	}
	return nil
}

func (a *App) pageSize() int { return max(1, a.layout().editH-2) }

func (a *App) enterInsert() tea.Cmd {
	a.mode = modeInsert
	return nil
}

func (a *App) helixPending(p, key string, msg tea.KeyMsg) tea.Cmd {
	t := a.cur()
	b := t.buf
	ext := a.mode == modeSelect
	ch := []rune(key)
	single := msg.Type == tea.KeyRunes && len(msg.Runes) == 1 || key == " "
	var r rune
	if key == " " || key == "space" {
		r = ' '
	} else if len(ch) > 0 {
		r = ch[0]
	}
	switch p {
	case "g":
		switch key {
		case "g":
			b.MoveCursor(buffer.DocStart, ext)
		case "e":
			b.MoveCursor(buffer.DocEnd, ext)
			b.Cur.Col = 0
		case "h":
			b.Cur.Col = 0
			b.SetCursor(b.Cur.Row, 0, ext)
		case "l":
			b.MoveCursor(buffer.End, ext)
		case "s":
			b.SetCursor(b.Cur.Row, 0, ext)
			b.MoveCursor(buffer.Home, ext)
		case "d":
			return a.gotoDefinition()
		case "r":
			return a.references()
		case "n":
			a.switchTab(1)
		case "p":
			a.switchTab(-1)
		case "t":
			b.SetCursor(b.ScrollRow, 0, ext)
		case "b":
			b.SetCursor(b.ScrollRow+a.layout().editH-1, 0, ext)
		case "c":
			b.SetCursor(b.ScrollRow+a.layout().editH/2, 0, ext)
		}
	case "m":
		if key == "m" {
			b.MatchBracket(ext)
		}
	case "]":
		switch key {
		case "d":
			a.gotoDiagnostic(1)
		}
	case "[":
		switch key {
		case "d":
			a.gotoDiagnostic(-1)
		}
	case "f", "t", "F", "T":
		if single {
			if !ext {
				b.Anchor = nil
			}
			if !b.FindChar(r, p == "f" || p == "t", p == "t" || p == "T", true) {
				a.info("見つかりません: %c", r)
			}
		}
	case "r":
		if single {
			b.ReplaceChars(r)
		} else if key == "enter" {
			b.ReplaceChars('\n')
		}
	case " ":
		return a.spaceMode(key)
	}
	return nil
}

// spaceMode handles "Space <key>" commands (Helix's space menu).
func (a *App) spaceMode(key string) tea.Cmd {
	switch key {
	case "f":
		return a.pal.show(a.root, "")
	case "b":
		return a.bufferPicker()
	case "k":
		return a.hover()
	case "o":
		return a.openURL("")
	case "r":
		return a.rename()
	case "d", "D":
		return a.diagnosticPicker()
	case "e", "E":
		a.showSide = true
		return a.setFocus(focusExplorer)
	case "m":
		return a.togglePreview()
	case "g":
		return a.prPicker()
	case "G":
		return a.openLocalReview()
	case "y":
		if t := a.cur(); t != nil && t.kind == tabText {
			s, e := t.buf.SelectionOrChar()
			a.copyText(t.buf.TextRange(s, e))
			a.info("クリップボードにコピーしました")
		}
	case "p", "P":
		if t := a.cur(); t != nil && t.kind == tabText {
			if s, ok := readSystemClipboard(); ok {
				a.paste(t.buf, s, key == "p")
			}
		}
	case "?":
		return a.pal.show(a.root, ">")
	case "/":
		return a.openCmdline(":grep ")
	case "c":
		a.toggleComment()
	case "t":
		return a.toggleTerm()
	case "a":
		return a.sendContext()
	}
	return nil
}

func (a *App) handleInsertKey(msg tea.KeyMsg) tea.Cmd {
	t := a.cur()
	b := t.buf
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.comp = nil
		b.ForEachCursor(b.ClearSelection)
		return nil
	case "ctrl+x", "ctrl+@", "ctrl+ ":
		return a.completion()
	case "ctrl+w", "alt+backspace":
		b.ForEachCursor(func() {
			end := b.Cur
			b.MoveCursor(buffer.WordLeft, false)
			b.DeleteRange(b.Cur, end)
		})
		return nil
	case "ctrl+u":
		b.ForEachCursor(func() {
			end := b.Cur
			b.SetCursor(b.Cur.Row, 0, false)
			b.DeleteRange(b.Cur, end)
		})
		return nil
	}
	return a.handleEditorKey(msg)
}

func (a *App) paste(b *buffer.Buffer, text string, after bool) {
	if text == "" {
		return
	}
	s, e, ok := b.Selection()
	if !ok {
		s, e = b.Cur, b.Cur
		if e.Col < len(b.Lines[e.Row]) {
			e.Col++
		}
	}
	var at buffer.Pos
	if strings.HasSuffix(text, "\n") {
		// linewise
		if after {
			row := e.Row
			if e.Col == 0 && e.Row > s.Row {
				row = e.Row - 1
			}
			if row+1 < len(b.Lines) {
				at = buffer.Pos{Row: row + 1}
			} else {
				at = buffer.Pos{Row: row, Col: len(b.Lines[row])}
				text = "\n" + strings.TrimSuffix(text, "\n")
			}
		} else {
			at = buffer.Pos{Row: s.Row}
		}
	} else if after {
		at = e
	} else {
		at = s
	}
	b.InsertAt(at, text)
	end := b.Cur
	b.SelectRange(at, end)
	a.follow = true
}

func (a *App) indentLines(b *buffer.Buffer, in bool) func() {
	return func() {
		if !in {
			b.Outdent()
			return
		}
		s, e, ok := b.Selection()
		if !ok {
			s, e = b.Cur, b.Cur
		}
		if e.Col == 0 && e.Row > s.Row {
			e.Row--
		}
		unit := b.IndentUnit()
		var edits []buffer.Edit
		for r := s.Row; r <= e.Row; r++ {
			if len(b.Lines[r]) > 0 {
				edits = append(edits, buffer.Edit{Start: buffer.Pos{Row: r}, End: buffer.Pos{Row: r}, Text: unit})
			}
		}
		anchor, cur := b.Anchor, b.Cur
		b.ApplyEdits(edits)
		b.Cur = b.ClampPos(cur)
		if anchor != nil {
			p := b.ClampPos(*anchor)
			b.Anchor = &p
		}
	}
}

func (a *App) changeCase(b *buffer.Buffer, key string) {
	s, e := b.SelectionOrChar()
	text := b.TextRange(s, e)
	var out string
	switch key {
	case "`":
		out = strings.ToLower(text)
	case "alt+`":
		out = strings.ToUpper(text)
	default:
		var sb strings.Builder
		for _, r := range text {
			if strings.ToUpper(string(r)) == string(r) {
				sb.WriteString(strings.ToLower(string(r)))
			} else {
				sb.WriteString(strings.ToUpper(string(r)))
			}
		}
		out = sb.String()
	}
	_, _, hadSel := b.Selection()
	b.DeleteRange(s, e)
	b.InsertAt(s, out)
	if hadSel {
		b.SelectRange(s, b.Cur)
	} else {
		b.Cur = s
	}
}

func (a *App) searchNext(backward bool) {
	t := a.cur()
	if t == nil || t.kind != tabText || a.lastSearch == "" {
		return
	}
	if a.searchBackward {
		backward = !backward
	}
	b := t.buf
	from := b.Cur
	if s, _, ok := b.Selection(); ok && backward {
		from = s
	}
	p, ok := b.Find(a.lastSearch, from, backward)
	if !ok {
		a.fail(errString("見つかりません: " + a.lastSearch))
		return
	}
	if a.mode == modeSelect && b.Anchor != nil {
		b.Cur = buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(a.lastSearch))}
	} else {
		b.SelectRange(p, buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(a.lastSearch))})
	}
	a.follow = true
}

type errString string

func (e errString) Error() string { return string(e) }

// bufferPicker lists open tabs (Helix "Space b").
func (a *App) bufferPicker() tea.Cmd {
	items := make([]paletteItem, len(a.tabs))
	for i, t := range a.tabs {
		label := t.title()
		if t.dirty() {
			label += " ●"
		}
		items[i] = paletteItem{label: label, detail: a.rel(t.path())}
	}
	return a.pal.pick("バッファ", items, func(i int) tea.Cmd {
		a.active = i
		a.follow = true
		return a.setFocus(focusEditor)
	})
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// selectMatches turns the selections into one cursor per match of the
// regular expression pattern (Helix "s").
func (a *App) selectMatches(pattern string) {
	t := a.curText()
	if t == nil || pattern == "" {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		a.fail(err)
		return
	}
	if err := t.buf.SelectMatches(re); err != nil {
		a.fail(err)
		return
	}
	a.mode = modeNormal
	a.follow = true
	a.info("%d 個のカーソル", len(t.buf.Extra())+1)
}

// cursorCell is where a cursor is drawn: Helix draws it on the last
// selected character when the selection extends forwards.
func (a *App) cursorCell(b *buffer.Buffer, anchor *buffer.Pos, cur buffer.Pos) buffer.Pos {
	if a.mode == modeInsert || anchor == nil || !anchor.Less(cur) {
		return cur
	}
	if cur.Col > 0 {
		cur.Col--
	} else if cur.Row > 0 {
		cur.Row--
		cur.Col = len(b.Lines[cur.Row])
	}
	return cur
}
