// Package ui implements the gh-editor terminal IDE.
package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

type focus int

const (
	focusEditor focus = iota
	focusExplorer
	focusTerminal
)

type command struct {
	name string
	key  string
	run  func(a *App) tea.Cmd
}

type span struct{ x0, x1 int }

// App is the root Bubble Tea model.
type App struct {
	root     string
	w, h     int
	focus    focus
	explorer *explorer
	term     *terminal
	pal      *palette
	tabs     []*tab
	active   int
	branch   string

	showSide bool
	showTerm bool

	findOpen   bool
	findInput  textinput.Model
	findOrigin buffer.Pos

	clipboard    string
	msg          string
	msgErr       bool
	confirmQuit  bool
	confirmClose bool
	follow       bool // scroll to the cursor on next render
	dragging     bool

	tabSpans  []span
	tabOffset int
	commands  []command
}

// New creates the app rooted at dir, optionally opening file.
func New(dir, file string) *App {
	abs, _ := filepath.Abs(dir)
	fi := textinput.New()
	fi.Prompt = ""
	fi.Placeholder = "検索"
	styleInput(&fi, cPaletteBg)
	a := &App{
		root:      abs,
		explorer:  newExplorer(abs),
		term:      newTerminal(abs),
		pal:       newPalette(),
		showSide:  true,
		findInput: fi,
		focus:     focusExplorer,
	}
	styleInput(&a.pal.input, cPaletteBg)
	styleInput(&a.term.input, cEditorBg)
	a.commands = defaultCommands()
	if out, err := exec.Command("git", "-C", abs, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		a.branch = strings.TrimSpace(string(out))
	}
	if file != "" && a.openFile(file) {
		a.focus = focusEditor
	}
	return a
}

func styleInput(ti *textinput.Model, bg string) {
	ti.TextStyle = bgfg(bg, cEditorFg)
	ti.PlaceholderStyle = bgfg(bg, "#6e6e6e")
	ti.PromptStyle = bgfg(bg, cEditorFg)
	ti.Cursor.Style = bgfg("#aeafad", bg)
	ti.Cursor.TextStyle = bgfg(bg, cEditorFg)
}

func defaultCommands() []command {
	return []command{
		{"ファイル: 保存", "Ctrl+S", func(a *App) tea.Cmd { return a.save() }},
		{"ファイル: 名前を付けて保存", "", func(a *App) tea.Cmd { return a.saveAs() }},
		{"ファイル: すべて保存", "", func(a *App) tea.Cmd { return a.saveAll() }},
		{"ファイル: 新しいファイル", "Ctrl+N", func(a *App) tea.Cmd { return a.newFile() }},
		{"ファイル: ファイルを開く", "Ctrl+P", func(a *App) tea.Cmd { return a.pal.show(a.root, "") }},
		{"表示: タブを閉じる", "Ctrl+W", func(a *App) tea.Cmd { a.closeTab(true); return nil }},
		{"表示: サイドバーの表示切替", "Ctrl+B", func(a *App) tea.Cmd { a.toggleSide(); return nil }},
		{"表示: ターミナルの表示切替", "Ctrl+J", func(a *App) tea.Cmd { return a.toggleTerm() }},
		{"表示: エクスプローラーへ移動", "Ctrl+E", func(a *App) tea.Cmd { a.showSide = true; a.setFocus(focusExplorer); return nil }},
		{"表示: 次のタブ", "Ctrl+PgDn", func(a *App) tea.Cmd { a.switchTab(1); return nil }},
		{"表示: 前のタブ", "Ctrl+PgUp", func(a *App) tea.Cmd { a.switchTab(-1); return nil }},
		{"編集: 検索", "Ctrl+F", func(a *App) tea.Cmd { return a.openFind() }},
		{"編集: 行へ移動", "Ctrl+G", func(a *App) tea.Cmd { return a.pal.show(a.root, ":") }},
		{"編集: 元に戻す", "Ctrl+Z", func(a *App) tea.Cmd { a.withBuf(func(b *buffer.Buffer) { b.Undo() }); return nil }},
		{"編集: やり直し", "Ctrl+Y", func(a *App) tea.Cmd { a.withBuf(func(b *buffer.Buffer) { b.Redo() }); return nil }},
		{"編集: すべて選択", "Ctrl+A", func(a *App) tea.Cmd { a.withBuf(func(b *buffer.Buffer) { b.SelectAll() }); return nil }},
		{"編集: 行コメントの切替", "Ctrl+/", func(a *App) tea.Cmd { a.toggleComment(); return nil }},
		{"エクスプローラー: 最新の情報に更新", "R", func(a *App) tea.Cmd { a.explorer.refresh(); return nil }},
		{"エクスプローラー: 新しいファイル", "n", func(a *App) tea.Cmd { return a.newFileIn(a.explorer.targetDir()) }},
		{"エクスプローラー: 新しいフォルダー", "N", func(a *App) tea.Cmd { return a.newFolder() }},
		{"エクスプローラー: 名前の変更", "r", func(a *App) tea.Cmd { return a.rename() }},
		{"終了", "Ctrl+Q", func(a *App) tea.Cmd { return a.quit() }},
	}
}

