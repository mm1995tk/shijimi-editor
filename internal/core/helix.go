package core

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// EditMode is the Helix mode: normal, insert or select.
type EditMode int

const (
	ModeNormal EditMode = iota
	ModeInsert
	ModeSelect
)

func (m EditMode) label() string {
	switch m {
	case ModeInsert:
		return "INS"
	case ModeSelect:
		return "SEL"
	}
	return "NOR"
}

// fullPreviewKey reports whether a key may run while the Markdown preview
// fills the editor area: scrolling, counts, the leader and command line, and
// ctrl chords. Anything else would edit a buffer that is not visible.
func (a *Core) fullPreviewKey(key string) bool {
	if a.Pending == " " {
		return strings.Contains("fbkodDeEmgGvwy?/ta", key) && len(key) == 1
	}
	if a.Pending == "space v" {
		return true // review commands do not edit the hidden buffer
	}
	if a.Pending == "space w" {
		return true
	}
	if a.Pending != "" {
		return a.Pending == "g"
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

// HandleHelixKey implements Helix-style modal editing for text tabs.
func (a *Core) HandleHelixKey(msg Key) []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	if a.FullPreview(t) {
		a.Mode = ModeNormal // e.g. after switching back to this tab from insert mode
		if !a.fullPreviewKey(msg.String()) {
			a.Pending, a.count = "", 0
			a.InfoText(i18n.T("app.fullpreview"))
			return nil
		}
	}
	if a.Mode == ModeInsert {
		return a.handleInsertKey(msg)
	}
	b := t.Buf
	key := msg.String()
	a.Follow = a.Pending != "" || key != "z" // the z prefix alone must not pull the view back to the cursor
	ext := a.Mode == ModeSelect

	// pending multi-key sequences
	if a.Pending != "" {
		p := a.Pending
		a.Pending = ""
		if b.HasExtra() && (strings.Contains("ftFTr", p) || p == "g" && strings.Contains("hls", key) || p == "m i" || p == "m a") {
			var cmd []Effect
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
			a.InfoText(i18n.T("helix.selectfirst"))
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

func (a *Core) helixNormalKey(b *buffer.Buffer, key string, ext bool, count int, hadCount bool) []Effect {
	switch key {
	// --- movement ---
	case "alt+pgup", "alt+pgdown": // scroll sideways; the cursor stays
		dir := 1
		if key == "alt+pgup" {
			dir = -1
		}
		a.scrollHalfScreen(a.Cur(), dir)
	case "ctrl+u":
		b.MoveLines(-a.PageSize()/2, ext)
	case "ctrl+d":
		b.MoveLines(a.PageSize()/2, ext)

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
			a.Info(i18n.T("helix.search"), s)
		}

	// --- prefixes ---
	case "g", "m", "]", "[", "f", "t", "F", "T", "r", "z", "space":
		a.Pending = key
		if key == "space" {
			a.Pending = " "
		}
		if hadCount && key == "g" {
			a.pushJump()
			b.GotoLine(count)
			a.Pending = ""
		}
		return nil
	case ":":
		return a.openCmdline(":")
	case "K":
		return a.hover()
	}
	return nil
}

// PageSize is how many rows a page scroll moves.
func (a *Core) PageSize() int { return max(1, a.EditRows()-2) }

func (a *Core) enterInsert() []Effect {
	a.recIns = nil
	a.Mode = ModeInsert
	return nil
}

func (a *Core) helixPending(p, key string, msg Key) []Effect {
	t := a.Cur()
	b := t.Buf
	ext := a.Mode == ModeSelect
	ch := []rune(key)
	single := utf8.RuneCountInString(Typed(msg)) == 1
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
			a.SwitchLast()
		case "t":
			b.SetCursor(b.ScrollRow, 0, ext)
		case "b":
			b.SetCursor(b.ScrollRow+a.EditRows()-1, 0, ext)
		case "c":
			b.SetCursor(b.ScrollRow+a.EditRows()/2, 0, ext)
		}
	case "m":
		switch key {
		case "m":
			b.MatchBracket(ext)
		case "i", "a":
			a.Pending = "m " + key // wait for the text object
		}
	case "m i", "m a":
		if single && !b.SelectTextObject(r, p == "m a") {
			a.Info(i18n.T("helix.notfoundc"), r)
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
				a.Info(i18n.T("helix.notfoundc"), r)
			}
		}
	case "r":
		if single {
			b.ReplaceChars(r)
		} else if key == "enter" {
			b.ReplaceChars('\n')
		}
	case " ":
		return a.SpaceMode(key)
	case "space v":
		return a.ReviewKey(key)
	case "space w":
		return a.windowKey(key)
	}
	return nil
}

