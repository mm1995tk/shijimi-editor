// Package ui implements the gh-editor terminal IDE.
package ui

import (
	"errors"
	"fmt"
	"github.com/mm1995tk/gh-editor/internal/render"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/config"
	"github.com/mm1995tk/gh-editor/internal/ide"
	"github.com/mm1995tk/gh-editor/internal/lsp"
	"github.com/mm1995tk/gh-editor/internal/proxy"
)

type focus int

const (
	focusEditor focus = iota
	focusExplorer
	focusTerminal
	focusChanges // the CHANGES list in git-diff mode
)

type span struct{ x0, x1 int }

// App is the root Bubble Tea model.
type App struct {
	root      string
	gitTop    string
	cfg       *config.Config
	renderer  render.Renderer // overrides Chromium for the browser tab (tests); nil = from config
	gfx       *kittyGfx       // non-nil on a terminal with Kitty graphics: the browser view is live
	lsp       *lspManager
	w, h      int
	focus     focus
	explorer  *explorer
	treeTicks int // watch ticks since the tree was last reloaded
	term      *terminal
	proxy     *proxy.Manager
	pal       *palette
	search    *searchPanel
	tabs      []*tab
	active    int
	branch    string
	gd        gitDiff

	showSide bool
	showTerm bool

	// pane sizes picked by the user (0 = automatic), see resize.go
	sideWPref  int
	panelHPref int
	previewPct int
	resizeMode bool
	resizing   resizeTarget // border being dragged with the mouse

	// Helix modal state
	helix          bool
	mode           editMode
	pending        string
	count          int
	lastSearch     string
	searchBackward bool

	// bottom command line (":" commands and "/" search)
	cmdOpen  bool
	cmdKind  string
	cmdInput textinput.Model
	cmdFrom  buffer.Pos

	// VS Code-style find bar
	findOpen   bool
	findInput  textinput.Model
	findOrigin buffer.Pos

	comp        *completionPopup
	hoverText   string
	hoverScroll int
	hoverAt     *screenPt // where a mouse-triggered popup is anchored; nil = at the cursor
	hoverWord   wordSpan  // word the pointer's pending or open hover belongs to
	hoverCell   screenPt  // last cell the pointer rested on without a button
	hoverSeq    int       // bumped on every input; stale dwell timers and replies are dropped
	ctx         *ctxPreview

	// Claude Code IDE integration (see ide.go)
	ide     *ide.Server
	props   []*proposal // agent changes waiting for the user's decision
	lastSel *ideSelection
	sentSel *ideSelection

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
	mermaid   map[string]*mermaidJob
}

// New creates the app rooted at dir, optionally opening file.
func New(dir, file string) *App {
	abs, _ := filepath.Abs(dir)
	cfg := config.Load(abs)
	a := &App{
		root:     abs,
		cfg:      cfg,
		lsp:      newLSPManager(cfg, abs),
		explorer: newExplorer(abs),
		term:     newTerminal(abs),
		proxy:    proxy.NewManager(),
		pal:      newPalette(),
		search:   newSearchPanel(cfg.Search),
		showSide: true,
		focus:    focusExplorer,
		helix:    cfg.Editor.Keymap != "vscode",
		gd:       gitDiff{split: true},
	}
	a.explorer.icons = cfg.Editor.FileIcons
	if cfg.Editor.Theme != "" {
		applyTheme(themeOrDefault(cfg.Editor.Theme))
	}
	a.findInput = textinput.New()
	a.findInput.Prompt = ""
	a.findInput.Placeholder = "検索"
	a.cmdInput = textinput.New()
	a.cmdInput.Prompt = ""
	styleInput(&a.findInput, cPaletteBg)
	styleInput(&a.cmdInput, cEditorBg)
	styleInput(&a.pal.input, cPaletteBg)
	a.search.restyle()
	a.commands = defaultCommands()
	if out, err := run(abs, nil, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		a.branch = strings.TrimSpace(string(out))
	}
	if out, err := run(abs, nil, "git", "rev-parse", "--show-toplevel"); err == nil {
		a.gitTop = strings.TrimSpace(string(out))
	}
	if file != "" && a.openFile(file) {
		a.focus = focusEditor
	}
	return a
}

// Close stops language servers; call after the program exits.
func (a *App) Close() {
	a.lsp.stopAll()
	for _, t := range a.tabs {
		if t.kind == tabBrowser {
			t.web.closeLiveNow()
		}
	}
	if a.gfx != nil {
		a.gfx.closing.Wait()
	}
}

func styleInput(ti *textinput.Model, bg string) {
	ti.TextStyle = bgfg(bg, cEditorFg)
	ti.PlaceholderStyle = bgfg(bg, curTheme.Dim)
	ti.PromptStyle = bgfg(bg, cEditorFg)
	// bubbles の cursor は Style に Reverse を重ねて描くので、前景にカーソル色を置く
	ti.Cursor.Style = bgfg(bg, curTheme.Cursor)
	ti.Cursor.TextStyle = bgfg(bg, cEditorFg)
}

func (a *App) Init() tea.Cmd { return tea.Batch(a.lsp.wait(), watchTick()) }

// ---- layout ----

type layout struct {
	sideW          int
	rightX, rightW int
	editY, editH   int
	panelY, panelH int
	statusY        int
}