func (a *App) Init() tea.Cmd { return nil }

// ---- layout ----

type layout struct {
	sideW          int
	rightX, rightW int
	editY, editH   int
	findY          int
	panelY, panelH int
	statusY        int
}

func (a *App) layout() layout {
	var l layout
	l.statusY = a.h - 1
	if a.showSide {
		l.sideW = clamp(a.w/4, 18, 36)
		if l.sideW > a.w/2 {
			l.sideW = a.w / 2
		}
		l.rightX = l.sideW + 1
	}
	l.rightW = max(0, a.w-l.rightX)
	l.editY = 1
	l.findY = -1
	if a.findOpen {
		l.findY = 1
		l.editY = 2
	}
	if a.showTerm {
		l.panelH = clamp((a.h-1)*2/5, 6, max(6, a.h-8))
	}
	l.editH = max(0, a.h-1-l.editY-l.panelH)
	l.panelY = l.editY + l.editH
	return l
}

// ---- helpers ----

func (a *App) cur() *tab {
	if a.active < 0 || a.active >= len(a.tabs) {
		return nil
	}
	return a.tabs[a.active]
}

func (a *App) withBuf(f func(b *buffer.Buffer)) {
	if t := a.cur(); t != nil {
		f(t.buf)
		a.follow = true
	}
}

func (a *App) info(format string, args ...any) {
	a.msg, a.msgErr = fmt.Sprintf(format, args...), false
}

func (a *App) fail(err error) {
	a.msg, a.msgErr = err.Error(), true
}

func (a *App) setFocus(f focus) tea.Cmd {
	a.focus = f
	a.term.blur()
	if f == focusTerminal {
		return a.term.focus()
	}
	return nil
}