// SpaceMode handles "Space <key>" commands (Helix's space menu).
func (a *Core) SpaceMode(key string) []Effect {
	if a.single {
		switch key {
		case "f", "b", "d", "D", "e", "E", "g", "G", "v", "w", "i", "?", "/", "t", "a", "s":
			return nil
		}
	}
	switch key {
	case "f":
		return a.Pal.Show(a.Root, "")
	case "b":
		return a.BufferPicker()
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
		a.SetSide(true)
		return a.SetFocus(FocusExplorer)
	case "E":
		if a.Gd.On {
			return a.focusChangesList()
		}
		a.SetSide(true)
		return a.SetFocus(FocusExplorer)
	case "m":
		return a.TogglePreview()
	case "g":
		return a.OpenDiff(BasePR)
	case "G":
		return a.OpenDiff(BaseHead)
	case "v":
		return a.openReviewMenu()
	case "w":
		a.Pending = "space w"
		return nil
	case "i":
		return a.toggleAgentPanel()
	case "y":
		if t := a.Cur(); t != nil && t.Kind == TabText {
			s, e := t.Buf.SelectionOrChar()
			a.CopyText(t.Buf.TextRange(s, e))
			a.InfoText(i18n.T("helix.copied"))
		}
	case "p", "P":
		if t := a.Cur(); t != nil && t.Kind == TabText {
			if s, ok := ReadSystemClipboard(); ok {
				a.paste(t.Buf, s, key == "p")
			}
		}
	case "?":
		return a.Pal.Show(a.Root, ">")
	case "/":
		return a.OpenSearch(FQuery)
	case "c":
		a.toggleComment()
	case "t":
		return a.ToggleTerm()
	case "a":
		return a.SendContext()
	}
	return nil
}

func (a *Core) handleInsertKey(msg Key) []Effect {
	cmd := a.insertKey(msg)
	if a.recIns != nil && !a.replaying {
		if a.Mode == ModeInsert {
			a.recIns.steps = append(a.recIns.steps, insertStep{key: msg})
		} else {
			a.lastIns, a.recIns = a.recIns, nil // left insert mode: the record is complete
		}
	}
	return cmd
}

func (a *Core) insertKey(msg Key) []Effect {
	t := a.Cur()
	b := t.Buf
	if bd, ok := a.keymap.insert[msg.String()]; ok {
		return a.runBinding(bd, b, false, 1, false)
	}
	switch msg.String() {
	case "esc":
		a.Mode = ModeNormal
		a.Comp = nil
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

func (a *Core) paste(b *buffer.Buffer, text string, after bool) {
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
	a.Follow = true
}

func (a *Core) indentLines(b *buffer.Buffer, in bool) func() {
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

func (a *Core) changeCase(b *buffer.Buffer, key string) {
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

func (a *Core) searchNext(backward bool) {
	t := a.Cur()
	if t == nil || t.Kind != TabText || a.lastSearch == "" {
		return
	}
	if a.searchBackward {
		backward = !backward
	}
	b := t.Buf
	from := b.Cur
	if s, _, ok := b.Selection(); ok && backward {
		from = s
	}
	p, ok := b.Find(a.lastSearch, from, backward)
	if !ok {
		a.Fail(errString(i18n.T("app.notfound") + a.lastSearch))
		return
	}
	if a.Mode == ModeSelect && b.Anchor != nil {
		b.Cur = buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(a.lastSearch))}
	} else {
		b.SelectRange(p, buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(a.lastSearch))})
	}
	a.Follow = true
}

type errString string

func (e errString) Error() string { return string(e) }

// BufferPicker lists open tabs (Helix "Space b").
func (a *Core) BufferPicker() []Effect {
	items := make([]PaletteItem, len(a.Tabs))
	for i, t := range a.Tabs {
		label := t.Title()
		if t.dirty() {
			label += " ●"
		}
		items[i] = PaletteItem{Label: label, Detail: a.Rel(t.Path())}
	}
	return a.Pal.Pick(i18n.T("helix.buffers"), items, func(i int) []Effect {
		a.pushJump()
		a.showInPane(i)
		return a.SetFocus(FocusEditor)
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
func (a *Core) selectMatches(pattern string) {
	t := a.CurText()
	if t == nil || pattern == "" {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		a.Fail(err)
		return
	}
	if err := t.Buf.SelectMatches(re); err != nil {
		a.Fail(err)
		return
	}
	a.Mode = ModeNormal
	a.Follow = true
	a.Info(i18n.T("helix.cursors"), len(t.Buf.Extra())+1)
}

// cursorCell is where a cursor is drawn: Helix draws it on the last
// selected character when the selection extends forwards.
func (a *Core) cursorCell(b *buffer.Buffer, anchor *buffer.Pos, cur buffer.Pos) buffer.Pos {
	if a.Mode == ModeInsert || anchor == nil || !anchor.Less(cur) {
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