func (a *App) layout() layout {
	var l layout
	l.statusY = a.h - 1
	if a.showSide {
		l.sideW = a.sideWidth()
		l.rightX = l.sideW + 1
	}
	l.rightW = max(0, a.w-l.rightX)
	l.editY = 1
	if a.findOpen {
		l.editY = 2
	}
	if a.showTerm {
		l.panelH = a.panelHeight()
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

func (a *App) curText() *tab {
	if t := a.cur(); t != nil && t.kind == tabText {
		return t
	}
	return nil
}

func (a *App) withBuf(f func(b *buffer.Buffer)) {
	if t := a.curText(); t != nil {
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

func (a *App) rel(p string) string { return relTo(a.root, p) }

func (a *App) addTab(t *tab) {
	a.tabs = append(a.tabs, t)
	a.active = len(a.tabs) - 1
	a.follow = true
	a.mode = modeNormal
}

func (a *App) openFile(path string) bool {
	abs, _ := filepath.Abs(path)
	for i, t := range a.tabs {
		if t.path() == abs {
			a.active = i
			a.explorer.reveal(abs)
			return true
		}
	}
	if st, err := os.Stat(abs); err == nil {
		if st.IsDir() {
			a.explorer.reveal(abs)
			return false
		}
		if isImage(abs) {
			a.addTab(&tab{kind: tabImage, img: newImageView(abs)})
			a.explorer.reveal(abs)
			return true
		}
		if st.Size() > 10<<20 {
			a.fail(fmt.Errorf("ファイルが大きすぎます (10MB 超): %s", a.rel(abs)))
			return false
		}
	}
	b, err := buffer.Open(abs)
	if err != nil {
		a.fail(err)
		return false
	}
	t := newTab(b)
	a.addTab(t)
	a.attachLSP(t)
	a.explorer.reveal(abs)
	return true
}

func (a *App) save() tea.Cmd {
	t := a.curText()
	if t == nil {
		return nil
	}
	if t.buf.Path == "" {
		return a.saveAs()
	}
	return a.saveTab(t)
}

func (a *App) saveTab(t *tab) tea.Cmd {
	if err := t.buf.Save(); err != nil {
		a.fail(err)
		return nil
	}
	cs, _ := a.lsp.clientsFor(t)
	a.syncLSP()
	for _, c := range cs {
		c.DidSave(t.buf.Path)
	}
	a.explorer.loadGitStatus()
	a.info("保存しました: %s", a.rel(t.buf.Path))
	return a.afterSave()
}

func (a *App) saveAll() tea.Cmd {
	n := 0
	for _, t := range a.tabs {
		if t.dirty() && t.buf.Path != "" {
			a.saveTab(t)
			if a.msgErr {
				return nil
			}
			n++
		}
	}
	a.info("%d 個のファイルを保存しました", n)
	return nil
}

func (a *App) saveAs() tea.Cmd {
	t := a.curText()
	if t == nil {
		return nil
	}
	initial := a.explorer.targetDir() + string(filepath.Separator)
	if t.buf.Path != "" {
		initial = t.buf.Path
	}
	return a.pal.ask("保存先", initial, func(v string) tea.Cmd {
		return a.saveTo(t, v)
	})
}

func (a *App) saveTo(t *tab, v string) tea.Cmd {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if !filepath.IsAbs(v) {
		v = filepath.Join(a.root, v)
	}
	isNew := t.buf.Path != v
	t.buf.Path = v
	t.hl = newHighlighter(v)
	cmd := a.saveTab(t)
	if isNew {
		a.attachLSP(t)
	}
	a.explorer.refresh()
	a.explorer.reveal(v)
	return cmd
}

func (a *App) newFile() tea.Cmd {
	a.addTab(newTab(buffer.New()))
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

func (a *App) renameFile() tea.Cmd {
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
			switch {
			case t.kind == tabText && t.buf.Path == old:
				t.buf.Path = v
				t.hl = newHighlighter(v)
			case t.kind == tabText && strings.HasPrefix(t.buf.Path, old+string(filepath.Separator)):
				t.buf.Path = v + t.buf.Path[len(old):]
			case t.kind == tabImage && t.img.path == old:
				t.img.path = v
			}
		}
		a.explorer.refresh()
		a.explorer.reveal(v)
		a.info("名前を変更しました: %s", a.rel(v))
		return nil
	})
}

func (a *App) deleteFile() tea.Cmd {
	n := a.explorer.selected()
	if n == nil {
		return nil
	}
	target := n.path
	return a.pal.ask(fmt.Sprintf("%s を削除しますか？ y で削除", a.rel(target)), "", func(v string) tea.Cmd {
		if strings.TrimSpace(strings.ToLower(v)) != "y" {
			return nil
		}
		if err := os.RemoveAll(target); err != nil {
			a.fail(err)
			return nil
		}
		a.explorer.refresh()
		a.info("削除しました: %s", a.rel(target))
		return nil
	})
}

func (a *App) closeTab(force bool) {
	t := a.cur()
	if t == nil {
		return
	}
	if !force && t.dirty() && !a.confirmClose {
		a.confirmClose = true
		a.msg, a.msgErr = "未保存の変更があります。もう一度 Ctrl+W（または :bc!）で破棄して閉じます", true
		return
	}
	a.confirmClose = false
	if t.kind == tabText && t.lang != nil {
		a.detachLSP(t)
	}
	if t.kind == tabBrowser {
		t.web.closeLive()
	}
	a.tabs = append(a.tabs[:a.active], a.tabs[a.active+1:]...)
	if a.active >= len(a.tabs) {
		a.active = len(a.tabs) - 1
	}
	a.comp = nil
	a.mode = modeNormal
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
	a.mode = modeNormal
	a.comp = nil
}

func (a *App) toggleSide() {
	a.showSide = !a.showSide
	if !a.showSide && (a.focus == focusExplorer || a.focus == focusChanges) {
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

func (a *App) dirtyCount() int {
	n := 0
	for _, t := range a.tabs {
		if t.dirty() {
			n++
		}
	}
	return n
}

func (a *App) quit() tea.Cmd {
	if d := a.dirtyCount(); d > 0 && !a.confirmQuit {
		a.confirmQuit = true
		a.msg, a.msgErr = fmt.Sprintf("未保存のファイルが %d 個あります。もう一度 Ctrl+Q（または :q!）で破棄して終了します", d), true
		return nil
	}
	a.term.closeAll()
	a.proxy.StopAll()
	return tea.Quit
}

func (a *App) openFind() tea.Cmd {
	t := a.curText()
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
	t := a.curText()
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
	if t := a.curText(); t != nil {
		t.buf.ToggleComment(commentPrefix(t.buf.Path))
		a.follow = true
	}
}

func (a *App) copyText(s string) {
	a.clipboard = s
	writeSystemClipboard(s)
}

// activePath is the file of the current tab, or "" when it has none.
func (a *App) activePath() string {
	if t := a.cur(); t != nil {
		return t.path()
	}
	return ""
}

// copyPath copies p to the clipboard (relative to the root unless abs) and
// shows what was copied.
func (a *App) copyPath(p string, abs bool) {
	if p == "" {
		a.fail(errors.New("パスをコピーできるファイルがありません"))
		return
	}
	if abs {
		if ap, err := filepath.Abs(p); err == nil {
			p = ap
		}
	} else {
		p = a.rel(p)
	}
	a.copyText(p)
	a.info("パスをコピーしました: %s", p)
}

func (a *App) pasteText() string {
	if s, ok := readSystemClipboard(); ok {
		return s
	}
	return a.clipboard
}

// ---- update ----

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case ctlMsg:
		return a, a.handleCtl(m)
	case ideMsg:
		return a, a.handleIDE(m)
	case ideCancelMsg:
		a.cancelProposal(m.reply)
		return a, nil
	}
	cmd := a.update(msg)
	a.syncLSP()
	a.syncLive()
	a.notifySelection()
	return a, tea.Batch(cmd, a.scheduleAutoSave(), a.syncPreview())
}

// autoSaveMsg fires when a tab has been idle for the auto-save delay.
type autoSaveMsg struct {
	tab *tab
	ver int
}

// scheduleAutoSave starts a timer for every named tab edited since the last
// call. The timer is ignored if the buffer changed again before it fires.
func (a *App) scheduleAutoSave() tea.Cmd {
	if !a.cfg.Editor.AutoSave {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range a.tabs {
		if !t.dirty() || t.buf.Path == "" || t.autoVer == t.buf.Version {
			continue
		}
		t.autoVer = t.buf.Version
		m := autoSaveMsg{tab: t, ver: t.buf.Version}
		cmds = append(cmds, tea.Tick(a.cfg.Editor.AutoSaveDelay, func(time.Time) tea.Msg { return m }))
	}
	return tea.Batch(cmds...)
}

func (a *App) handleAutoSave(m autoSaveMsg) tea.Cmd {
	t := m.tab
	if !t.dirty() || t.buf.Version != m.ver || t.buf.Path == "" || !slices.Contains(a.tabs, t) {
		return nil
	}
	if t.buf.DiskChanged() {
		return nil // never overwrite another process's change silently
	}
	return a.saveTab(t)
}

func (a *App) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case autoSaveMsg:
		return a.handleAutoSave(msg)
	case watchMsg:
		reload := a.checkDiskChanges()
		a.treeTicks++
		if reload || a.treeTicks >= treeRefreshTicks {
			a.treeTicks = 0
			a.explorer.refresh()
		}
		if a.gd.on {
			a.gd.ticks++
			if reload || a.gd.ticks%gitDiffRefreshTicks == 0 {
				return tea.Batch(watchTick(), a.reloadChanges())
			}
		}
		return watchTick()
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		a.follow = true
	case termOutputMsg, termDoneMsg:
		cmd := a.term.update(msg)
		if len(a.term.tabs) == 0 && a.showTerm {
			a.showTerm = false
			if a.focus == focusTerminal {
				return tea.Batch(cmd, a.setFocus(focusEditor))
			}
		}
		return cmd
	case termErrMsg:
		a.fail(msg.err)
		if len(a.term.tabs) == 0 && a.showTerm {
			a.showTerm = false
			if a.focus == focusTerminal {
				return a.setFocus(focusEditor)
			}
		}
		return nil
	case lspEventMsg:
		a.handleLSPEvent(lsp.Event(msg))
		return a.lsp.wait()
	case mermaidStartMsg:
		return a.handleMermaidStart(msg)
	case mermaidDoneMsg:
		a.handleMermaidDone(msg)
	case hoverMsg:
		if t := a.curText(); a.fullPreview(t) {
			return nil // the popup would be anchored to the hidden buffer
		}
		if m := msg.mouse; m != nil {
			if m.seq != a.hoverSeq || m.tab != a.cur() || strings.TrimSpace(msg.text) == "" {
				return nil
			}
			a.hoverText, a.hoverScroll = msg.text, 0
			a.hoverAt, a.hoverWord = &m.pt, m.word
			return nil
		}
		if strings.TrimSpace(msg.text) == "" {
			a.info("ホバー情報はありません")
		} else {
			a.hoverText = msg.text
			a.hoverScroll = 0
			a.hoverAt = nil
		}
	case hoverDwellMsg:
		return a.hoverDwell(msg)
	case locationsMsg:
		return a.showLocations(msg)
	case completionMsg:
		t := a.curText()
		if t == nil || t != msg.tab || a.mode != modeInsert && a.helix || len(msg.items) == 0 {
			return nil
		}
		a.comp = &completionPopup{tab: t, start: msg.start, all: msg.items}
		a.comp.filter()
		if len(a.comp.items) == 0 {
			a.comp = nil
		}
	case workspaceEditMsg:
		a.applyWorkspaceEdit(msg.edit)
	case formatMsg:
		if len(msg.edits) > 0 {
			msg.tab.buf.ApplyEdits(toEdits(msg.tab.buf, msg.edits))
		}
		if msg.save {
			return a.saveTab(msg.tab)
		}
		a.info("整形しました")
	case lspErrMsg:
		a.fail(msg.err)
	case browserLoadedMsg:
		a.browserLoaded(msg)
	case liveStartedMsg:
		a.liveStarted(msg)
	case liveStateMsg:
		// nothing to do: the update itself redraws
	case openURLMsg:
		if msg.err != nil {
			a.fail(msg.err)
		} else {
			a.info("開きました: %s", msg.url)
		}
	case changesLoadedMsg:
		a.applyChanges(msg)
	case searchTickMsg:
		return a.searchTick(msg)
	case searchDoneMsg:
		a.searchDone(msg)
	case reviewLoadedMsg:
		for _, t := range a.tabs {
			if t.kind == tabReview && t.rev.id == msg.id {
				t.rev.applyLoaded(msg)
			}
		}
	case reviewSubmittedMsg:
		return a.handleSubmitted(msg)
	case prListMsg:
		return a.showPRList(msg)
	case tea.MouseMsg:
		return a.handleMouse(msg)
	case tea.KeyMsg:
		a.hoverSeq++ // a pending mouse hover is stale once the user types
		a.hoverWord = wordSpan{}
		// Fast typing can deliver several runes in one message; Helix-style
		// commands need them one at a time.
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !msg.Paste {
			var cmds []tea.Cmd
			for _, r := range msg.Runes {
				cmds = append(cmds, a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt}))
			}
			return tea.Batch(cmds...)
		}
		return a.handleKey(msg)
	}
	return nil
}