func (a *App) rel(p string) string {
	if r, err := filepath.Rel(a.root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

func (a *App) openFile(path string) bool {
	abs, _ := filepath.Abs(path)
	for i, t := range a.tabs {
		if t.buf.Path == abs {
			a.active = i
			a.explorer.reveal(abs)
			return true
		}
	}
	if st, err := os.Stat(abs); err == nil && st.Size() > 10<<20 {
		a.fail(fmt.Errorf("ファイルが大きすぎます (10MB 超): %s", a.rel(abs)))
		return false
	}
	b, err := buffer.Open(abs)
	if err != nil {
		a.fail(err)
		return false
	}
	a.tabs = append(a.tabs, newTab(b))
	a.active = len(a.tabs) - 1
	a.explorer.reveal(abs)
	a.follow = true
	return true
}

func (a *App) save() tea.Cmd {
	t := a.cur()
	if t == nil {
		return nil
	}
	if t.buf.Path == "" {
		return a.saveAs()
	}
	if err := t.buf.Save(); err != nil {
		a.fail(err)
		return nil
	}
	a.info("保存しました: %s", a.rel(t.buf.Path))
	return nil
}

func (a *App) saveAll() tea.Cmd {
	n := 0
	for _, t := range a.tabs {
		if t.buf.Dirty && t.buf.Path != "" {
			if err := t.buf.Save(); err != nil {
				a.fail(err)
				return nil
			}
			n++
		}
	}
	a.info("%d 個のファイルを保存しました", n)
	return nil
}

func (a *App) saveAs() tea.Cmd {
	t := a.cur()
	if t == nil {
		return nil
	}
	initial := a.explorer.targetDir() + string(filepath.Separator)
	if t.buf.Path != "" {
		initial = t.buf.Path
	}
	return a.pal.ask("保存先", initial, func(v string) tea.Cmd {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(a.root, v)
		}
		t.buf.Path = v
		t.hl = newHighlighter(v)
		if err := t.buf.Save(); err != nil {
			a.fail(err)
			return nil
		}
		a.explorer.refresh()
		a.explorer.reveal(v)
		a.info("保存しました: %s", a.rel(v))
		return nil
	})
}

func (a *App) newFile() tea.Cmd {
	a.tabs = append(a.tabs, newTab(buffer.New()))
	a.active = len(a.tabs) - 1
	return a.setFocus(focusEditor)
}

func (a *App) newFileIn(dir string) tea.Cmd {
	return a.pal.ask("新しいファイル", dir+string(filepath.Separator), func(v string) tea.Cmd {
		v = strings.TrimSpace(v)
		if v == "" || strings.HasSuffix(v, string(filepath.Separator)) {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(a.root, v)
		}
		if _, err := os.Stat(v); err != nil {
			if err := os.MkdirAll(filepath.Dir(v), 0o755); err != nil {
				a.fail(err)
				return nil
			}
			if err := os.WriteFile(v, nil, 0o644); err != nil {
				a.fail(err)
				return nil
			}
		}
		a.explorer.refresh()
		if a.openFile(v) {
			return a.setFocus(focusEditor)
		}
		return nil
	})
}

func (a *App) newFolder() tea.Cmd {
	return a.pal.ask("新しいフォルダー", a.explorer.targetDir()+string(filepath.Separator), func(v string) tea.Cmd {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(a.root, v)
		}
		if err := os.MkdirAll(v, 0o755); err != nil {
			a.fail(err)
			return nil
		}
		a.explorer.refresh()
		a.explorer.reveal(v)
		return nil
	})
}

func (a *App) rename() tea.Cmd {
	n := a.explorer.selected()
	if n == nil {
		return nil
	}
	old := n.path
	return a.pal.ask("新しい名前", old, func(v string) tea.Cmd {
		v = strings.TrimSpace(v)
		if v == "" || v == old {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(filepath.Dir(old), v)
		}
		if _, err := os.Stat(v); err == nil {
			a.fail(fmt.Errorf("既に存在します: %s", a.rel(v)))
			return nil
		}
		if err := os.Rename(old, v); err != nil {
			a.fail(err)
			return nil
		}
		for _, t := range a.tabs {
			if t.buf.Path == old {
				t.buf.Path = v
				t.hl = newHighlighter(v)
			} else if strings.HasPrefix(t.buf.Path, old+string(filepath.Separator)) {
				t.buf.Path = v + t.buf.Path[len(old):]
			}
		}
		a.explorer.refresh()
		a.explorer.reveal(v)
		a.info("名前を変更しました: %s", a.rel(v))
		return nil
	})
}

func (a *App) closeTab(confirm bool) {
	t := a.cur()
	if t == nil {
		return
	}
	if confirm && t.buf.Dirty && !a.confirmClose {
		a.confirmClose = true
		a.msg, a.msgErr = "未保存の変更があります。もう一度 Ctrl+W で破棄して閉じます", true
		return
	}
	a.confirmClose = false
	a.tabs = append(a.tabs[:a.active], a.tabs[a.active+1:]...)
	if a.active >= len(a.tabs) {
		a.active = len(a.tabs) - 1
	}
	if len(a.tabs) == 0 && a.focus == focusEditor {
		a.focus = focusExplorer
	}
}

func (a *App) switchTab(d int) {
	if len(a.tabs) == 0 {
		return
	}
	a.active = (a.active + d + len(a.tabs)) % len(a.tabs)
	a.follow = true
}

