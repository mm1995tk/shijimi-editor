package ui

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
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

// fullPreviewKey reports whether a key may run while the Markdown preview
// fills the editor area: scrolling, counts, the leader and command line, and
// ctrl chords. Anything else would edit a buffer that is not visible.
func (a *App) fullPreviewKey(key string) bool {
	if a.pending == " " {
		return strings.Contains("fbkodDeEmgGvwy?/ta", key) && len(key) == 1
	}
	if a.pending == "space v" {
		return true // review commands do not edit the hidden buffer
	}
	if a.pending == "space w" {
		return true
	}
	if a.pending != "" {
		return a.pending == "g"
	}
	switch key {
	case "j", "k", "up", "down", "G", "g", "pgup", "pgdown", "esc", ":", "space":
		return true
	}
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		return true
	}
	return strings.HasPrefix(key, "ctrl+")
}

// fullPreviewEditorKey is fullPreviewKey for the modeless (vscode) keymap,
// where plain characters would be typed into the hidden buffer.
func fullPreviewEditorKey(msg Key) bool {
	switch msg.String() {
	case "up", "down", "pgup", "pgdown", "home", "end", "ctrl+home", "ctrl+end", "esc":
		return true
	}
	return false
}

// handleHelixKey implements Helix-style modal editing for text tabs.
func (a *App) handleHelixKey(msg Key) tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	if a.fullPreview(t) {
		a.mode = modeNormal // e.g. after switching back to this tab from insert mode
		if !a.fullPreviewKey(msg.String()) {
			a.pending, a.count = "", 0
			a.infoText(i18n.T("app.fullpreview"))
			return nil
		}
	}
	if a.mode == modeInsert {
		return a.handleInsertKey(msg)
	}
	b := t.buf
	key := msg.String()
	a.follow = a.pending != "" || key != "z" // the z prefix alone must not pull the view back to the cursor
	ext := a.mode == modeSelect

	// pending multi-key sequences
	if a.pending != "" {
		p := a.pending
		a.pending = ""
		if b.HasExtra() && (strings.Contains("ftFTr", p) || p == "g" && strings.Contains("hls", key) || p == "m i" || p == "m a") {
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

	km := a.keymap.normal
	if ext {
		km = a.keymap.sel
	}
	if bd, ok := km[key]; ok {
		return a.runBinding(bd, b, ext, count, hadCount)
	}

	switch key {
	case "C", "alt+C":
		dir := 1
		if key == "alt+C" {
			dir = -1
		}
		for range count {
			b.CopySelection(dir)
		}
		return nil
	case ",":
		b.ClearExtra()
		return nil
	case "s":
		if _, _, ok := b.Selection(); !ok && !b.HasExtra() {
			a.infoText(i18n.T("helix.selectfirst"))
			return nil
		}
		return a.openCmdline("s")
	}

	if key == string(lineFlipKey) {
		a.flipLineNumbers()
		return nil
	}

	if (key == "ctrl+d" || key == "ctrl+u" || key == "ctrl+alt+d" || key == "ctrl+alt+u") && !b.HasExtra() {
		dir := 1
		if key == "ctrl+u" || key == "ctrl+alt+u" {
			dir = -1
		}
		return a.halfPage(b, dir, ext, msg)
	}

	return a.helixNormalKey(b, key, ext, count, hadCount)
}

func (a *App) helixNormalKey(b *buffer.Buffer, key string, ext bool, count int, hadCount bool) tea.Cmd {
	switch key {
	// --- movement ---
	case "alt+pgup", "alt+pgdown": // scroll sideways; the cursor stays
		dir := 1
		if key == "alt+pgup" {
			dir = -1
		}
		a.scrollHalfScreen(a.cur(), dir)
	case "ctrl+u":
		b.MoveLines(-a.pageSize()/2, ext)
	case "ctrl+d":
		b.MoveLines(a.pageSize()/2, ext)

	// --- search ---
	case "/", "?":
		return a.openCmdline(key)
	case "n":
		a.pushJump()
		a.searchNext(false)
	case "N":
		a.pushJump()
		a.searchNext(true)
	case "*":
		if s := b.SelectedText(); s != "" && !strings.Contains(s, "\n") {
			a.pushJump()
			a.lastSearch = s
			a.info(i18n.T("helix.search"), s)
		}

	// --- prefixes ---
	case "g", "m", "]", "[", "f", "t", "F", "T", "r", "z", "space":
		a.pending = key
		if key == "space" {
			a.pending = " "
		}
		if hadCount && key == "g" {
			a.pushJump()
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
	a.recIns = nil
	a.mode = modeInsert
	return nil
}

func (a *App) helixPending(p, key string, msg Key) tea.Cmd {
	t := a.cur()
	b := t.buf
	ext := a.mode == modeSelect
	ch := []rune(key)
	single := utf8.RuneCountInString(typed(msg)) == 1
	var r rune
	if key == "space" {
		r = ' '
	} else if len(ch) > 0 {
		r = ch[0]
	}
	switch p {
	case "z": // view: scroll sideways, the cursor stays
		switch key {
		case "h", "left":
			a.scrollHalfScreen(t, -1)
		case "l", "right":
			a.scrollHalfScreen(t, 1)
		}
	case "g":
		switch key {
		case "g":
			a.pushJump()
			b.MoveCursor(buffer.DocStart, ext)
		case "e":
			a.pushJump()
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
		case "a":
			a.switchLast()
		case "t":
			b.SetCursor(b.ScrollRow, 0, ext)
		case "b":
			b.SetCursor(b.ScrollRow+a.layout().editH-1, 0, ext)
		case "c":
			b.SetCursor(b.ScrollRow+a.layout().editH/2, 0, ext)
		}
	case "m":
		switch key {
		case "m":
			b.MatchBracket(ext)
		case "i", "a":
			a.pending = "m " + key // wait for the text object
		}
	case "m i", "m a":
		if single && !b.SelectTextObject(r, p == "m a") {
			a.info(i18n.T("helix.notfoundc"), r)
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
				a.info(i18n.T("helix.notfoundc"), r)
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
	case "space v":
		return a.reviewKey(key)
	case "space w":
		return a.windowKey(key)
	}
	return nil
}

// spaceMode handles "Space <key>" commands (Helix's space menu).
func (a *App) spaceMode(key string) tea.Cmd {
	if a.single {
		switch key {
		case "f", "b", "d", "D", "e", "E", "g", "G", "v", "w", "i", "?", "/", "t", "a", "s":
			return nil
		}
	}
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
	case ".":
		return a.codeAction()
	case "s":
		return a.documentSymbols()
	case "S":
		return a.workspaceSymbols()
	case "d", "D":
		return a.diagnosticPicker()
	case "e":
		a.showSide = true
		return a.setFocus(focusExplorer)
	case "E":
		if a.gd.on {
			return a.focusChangesList()
		}
		a.showSide = true
		return a.setFocus(focusExplorer)
	case "m":
		return a.togglePreview()
	case "g":
		return a.openDiff(basePR)
	case "G":
		return a.openDiff(baseHead)
	case "v":
		return a.openReviewMenu()
	case "w":
		a.pending = "space w"
		return nil
	case "i":
		return a.toggleAgentPanel()
	case "y":
		if t := a.cur(); t != nil && t.kind == tabText {
			s, e := t.buf.SelectionOrChar()
			a.copyText(t.buf.TextRange(s, e))
			a.infoText(i18n.T("helix.copied"))
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
		return a.openSearch(fQuery)
	case "c":
		a.toggleComment()
	case "t":
		return a.toggleTerm()
	case "a":
		return a.sendContext()
	}
	return nil
}

func (a *App) handleInsertKey(msg Key) tea.Cmd {
	cmd := a.insertKey(msg)
	if a.recIns != nil && !a.replaying {
		if a.mode == modeInsert {
			a.recIns.steps = append(a.recIns.steps, insertStep{key: msg})
		} else {
			a.lastIns, a.recIns = a.recIns, nil // left insert mode: the record is complete
		}
	}
	return cmd
}

func (a *App) insertKey(msg Key) tea.Cmd {
	t := a.cur()
	b := t.buf
	if bd, ok := a.keymap.insert[msg.String()]; ok {
		return a.runBinding(bd, b, false, 1, false)
	}
	switch msg.String() {
	case "esc":
		a.mode = modeNormal
		a.comp = nil
		b.ForEachCursor(b.ClearSelection)
		return nil
	case "ctrl+x", "ctrl+space":
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
	if body, linewise := strings.CutSuffix(text, "\n"); linewise {
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
				text = "\n" + body
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
		a.fail(errString(i18n.T("app.notfound") + a.lastSearch))
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
	return a.pal.pick(i18n.T("helix.buffers"), items, func(i int) tea.Cmd {
		a.pushJump()
		a.showInPane(i)
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
	a.info(i18n.T("helix.cursors"), len(t.buf.Extra())+1)
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