func (a *App) handleLSPEvent(e lsp.Event) {
	switch {
	case e.Err != nil:
		a.fail(e.Err)
	case e.Message != "":
		a.info("[%s] %s", e.Server, firstLine(e.Message))
	case e.URI != "":
		p := lsp.Path(e.URI)
		if a.lsp.diags[p] == nil {
			a.lsp.diags[p] = map[string][]lsp.Diagnostic{}
		}
		a.lsp.diags[p][e.Server] = e.Diagnostics
	}
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	a.msg = ""
	if a.hoverAt != nil {
		// A popup the pointer opened never takes keys: it closes whatever has the
		// focus, and the key goes on to it.
		a.hoverText, a.hoverAt = "", nil
	}
	if key != "ctrl+q" {
		a.confirmQuit = false
	}
	if key != "ctrl+w" && key != "q" && key != "esc" {
		a.confirmClose = false
	}

	if a.search.open {
		return a.handleSearchKey(msg)
	}
	if a.pal.open {
		return a.handlePaletteKey(msg)
	}
	if len(a.props) > 0 {
		return a.handleProposalKey(msg)
	}
	if a.ctx != nil {
		return a.handleContextKey(msg)
	}
	if a.cmdOpen {
		return a.handleCmdlineKey(msg)
	}
	if a.resizeMode {
		return a.handleResizeKey(key)
	}
	if a.hoverText != "" {
		if d := hoverScrollKey(key); d != 0 {
			a.scrollHover(d)
			return nil
		}
		if key == "y" {
			a.copyHover()
			return nil
		}
		a.hoverText = ""
		if key == "esc" {
			return nil
		}
	}
	if a.comp != nil {
		if cmd, handled := a.handleCompletionKey(msg); handled {
			return cmd
		}
	}

	if a.focus == focusTerminal {
		// The shell owns the keyboard; only these stay global.
		switch key {
		case "ctrl+q":
			return a.quit()
		case "ctrl+j":
			return a.toggleTerm()
		case "alt+r":
			return a.startResize()
		case "f1":
			return a.pal.show(a.root, ">")
		}
		return a.term.update(msg)
	}

	insert := a.helix && a.mode == modeInsert && a.focus == focusEditor
	switch key {
	case "ctrl+q":
		return a.quit()
	case "ctrl+s":
		return a.save()
	case "ctrl+p":
		return a.pal.show(a.root, "")
	case "f1":
		return a.pal.show(a.root, ">")
	case "ctrl+k":
		if !insert {
			return a.pal.show(a.root, ">")
		}
	case "ctrl+b":
		a.toggleSide()
		return nil
	case "ctrl+j", "ctrl+@":
		// Ctrl+Space is completion wherever the editor accepts typing: VS Code
		// mode, or Helix insert mode.
		typing := a.focus == focusEditor && a.curText() != nil && (!a.helix || a.mode == modeInsert)
		if key == "ctrl+j" || !typing {
			return a.toggleTerm()
		}
	case "ctrl+g":
		return a.toggleGitDiff()
	case "ctrl+e":
		if a.focus == focusExplorer && len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
		a.showSide = true
		return a.setFocus(focusExplorer)
	case "ctrl+n":
		if !insert {
			return a.newFile()
		}
	case "ctrl+w":
		if !insert {
			a.closeTab(false)
			return nil
		}
	case "ctrl+pgdown":
		a.switchTab(1)
		return nil
	case "ctrl+pgup":
		a.switchTab(-1)
		return nil
	case "alt+a":
		return a.sendContext()
	case "alt+r":
		return a.startResize()
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
	case focusChanges:
		return a.handleChangesKey(msg)
	}
	t := a.cur()
	if t == nil {
		if key == "esc" || key == "tab" || key == " " {
			a.showSide = true
			return a.setFocus(focusExplorer)
		}
		if key == ":" {
			return a.openCmdline(":")
		}
		return nil
	}
	switch t.kind {
	case tabReview:
		return a.handleReviewKey(t, msg)
	case tabBrowser:
		if l := t.web.live; l != nil && l.input && l.sess != nil && msg.Paste {
			// pasted text goes in at once (typed runes arrive here one by one)
			l.sess.Text(string(msg.Runes))
			return nil
		}
		return a.handleBrowserKey(t, key)
	case tabImage:
		switch key {
		case "q", "esc":
			a.closeTab(true)
		case ":":
			return a.openCmdline(":")
		case " ":
			a.pending = " "
		default:
			if a.pending == " " {
				a.pending = ""
				return a.spaceMode(key)
			}
		}
		return nil
	}
	if a.findOpen {
		return a.handleFindKey(msg)
	}
	if a.helix {
		return a.handleHelixKey(msg)
	}
	if t := a.curText(); a.fullPreview(t) && !fullPreviewEditorKey(msg) {
		a.info("全幅プレビュー中は編集できません")
		return nil
	}
	return a.handleEditorKey(msg)
}