func (a *App) toggleSide() {
	a.showSide = !a.showSide
	if !a.showSide && a.focus == focusExplorer {
		a.focus = focusEditor
	}
}

func (a *App) toggleTerm() tea.Cmd {
	if a.showTerm && a.focus == focusTerminal {
		a.showTerm = false
		return a.setFocus(focusEditor)
	}
	a.showTerm = true
	return a.setFocus(focusTerminal)
}

func (a *App) quit() tea.Cmd {
	dirty := 0
	for _, t := range a.tabs {
		if t.buf.Dirty {
			dirty++
		}
	}
	if dirty > 0 && !a.confirmQuit {
		a.confirmQuit = true
		a.msg, a.msgErr = fmt.Sprintf("未保存のファイルが %d 個あります。もう一度 Ctrl+Q で破棄して終了します", dirty), true
		return nil
	}
	return tea.Quit
}

func (a *App) openFind() tea.Cmd {
	t := a.cur()
	if t == nil {
		return nil
	}
	a.findOpen = true
	a.focus = focusEditor
	a.findOrigin = t.buf.Cur
	if sel := t.buf.SelectedText(); sel != "" && !strings.Contains(sel, "\n") {
		a.findInput.SetValue(sel)
	}
	a.findInput.CursorEnd()
	return a.findInput.Focus()
}

func (a *App) closeFind() {
	a.findOpen = false
	a.findInput.Blur()
}

func (a *App) findNext(from buffer.Pos, backward bool) {
	t := a.cur()
	q := a.findInput.Value()
	if t == nil || q == "" {
		return
	}
	p, ok := t.buf.Find(q, from, backward)
	if !ok {
		a.msg, a.msgErr = "見つかりません: "+q, true
		return
	}
	a.msg = ""
	t.buf.SelectRange(p, buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(q))})
	a.follow = true
}

func (a *App) toggleComment() {
	if t := a.cur(); t != nil {
		t.buf.ToggleComment(commentPrefix(t.buf.Path))
		a.follow = true
	}
}

func (a *App) copyText(s string) {
	a.clipboard = s
	writeSystemClipboard(s)
}

func (a *App) pasteText() string {
	if s, ok := readSystemClipboard(); ok {
		return s
	}
	return a.clipboard
}

// ---- update ----

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		a.follow = true
		return a, nil
	case termOutputMsg, termDoneMsg:
		return a, a.term.update(msg)
	case tea.MouseMsg:
		return a, a.handleMouse(msg)
	case tea.KeyMsg:
		return a, a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	a.msg = ""
	if key != "ctrl+q" {
		a.confirmQuit = false
	}
	if key != "ctrl+w" {
		a.confirmClose = false
	}

	if a.pal.open {
		return a.handlePaletteKey(msg)
	}

	switch key {
	case "ctrl+q":
		return a.quit()
	case "ctrl+s":
		return a.save()
	case "ctrl+p":
		return a.pal.show(a.root, "")
	case "f1", "ctrl+k":
		return a.pal.show(a.root, ">")
	case "ctrl+g":
		return a.pal.show(a.root, ":")
	case "ctrl+b":
		a.toggleSide()
		return nil
	case "ctrl+j", "ctrl+@":
		return a.toggleTerm()
	case "ctrl+e":
		if a.focus == focusExplorer && len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
		a.showSide = true
		return a.setFocus(focusExplorer)
	case "ctrl+n":
		return a.newFile()
	case "ctrl+w":
		a.closeTab(true)
		return nil
	case "ctrl+pgdown":
		a.switchTab(1)
		return nil
	case "ctrl+pgup":
		a.switchTab(-1)
		return nil
	case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
		if i := int(key[4] - '1'); i < len(a.tabs) {
			a.active = i
			a.follow = true
			return a.setFocus(focusEditor)
		}
		return nil
	}

	switch a.focus {
	case focusExplorer:
		return a.handleExplorerKey(msg)
	case focusTerminal:
		if key == "esc" {
			return a.setFocus(focusEditor)
		}
		return a.term.update(msg)
	default:
		if a.findOpen {
			return a.handleFindKey(msg)
		}
		return a.handleEditorKey(msg)
	}
}

func (a *App) handlePaletteKey(msg tea.KeyMsg) tea.Cmd {
	p := a.pal
	switch msg.String() {
	case "esc", "ctrl+c":
		p.close()
		return nil
	case "up", "ctrl+p":
		if p.sel > 0 {
			p.sel--
		}
		return nil
	case "down", "ctrl+n":
		if p.sel < len(p.items)-1 {
			p.sel++
		}
		return nil
	case "enter":
		return a.submitPalette()
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.sel = 0
	p.filter(a.commands)
	return cmd
}

func (a *App) submitPalette() tea.Cmd {
	p := a.pal
	v := p.input.Value()
	p.close()
	if p.prompt != "" {
		if p.onSubmit != nil {
			return p.onSubmit(v)
		}
		return nil
	}
	switch {
	case strings.HasPrefix(v, ":"):
		n, err := strconv.Atoi(strings.TrimSpace(v[1:]))
		if err == nil {
			a.withBuf(func(b *buffer.Buffer) { b.GotoLine(n) })
			return a.setFocus(focusEditor)
		}
	case strings.HasPrefix(v, ">"):
		if p.sel < len(p.items) {
			return a.commands[p.items[p.sel].index].run(a)
		}
	default:
		if p.sel < len(p.items) {
			f := p.files[p.items[p.sel].index]
			if a.openFile(filepath.Join(a.root, f)) {
				return a.setFocus(focusEditor)
			}
		}
	}
	return nil
}

func (a *App) handleExplorerKey(msg tea.KeyMsg) tea.Cmd {
	e := a.explorer
	switch msg.String() {
	case "up", "k":
		e.sel = max(0, e.sel-1)
	case "down", "j":
		e.sel = min(len(e.flat)-1, e.sel+1)
	case "pgup":
		e.sel = max(0, e.sel-(a.h-3))
	case "pgdown":
		e.sel = min(len(e.flat)-1, e.sel+(a.h-3))
	case "home", "g":
		e.sel = 0
	case "end", "G":
		e.sel = len(e.flat) - 1
	case "enter", "l", "right", " ":
		n := e.selected()
		if n == nil {
			return nil
		}
		if n.isDir {
			if msg.String() == "right" || msg.String() == "l" {
				if !n.expanded {
					e.toggle(n)
				}
			} else {
				e.toggle(n)
			}
			return nil
		}
		if a.openFile(n.path) && msg.String() == "enter" {
			return a.setFocus(focusEditor)
		}
	case "left", "h":
		e.collapseOrParent()
	case "n":
		return a.newFileIn(e.targetDir())
	case "N":
		return a.newFolder()
	case "r":
		return a.rename()
	case "R":
		e.refresh()
		a.info("エクスプローラーを更新しました")
	case "esc":
		if len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
	}
	return nil
}

func (a *App) handleFindKey(msg tea.KeyMsg) tea.Cmd {
	t := a.cur()
	if t == nil {
		a.closeFind()
		return nil
	}
	switch msg.String() {
	case "esc":
		a.closeFind()
		return nil
	case "enter", "down", "f3":
		a.findNext(t.buf.Cur, false)
		return nil
	case "up", "shift+tab", "shift+f3":
		s, _, ok := t.buf.Selection()
		if !ok {
			s = t.buf.Cur
		}
		a.findNext(s, true)
		return nil
	case "ctrl+f":
		return nil
	}
	before := a.findInput.Value()
	var cmd tea.Cmd
	a.findInput, cmd = a.findInput.Update(msg)
	if a.findInput.Value() != before {
		a.findNext(a.findOrigin, false)
	}
	return cmd
}

func (a *App) handleEditorKey(msg tea.KeyMsg) tea.Cmd {
	t := a.cur()
	if t == nil {
		if msg.String() == "esc" || msg.String() == "tab" {
			a.showSide = true
			return a.setFocus(focusExplorer)
		}
		return nil
	}
	b := t.buf
	a.follow = true
	l := a.layout()
	page := max(1, l.editH-1)

	if msg.Paste {
		s := strings.ReplaceAll(string(msg.Runes), "\r\n", "\n")
		b.InsertBlock(strings.ReplaceAll(s, "\r", "\n"))
		return nil
	}

	switch msg.String() {
	case "esc":
		b.ClearSelection()
	case "left":
		b.MoveCursor(buffer.Left, false)
	case "right":
		b.MoveCursor(buffer.Right, false)
	case "up":
		b.MoveCursor(buffer.Up, false)
	case "down":
		b.MoveCursor(buffer.Down, false)
	case "shift+left":
		b.MoveCursor(buffer.Left, true)
	case "shift+right":
		b.MoveCursor(buffer.Right, true)
	case "shift+up":
		b.MoveCursor(buffer.Up, true)
	case "shift+down":
		b.MoveCursor(buffer.Down, true)
	case "home":
		b.MoveCursor(buffer.Home, false)
	case "end":
		b.MoveCursor(buffer.End, false)
	case "shift+home":
		b.MoveCursor(buffer.Home, true)
	case "shift+end":
		b.MoveCursor(buffer.End, true)
	case "ctrl+left", "alt+b":
		b.MoveCursor(buffer.WordLeft, false)
	case "ctrl+right", "alt+f":
		b.MoveCursor(buffer.WordRight, false)
	case "ctrl+shift+left":
		b.MoveCursor(buffer.WordLeft, true)
	case "ctrl+shift+right":
		b.MoveCursor(buffer.WordRight, true)
	case "ctrl+home":
		b.MoveCursor(buffer.DocStart, false)
	case "ctrl+end":
		b.MoveCursor(buffer.DocEnd, false)
	case "ctrl+shift+home":
		b.MoveCursor(buffer.DocStart, true)
	case "ctrl+shift+end":
		b.MoveCursor(buffer.DocEnd, true)
	case "pgup":
		b.MoveLines(-page, false)
		b.ScrollRow = max(0, b.ScrollRow-page)
	case "pgdown":
		b.MoveLines(page, false)
		b.ScrollRow = min(max(0, len(b.Lines)-1), b.ScrollRow+page)
	case "shift+pgup":
		b.MoveLines(-page, true)
	case "shift+pgdown":
		b.MoveLines(page, true)
	case "alt+up":
		b.MoveLine(-1)
	case "alt+down":
		b.MoveLine(1)
	case "enter":
		b.Newline()
	case "tab":
		b.Indent()
	case "shift+tab":
		b.Outdent()
	case "backspace", "ctrl+h":
		b.Backspace()
	case "delete":
		b.Delete()
	case "ctrl+z":
		b.Undo()
	case "ctrl+y", "ctrl+shift+z":
		b.Redo()
	case "ctrl+a":
		b.SelectAll()
	case "ctrl+c":
		if s := b.SelectedText(); s != "" {
			a.copyText(s)
		} else {
			a.copyText(string(b.Lines[b.Cur.Row]) + "\n")
		}
		a.info("コピーしました")
	case "ctrl+x":
		if s := b.SelectedText(); s != "" {
			a.copyText(s)
			b.DeleteSelection()
		} else {
			a.copyText(b.CutLine())
		}
	case "ctrl+v":
		if s := a.pasteText(); s != "" {
			b.InsertBlock(s)
		}
	case "ctrl+f":
		return a.openFind()
	case "ctrl+_", "ctrl+/":
		a.toggleComment()
	case " ":
		b.Insert(" ")
	default:
		if msg.Type == tea.KeyRunes && !msg.Alt {
			b.Insert(autoPair(b, string(msg.Runes)))
		}
	}
	return nil
}

// autoPair returns the text to insert; currently it passes text through and
// is a hook for future bracket completion.
func autoPair(_ *buffer.Buffer, s string) string { return s }

func (a *App) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if a.pal.open {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			a.pal.close()
		}
		return nil
	}
	l := a.layout()
	x, y := msg.X, msg.Y
	wheel := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		wheel = -3
	case tea.MouseButtonWheelDown:
		wheel = 3
	}
	if wheel != 0 && msg.Action == tea.MouseActionPress {
		switch {
		case a.showSide && x < l.sideW:
			e := a.explorer
			e.scroll = clamp(e.scroll+wheel, 0, max(0, len(e.flat)-(a.h-2)))
			// keep selection inside the viewport so ensureVisible doesn't jump back
			e.sel = clamp(e.sel, e.scroll, e.scroll+a.h-3)
		case a.showTerm && y >= l.panelY && y < l.statusY:
			a.term.scroll = clamp(a.term.scroll-wheel, 0, max(0, len(a.term.lines)-1))
		default:
			if t := a.cur(); t != nil {
				t.buf.ScrollRow = clamp(t.buf.ScrollRow+wheel, 0, max(0, len(t.buf.Lines)-1))
			}
		}
		return nil
	}

	left := msg.Button == tea.MouseButtonLeft
	switch msg.Action {
	case tea.MouseActionRelease:
		a.dragging = false
		return nil
	case tea.MouseActionMotion:
		if a.dragging && left {
			a.mouseToCursor(l, x, y, true)
		}
		return nil
	}
	if !left {
		return nil
	}

	switch {
	case a.showSide && x < l.sideW:
		row := y - 1 + a.explorer.scroll
		if y >= 1 && row < len(a.explorer.flat) {
			a.explorer.sel = row
			n := a.explorer.flat[row]
			if n.isDir {
				a.explorer.toggle(n)
			} else if a.openFile(n.path) {
				return a.setFocus(focusEditor)
			}
		}
		return a.setFocus(focusExplorer)
	case y == 0:
		for i, s := range a.tabSpans {
			if x >= s.x0 && x < s.x1 {
				a.active = i
				a.follow = true
				return a.setFocus(focusEditor)
			}
		}
	case a.showTerm && y >= l.panelY && y < l.statusY:
		return a.setFocus(focusTerminal)
	case y >= l.editY && y < l.editY+l.editH:
		if a.cur() != nil {
			a.mouseToCursor(l, x, y, false)
			a.dragging = true
			return a.setFocus(focusEditor)
		}
	}
	return nil
}