func (a *App) handleCompletionKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	c := a.comp
	switch msg.String() {
	case "up", "ctrl+p", "shift+tab":
		c.sel = (c.sel - 1 + len(c.items)) % len(c.items)
		return nil, true
	case "down", "ctrl+n":
		c.sel = (c.sel + 1) % len(c.items)
		return nil, true
	case "tab", "enter":
		a.acceptCompletion()
		return nil, true
	case "esc":
		a.comp = nil
		return nil, !a.helix
	}
	return nil, false
}

func (a *App) handlePaletteKey(msg tea.KeyMsg) tea.Cmd {
	p := a.pal
	switch msg.String() {
	case "esc", "ctrl+c":
		p.close()
		return nil
	case "up", "ctrl+p", "shift+tab":
		if p.sel > 0 {
			p.sel--
		}
		return nil
	case "down", "ctrl+n", "tab":
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
	if p.picking {
		if p.sel < len(p.items) && p.onPick != nil {
			return p.onPick(p.items[p.sel].index)
		}
		return nil
	}
	if p.prompt != "" {
		if p.onSubmit != nil {
			return p.onSubmit(v)
		}
		return nil
	}
	switch {
	case strings.HasPrefix(v, ":"):
		return a.execCommand(strings.TrimSpace(v[1:]))
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
	key := msg.String()
	if a.pending == " " {
		a.pending = ""
		return a.spaceMode(key)
	}
	switch key {
	case "up", "k":
		e.sel = max(0, e.sel-1)
	case "down", "j":
		e.sel = min(len(e.flat)-1, e.sel+1)
	case "pgup", "ctrl+u":
		e.sel = max(0, e.sel-(a.h-3)/2)
	case "pgdown", "ctrl+d":
		e.sel = min(len(e.flat)-1, e.sel+(a.h-3)/2)
	case "home", "g":
		e.sel = 0
	case "end", "G":
		e.sel = len(e.flat) - 1
	case "enter", "l", "right", "o":
		n := e.selected()
		if n == nil {
			return nil
		}
		if n.isDir {
			if key == "enter" || key == "o" || !n.expanded {
				e.toggle(n)
			}
			return nil
		}
		if a.openFile(n.path) && (key == "enter" || key == "o") {
			return a.setFocus(focusEditor)
		}
	case "left", "h":
		e.collapseOrParent()
	case "a", "n":
		return a.newFileIn(e.targetDir())
	case "A", "N":
		return a.newFolder()
	case "r":
		return a.renameFile()
	case "d", "delete":
		return a.deleteFile()
	case "y", "Y":
		if n := e.selected(); n != nil {
			a.copyPath(n.path, key == "Y")
		}
	case "R":
		e.refresh()
		a.info("ファイルツリーを更新しました")
	case " ":
		a.pending = " "
	case ":":
		return a.openCmdline(":")
	case "tab":
		if a.gd.on {
			return a.setFocus(focusChanges)
		}
		if len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
	case "esc", "q":
		if len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
	}
	return nil
}

func (a *App) handleFindKey(msg tea.KeyMsg) tea.Cmd {
	t := a.curText()
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

// handleEditorKey is the modeless (VS Code style) editing used by the
// vscode keymap and by Helix insert mode.
func (a *App) handleEditorKey(msg tea.KeyMsg) tea.Cmd {
	t := a.curText()
	if t != nil && t.buf.HasExtra() && multiEditable(msg) {
		a.comp = nil
		t.buf.ForEachCursor(func() { a.editorKey(t, msg) })
		a.follow = true
		return nil
	}
	return a.editorKey(t, msg)
}

// multiEditable reports whether msg is a movement or editing key that
// applies to every cursor.
func multiEditable(msg tea.KeyMsg) bool {
	if msg.Paste || msg.Type == tea.KeyRunes && !msg.Alt {
		return true
	}
	switch msg.String() {
	case "left", "right", "up", "down", "shift+left", "shift+right", "shift+up", "shift+down",
		"home", "end", "shift+home", "shift+end", "ctrl+left", "ctrl+right", "alt+b", "alt+f",
		"ctrl+shift+left", "ctrl+shift+right", "enter", "tab", "backspace", "ctrl+h", "delete", " ":
		return true
	}
	return false
}

func (a *App) editorKey(t *tab, msg tea.KeyMsg) tea.Cmd {
	if t == nil {
		return nil
	}
	b := t.buf
	a.follow = true
	page := a.pageSize()

	if msg.Paste {
		s := strings.ReplaceAll(string(msg.Runes), "\r\n", "\n")
		b.InsertBlock(strings.ReplaceAll(s, "\r", "\n"))
		return nil
	}

	switch msg.String() {
	case "esc":
		b.ClearSelection()
		b.ClearExtra()
		a.comp = nil
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
	case "pgdown":
		b.MoveLines(page, false)
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
		return a.refreshCompletion()
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
	case "ctrl+@", "ctrl+ ":
		return a.completion()
	case "f12":
		return a.gotoDefinition()
	case "shift+f12":
		return a.references()
	case "f2":
		return a.rename()
	case " ":
		b.Insert(" ")
		a.comp = nil
	default:
		if msg.Type == tea.KeyRunes && !msg.Alt {
			s := string(msg.Runes)
			b.Insert(s)
			return a.afterTyping(t, msg.Runes)
		}
	}
	return nil
}

// afterTyping opens or refreshes the completion popup.
func (a *App) afterTyping(t *tab, typed []rune) tea.Cmd {
	if len(typed) != 1 {
		return nil
	}
	r := typed[0]
	if a.comp != nil {
		if isIdent(r) {
			a.comp.filter()
			if len(a.comp.items) == 0 {
				a.comp = nil
			}
			return nil
		}
		a.comp = nil
	}
	// Typing must not report a missing or starting server; only an explicit
	// request does.
	if a.lsp.client(t, "completionProvider") == nil {
		return nil
	}
	cs, _ := a.lsp.clientsFor(t)
	for _, c := range cs {
		if c.IsCompletionTrigger(r) {
			return a.completion()
		}
	}
	if isIdent(r) {
		ws := wordStart(t.buf)
		if t.buf.Cur.Col-ws.Col == 2 {
			return a.completion()
		}
	}
	return nil
}

func (a *App) refreshCompletion() tea.Cmd {
	if a.comp == nil {
		return nil
	}
	b := a.comp.tab.buf
	if b.Cur.Less(a.comp.start) || b.Cur.Row != a.comp.start.Row {
		a.comp = nil
		return nil
	}
	a.comp.filter()
	if len(a.comp.items) == 0 {
		a.comp = nil
	}
	return nil
}

// ---- mouse ----

// termMouse passes presses, drags and releases over the terminal body to a
// program that asked for mouse input (vim, htop, tmux). Once a press went to
// the program, its drag and release do too, wherever the pointer is. ok is
// false when the event is not for the program.
func (a *App) termMouse(l layout, msg tea.MouseMsg) (cmd tea.Cmd, ok bool) {
	t := a.term
	if !a.showTerm {
		return nil, false
	}
	x, y := msg.X-l.rightX, msg.Y-l.panelY-2
	inBody := x >= 0 && x < l.rightW && y >= 0 && msg.Y < l.statusY
	grabbed := t.grab
	if !grabbed && !inBody {
		return nil, false
	}
	sent := t.mouse(msg, x, y)
	switch msg.Action {
	case tea.MouseActionPress:
		if sent {
			t.grab = true
			return a.setFocus(focusTerminal), true
		}
	case tea.MouseActionRelease:
		t.grab = false
	}
	return nil, sent || grabbed
}

func (a *App) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if a.search.open {
		return a.searchMouse(msg)
	}
	if a.pal.open {
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			a.pal.close()
		}
		return nil
	}
	l := a.layout()
	if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonNone {
		return a.passiveMotion(l, msg)
	}
	x, y := msg.X, msg.Y
	wheel := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		wheel = -3
	case tea.MouseButtonWheelDown:
		wheel = 3
	}
	// A popup the pointer opened only owns the wheel while the pointer is on it.
	if wheel != 0 && a.hoverText != "" && (a.hoverAt == nil || a.inHoverBox(l, screenPt{x, y})) {
		if msg.Action == tea.MouseActionPress {
			a.scrollHover(wheel)
		}
		return nil
	}
	if a.hoverText != "" && msg.Button == tea.MouseButtonLeft {
		// The button copies on press; the matching release must not close the hover.
		if bx, by, ok := a.hoverPos(l); ok && a.hoverView(l).copyHit(x-bx, y-by) {
			if msg.Action == tea.MouseActionPress {
				a.copyHover()
			}
			return nil
		}
	}
	// Past the branches that keep the popup open: the pointer's hover is over.
	a.hoverSeq++
	a.hoverWord = wordSpan{}
	a.hoverCell = screenPt{-1, -1}
	a.hoverText, a.hoverAt = "", nil
	if wheel != 0 && msg.Action == tea.MouseActionPress {
		switch {
		case a.showSide && x < l.sideW:
			e := a.explorer
			if a.gd.on && y >= a.explorerH() {
				g := &a.gd
				body := a.h - 2 - a.explorerH()
				g.scroll = clamp(g.scroll+wheel, 0, max(0, len(g.files)-body))
				g.sel = clamp(g.sel, g.scroll, g.scroll+body-1)
			} else {
				eh := a.explorerH()
				e.scroll = clamp(e.scroll+wheel, 0, max(0, len(e.flat)-(eh-1)))
				e.sel = clamp(e.sel, e.scroll, e.scroll+eh-2)
			}
		case a.showTerm && y >= l.panelY && y < l.statusY:
			a.term.wheel(msg, x-l.rightX, y-l.panelY-2, wheel)
		default:
			if t := a.cur(); t != nil {
				switch t.kind {
				case tabText:
					if a.fullPreview(t) {
						t.buf.MoveLines(wheel, false) // the preview scrolls with the cursor
						break
					}
					t.buf.ScrollRow = clamp(t.buf.ScrollRow+wheel, 0, max(0, len(t.buf.Lines)-1))
				case tabReview:
					t.rev.moveCursor(wheel)
				case tabBrowser:
					a.browserWheel(t.web, x, y, wheel)
				}
			}
		}
		return nil
	}

	if a.resizing != resizeNone {
		switch msg.Action {
		case tea.MouseActionMotion:
			a.dragResize(l, x, y)
			return nil
		case tea.MouseActionRelease:
			a.resizing = resizeNone
			return nil
		}
		// A press means the release was lost (pointer left the window).
		a.resizing = resizeNone
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		if r := a.resizeHit(l, x, y); r != resizeNone {
			a.resizing = r
			return nil
		}
	}

	if cmd, ok := a.termMouse(l, msg); ok {
		return cmd
	}

	if v := a.browserTab(); v != nil && v.live != nil {
		if cmd, ok := a.liveMouse(v, msg); ok {
			return cmd
		}
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
		if a.gd.on && y >= a.explorerH() {
			if i := y - 1 - a.explorerH() + a.gd.scroll; y > a.explorerH() && i < len(a.gd.files) {
				a.gd.sel = i
				return a.openChange(a.gd.files[i])
			}
			return a.setFocus(focusChanges)
		}
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
				a.mode = modeNormal
				return a.setFocus(focusEditor)
			}
		}
	case a.showTerm && y >= l.panelY && y < l.statusY:
		if y == l.panelY+1 {
			a.term.clickTab(x - l.rightX)
		}
		return a.setFocus(focusTerminal)
	case y >= l.editY && y < l.editY+l.editH:
		t := a.cur()
		if t == nil {
			return nil
		}
		if t.kind == tabReview {
			a.reviewClick(t.rev, l, x, y)
			return a.setFocus(focusEditor)
		}
		if t.kind == tabText {
			if a.diffFile(t) != nil {
				if y == l.editY {
					a.toggleSplit()
					return nil
				}
				if x < a.dvGeom(l).rightX {
					return nil // the base side is read-only
				}
			}
			if t.preview != previewOff && x >= l.rightX+a.editW(t, l) {
				return nil // the preview pane is read-only
			}
			a.comp = nil
			a.mouseToCursor(l, x, y, false)
			a.dragging = true
			return a.setFocus(focusEditor)
		}
	}
	return nil
}