func (a *App) mouseToCursor(l layout, x, y int, extend bool) {
	t := a.cur()
	if t == nil {
		return
	}
	b := t.buf
	if y < l.editY {
		b.MoveLines(-1, extend)
	} else if y >= l.editY+l.editH {
		b.MoveLines(1, extend)
	}
	row := b.ScrollRow + clamp(y-l.editY, 0, max(0, l.editH-1))
	vis := max(0, x-l.rightX-gutterWidth(b)) + b.ScrollCol
	b.SetCursor(row, vis, extend)
	a.follow = true
}

// ---- view ----

func (a *App) View() string {
	if a.w <= 0 || a.h <= 0 {
		return ""
	}
	if a.w < 30 || a.h < 8 {
		return "ウィンドウが小さすぎます"
	}
	l := a.layout()
	if t := a.cur(); t != nil && a.follow {
		ensureCursorVisible(t.buf, l.rightW, l.editH)
		a.follow = false
	}
	if t := a.cur(); t != nil {
		t.buf.ScrollRow = clamp(t.buf.ScrollRow, 0, max(0, len(t.buf.Lines)-1))
	}

	right := make([]string, 0, a.h-1)
	right = append(right, a.renderTabs(l.rightW))
	if a.findOpen {
		right = append(right, a.renderFind(l.rightW))
	}
	if t := a.cur(); t != nil {
		q := ""
		if a.findOpen {
			q = a.findInput.Value()
		}
		right = append(right, renderEditor(t, l.rightW, l.editH, a.focus == focusEditor && !a.findOpen && !a.pal.open, q)...)
	} else {
		right = append(right, renderWelcome(l.rightW, l.editH)...)
	}
	if a.showTerm {
		right = append(right, a.term.render(l.rightW, l.panelH, a.focus == focusTerminal && !a.pal.open)...)
	}

	rows := make([]string, a.h-1)
	var side []string
	activePath := ""
	if t := a.cur(); t != nil {
		activePath = t.buf.Path
	}
	if a.showSide {
		side = a.explorer.render(l.sideW, a.h-1, a.focus == focusExplorer && !a.pal.open, activePath)
	}
	for i := range rows {
		var sb strings.Builder
		if a.showSide {
			sb.WriteString(side[i])
			sb.WriteString(stSeparator.Render("│"))
		}
		if i < len(right) {
			sb.WriteString(right[i])
		}
		rows[i] = sb.String()
	}

	if a.pal.open {
		a.pal.filter(a.commands)
		pw := clamp(a.w*3/5, 30, 90)
		box := a.pal.render(pw)
		rows = overlay(rows, box, (a.w-pw)/2, 1)
	}

	rows = append(rows, a.renderStatus())
	return strings.Join(rows, "\n")
}

func (a *App) renderTabs(w int) string {
	type piece struct {
		text string
		w    int
	}
	pieces := make([]piece, len(a.tabs))
	for i, t := range a.tabs {
		mark := "  "
		if t.buf.Dirty {
			mark = " ●"
		}
		var s string
		label := " " + t.buf.Name() + mark + " "
		switch {
		case i == a.active && t.buf.Dirty:
			s = stTabActive.Render(" "+t.buf.Name()) + stTabDirty.Render(mark) + stTabActive.Render(" ")
		case i == a.active:
			s = stTabActive.Render(label)
		default:
			s = stTabInactive.Render(label)
		}
		pieces[i] = piece{s + stTabBar.Render(" "), ansi.StringWidth(label) + 1}
	}
	// scroll the tab strip so the active tab is visible
	if a.active < a.tabOffset {
		a.tabOffset = a.active
	}
	for {
		total := 0
		for i := a.tabOffset; i <= a.active && i < len(pieces); i++ {
			total += pieces[i].w
		}
		if total <= w || a.tabOffset >= a.active {
			break
		}
		a.tabOffset++
	}
	var sb strings.Builder
	a.tabSpans = make([]span, len(a.tabs))
	x := 0
	for i := a.tabOffset; i < len(pieces); i++ {
		if x+pieces[i].w > w {
			break
		}
		sb.WriteString(pieces[i].text)
		a.tabSpans[i] = span{x0: x + a.layout().rightX, x1: x + a.layout().rightX + pieces[i].w}
		x += pieces[i].w
	}
	if x < w {
		sb.WriteString(stTabBar.Render(strings.Repeat(" ", w-x)))
	}
	return sb.String()
}