func (a *App) mouseToCursor(l layout, x, y int, extend bool) {
	t := a.curText()
	if t == nil {
		return
	}
	if dv := a.diffFor(t); dv != nil {
		a.diffMouseToCursor(t, dv, l, x, y, extend)
		return
	}
	x = min(x, l.rightX+a.editW(t, l)-1)
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

// editW is the width of the text editor, narrower while the Markdown preview
// is open and zero while the preview fills the area.
func (a *App) editW(t *tab, l layout) int {
	if a.diffFile(t) != nil {
		return a.dvGeom(l).rightW
	}
	switch t.preview {
	case previewSplit:
		return a.editorWidth(l.rightW)
	case previewFull:
		return 0
	}
	return l.rightW
}

func (a *App) renderText(t *tab, l layout, focused bool) []string {
	if dv := a.diffFor(t); dv != nil {
		return a.renderDiffText(t, dv, l, focused)
	}
	if t.preview == previewFull {
		return a.previewLines(t, l.rightW, l.editH)
	}
	ew := a.editW(t, l)
	edit := renderEditor(t, ew, l.editH, a.editorOpts(t, focused))
	if t.preview == previewOff {
		return edit
	}
	pw := l.rightW - ew - 1
	prev := a.previewLines(t, pw, l.editH)
	sep := stSeparator.Render("│")
	for i := range edit {
		edit[i] += sep + prev[i]
	}
	return edit
}

func (a *App) View() string {
	if a.w <= 0 || a.h <= 0 {
		return ""
	}
	if a.w < 30 || a.h < 8 {
		return "ウィンドウが小さすぎます"
	}
	l := a.layout()
	if t := a.curText(); t != nil {
		if a.follow {
			if dv := a.diffFor(t); dv != nil {
				a.ensureDiffVisible(t, dv, a.editW(t, l), a.dvGeom(l).h)
			} else {
				ensureCursorVisible(t.buf, a.editW(t, l), l.editH)
			}
			a.follow = false
		}
		t.buf.ScrollRow = clamp(t.buf.ScrollRow, 0, max(0, len(t.buf.Lines)-1))
	}

	right := make([]string, 0, a.h-1)
	right = append(right, a.renderTabs(l.rightW))
	if a.findOpen {
		right = append(right, a.renderFind(l.rightW))
	}
	editorFocused := a.focus == focusEditor && !a.overlayOpen()
	t := a.cur()
	switch {
	case t == nil:
		right = append(right, renderWelcome(l.rightW, l.editH, a.helix)...)
	case t.kind == tabImage:
		right = append(right, t.img.render(l.rightW, l.editH)...)
	case t.kind == tabBrowser:
		right = append(right, t.web.render(l.rightW, l.editH)...)
	case t.kind == tabReview:
		right = append(right, t.rev.render(l.rightW, l.editH, editorFocused)...)
	default:
		right = append(right, a.renderText(t, l, editorFocused)...)
	}
	if a.showTerm {
		right = append(right, a.term.render(l.rightW, l.panelH, a.focus == focusTerminal && !a.overlayOpen())...)
	}

	rows := make([]string, a.h-1)
	var side []string
	activePath := ""
	if t != nil {
		activePath = t.path()
	}
	if a.showSide {
		side = a.explorer.render(l.sideW, a.explorerH(), a.focus == focusExplorer && !a.overlayOpen(), activePath)
		if a.gd.on {
			side = append(side, a.renderChanges(l.sideW, a.h-1-a.explorerH(), a.focus == focusChanges && !a.overlayOpen())...)
		}
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

	// popups anchored at the cursor
	if t := a.curText(); t != nil && !a.fullPreview(t) && (a.comp != nil || a.hoverText != "") {
		cx, cy := a.popupAnchor(t, l)
		var box []string
		if a.comp != nil {
			box = a.comp.render(max(3, l.editH/2))
		} else {
			box = a.hoverView(l).render(a.hoverScroll)
		}
		x, y := popupPos(l, a.w, cx, cy, box)
		rows = overlay(rows, box, x, y)
	}

	if a.pal.open {
		a.pal.filter(a.commands)
		pw := clamp(a.w*3/5, 30, 100)
		box := a.pal.render(pw)
		rows = overlay(rows, box, (a.w-pw)/2, 1)
	}

	if a.search.open {
		x, y, box := a.renderSearch()
		rows = overlay(rows, box, x, y)
	}

	if a.ctx != nil {
		rows = a.renderContext(rows)
	}
	if len(a.props) > 0 {
		rows = a.renderProposal(rows)
	}

	if a.cmdOpen {
		rows = append(rows, a.renderCmdline())
	} else {
		rows = append(rows, a.renderStatus())
	}
	return strings.Join(rows, "\n")
}

func (a *App) editorOpts(t *tab, focused bool) editorOpts {
	o := editorOpts{
		focused:     focused && !a.findOpen && !a.cmdOpen,
		cursorColor: curTheme.Cursor,
		cursorAt:    t.buf.Cur,
		relative:    a.cfg.Editor.LineNumber == "relative",
		diags:       a.lsp.diagnostics(t.buf.Path),
	}
	if a.findOpen {
		o.query = a.findInput.Value()
	}
	if a.cmdOpen && (a.cmdKind == "/" || a.cmdKind == "?") {
		o.query = a.cmdInput.Value()
	}
	if a.helix {
		switch a.mode {
		case modeInsert:
			o.cursorColor = curTheme.ModeInsert
		case modeSelect:
			o.cursorColor = curTheme.ModeSelect
		}
		// Helix draws the cursor on the last selected character
		o.cursorAt = a.cursorCell(t.buf, t.buf.Anchor, t.buf.Cur)
	}
	for _, r := range t.buf.Extra() {
		var anchor *buffer.Pos
		if r.HasAnchor {
			anchor = &r.Anchor
		}
		c := r.Cur
		if a.helix {
			c = a.cursorCell(t.buf, anchor, r.Cur)
		}
		o.extraCursors = append(o.extraCursors, c)
		if r.HasAnchor && r.Anchor != r.Cur {
			o.extraSels = append(o.extraSels, [2]buffer.Pos{r.Start(), r.End()})
		}
	}
	return o
}

func (a *App) renderTabs(w int) string {
	type piece struct {
		text string
		w    int
	}
	pieces := make([]piece, len(a.tabs))
	for i, t := range a.tabs {
		mark := "  "
		if t.dirty() {
			mark = " ●"
		}
		name := t.title()
		label := " " + name + mark + " "
		var s string
		switch {
		case i == a.active && t.dirty():
			s = stTabActive.Render(" "+name) + stTabDirty.Render(mark) + stTabActive.Render(" ")
		case i == a.active:
			s = stTabActive.Render(label)
		default:
			s = stTabInactive.Render(label)
		}
		pieces[i] = piece{s + stTabBar.Render(" "), ansi.StringWidth(label) + 1}
	}
	// active is -1 once the last tab is closed
	a.tabOffset = clamp(min(a.tabOffset, a.active), 0, max(0, len(a.tabs)-1))
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
	rx := a.layout().rightX
	for i := a.tabOffset; i < len(pieces); i++ {
		if x+pieces[i].w > w {
			break
		}
		sb.WriteString(pieces[i].text)
		a.tabSpans[i] = span{x0: x + rx, x1: x + rx + pieces[i].w}
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

var modeStyles map[editMode]lipgloss.Style

func (a *App) renderStatus() string {
	var prefix string
	prefixW := 0
	if a.helix && a.focus == focusEditor && a.curText() != nil {
		label := " " + a.mode.label() + " "
		if a.pending != "" {
			p := a.pending
			if p == " " {
				p = "space"
			}
			label = " " + a.mode.label() + " " + p + " "
		}
		prefix = modeStyles[a.mode].Render(label)
		prefixW = ansi.StringWidth(label)
	}
	leftParts := []string{}
	if a.branch != "" {
		leftParts = append(leftParts, "⎇ "+a.branch)
	}
	switch a.focus {
	case focusExplorer:
		leftParts = append(leftParts, "ファイルツリー")
	case focusChanges:
		leftParts = append(leftParts, "変更一覧")
	case focusTerminal:
		leftParts = append(leftParts, "ターミナル")
	}
	if t := a.curText(); t != nil {
		ds := a.lsp.diagnostics(t.buf.Path)
		errs, warns := 0, 0
		for _, d := range ds {
			if sevRank(d.Severity) == 1 {
				errs++
			} else if d.Severity == 2 {
				warns++
			}
		}
		if errs+warns > 0 {
			leftParts = append(leftParts, fmt.Sprintf("● %d  ▲ %d", errs, warns))
		}
	}
	left := " " + strings.Join(leftParts, "  ")
	right := ""
	if t := a.cur(); t != nil && t.kind == tabText {
		b := t.buf
		indent := fmt.Sprintf("スペース: %d", len(b.IndentUnit()))
		if b.IndentUnit() == "\t" {
			indent = "タブ"
		}
		pos := fmt.Sprintf("%d:%d", b.Cur.Row+1, buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)+1)
		if b.HasExtra() {
			pos = fmt.Sprintf("%d カーソル ", len(b.Extra())+1) + pos
		}
		if s := b.SelectedText(); s != "" {
			pos += fmt.Sprintf(" (%d 文字選択)", len([]rune(s)))
		}
		lang := t.hl.Language()
		if st := a.lspStatus(); st != "" {
			lang += " [" + st + "]"
		}
		right = fmt.Sprintf("%s   %s   %s ", pos, indent, lang)
	}
	st := stStatus
	if a.msg != "" {
		if a.msgErr {
			st = stStatusError
		}
		left += "   " + a.msg
	}
	lw := a.w - prefixW - ansi.StringWidth(right)
	return prefix + st.Render(fit(left, lw)+right)
}

// popupPos places box below the cursor cell (cx, cy), or above it when there
// is no room below, clamped to the editor area of a screen w cells wide.
func popupPos(l layout, w, cx, cy int, box []string) (x, y int) {
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	x = clamp(cx, l.rightX, max(l.rightX, w-bw))
	y = cy + 1
	if y+len(box) > l.editY+l.editH && cy-len(box) >= l.editY {
		y = cy - len(box)
	}
	y = max(l.editY, min(y, l.editY+l.editH-len(box)))
	return x, y
}

// hoverPos is the screen position of the hover popup's top-left corner.
func (a *App) hoverPos(l layout) (x, y int, ok bool) {
	t := a.curText()
	if t == nil || a.fullPreview(t) {
		return 0, 0, false
	}
	cx, cy := a.popupAnchor(t, l)
	x, y = popupPos(l, a.w, cx, cy, a.hoverView(l).render(a.hoverScroll))
	return x, y, true
}

// copyHover copies the hover markdown source to the clipboard.
func (a *App) copyHover() {
	a.copyText(a.hoverText)
	a.info("ホバーをコピーしました")
}

// hoverView lays out the current hover text for layout l. The box height is
// capped to the larger free side of the cursor so it never overflows.
func (a *App) hoverView(l layout) hoverView {
	h := l.editH * 2 / 3
	if t := a.curText(); t != nil {
		_, cy := a.popupAnchor(t, l)
		below := l.editY + l.editH - (cy + 1)
		above := cy - l.editY
		h = min(h, max(below, above))
	}
	return newHoverView(a.hoverText, min(80, l.rightW-2), max(h, 3))
}

// hoverScrollKey maps a key to a hover scroll amount (0 = not a scroll key).
func hoverScrollKey(key string) int {
	switch key {
	case "ctrl+d", "pgdown":
		return 5
	case "ctrl+u", "pgup":
		return -5
	}
	return 0
}

func (a *App) scrollHover(d int) {
	v := a.hoverView(a.layout())
	a.hoverScroll = clamp(a.hoverScroll+d, 0, v.maxScroll())
}

// overlayOpen reports whether a floating window (palette or project search)
// has the keyboard.
func (a *App) overlayOpen() bool { return a.pal.open || a.search.open }