func (a *App) renderFind(w int) string {
	label := stFindLabel.Render(" 検索: ")
	hint := stFind.Render("  Enter:次  ↑:前  Esc:閉じる ")
	iw := w - lipgloss.Width(label) - lipgloss.Width(hint)
	a.findInput.Width = max(1, iw-1)
	return label + fitStyled(a.findInput.View(), iw, stFind) + hint
}

func (a *App) renderStatus() string {
	leftParts := []string{}
	if a.branch != "" {
		leftParts = append(leftParts, "⎇ "+a.branch)
	}
	switch a.focus {
	case focusExplorer:
		leftParts = append(leftParts, "エクスプローラー")
	case focusTerminal:
		leftParts = append(leftParts, "ターミナル")
	}
	left := " " + strings.Join(leftParts, "  ")
	right := ""
	if t := a.cur(); t != nil {
		b := t.buf
		indent := "スペース: 4"
		if b.UseTabs {
			indent = "タブ"
		}
		pos := fmt.Sprintf("行 %d、列 %d", b.Cur.Row+1, buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)+1)
		if s := b.SelectedText(); s != "" {
			pos += fmt.Sprintf(" (%d 文字選択)", len([]rune(s)))
		}
		right = fmt.Sprintf("%s   %s   UTF-8   %s ", pos, indent, t.hl.Language())
	}
	st := stStatus
	if a.msg != "" {
		if a.msgErr {
			st = stStatusError
		}
		left += "   " + a.msg
	}
	lw := a.w - ansi.StringWidth(right)
	return st.Render(fit(left, lw) + right)
}
