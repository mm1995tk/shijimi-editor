// Package tui implements the shijimi terminal IDE.
package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/caret"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/proxy"
)

type focus int

const (
	focusEditor focus = iota
	focusExplorer
	focusTerminal
	focusChanges // the CHANGES list in diff mode
	focusAgent   // the suggestion panel on the right
)

type span struct{ x0, x1 int }

// Core is the state that decides what is shown: the files, the cursor, the
// keys' state, the language servers. It is what moves to internal/core (#384);
// App keeps what only a terminal has (the size of the screen, which regions
// are shown, the terminal, the pane sizes the user picked).
type Core struct {
	root      string
	gitTop    string
	cfg       *config.Config
	keymap    helixKeymap
	lastIns   *insertRecord // what "." repeats
	recIns    *insertRecord // the insert being recorded, until Esc
	replaying bool          // "." is running; do not record
	host      Host          // the GUI of `shj serve`; nil in the TUI
	lsp       *lspManager
	focus     focus
	explorer  *explorer
	treeTicks int // watch ticks since the tree was last reloaded
	proxy     *proxy.Manager
	pal       *palette
	wsSyms    []lsp.Symbol // rows of the project symbol picker
	wsClient  *lsp.Client  // server the project symbol picker asks
	search    *searchPanel
	tabs      []*tab
	jumps     []jumpLoc // positions to return to (Ctrl+O / Ctrl+I)
	jumpIdx   int       // where in jumps the user is; len(jumps) = at the newest
	active    int
	branch    string
	gd        gitDiff
	confirm   *yesNo    // a y/n question on the status line
	card      *prCard   // a review thread shown like a hover
	composer  *composer // the editor for a review comment

	// pane sizes picked by the user (0 = automatic), see resize.go

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
	cmdInput inputField
	cmdFrom  buffer.Pos

	// VS Code-style find bar
	findOpen   bool
	findInput  inputField
	findOrigin buffer.Pos

	comp        *completionPopup
	sig         *signaturePopup
	sigSeq      int // bumped on every request and close; stale replies are dropped
	snip        *snippetSession
	hoverText   string
	hoverScroll int
	hoverAt     *buffer.Pos // the character a mouse-triggered popup hangs from (a place in the file, not on the screen); nil = at the cursor
	hoverWord   wordSpan    // word the pointer's pending or open hover belongs to
	hoverCell   screenPt    // last cell the pointer rested on without a button
	hoverSeq    int         // bumped on every input; stale dwell timers and replies are dropped
	ctx         *ctxPreview

	// Claude Code IDE integration (see ide.go)
	ide       *ide.Server
	props     core.Proposals // agent changes waiting for the user's decision
	propShown *proposal      // the first of them as drawn (propView)
	selTrack  core.SelectionTracker

	// suggestion panel and ACP agent (see suggest.go, agent.go)
	ag *agentState
	sg suggestState

	clipboard    string
	msg          string
	msgErr       bool
	confirmQuit  bool
	confirmClose bool
	follow       bool // scroll to the cursor on next render
	dragging     bool
	lastClick    click // the previous left press (adapter state: see clicks in inputtea.go)
	hold         holdScroll
	holdSeq      int  // tells the current hold-scroll tick chain from stale ones
	lineFlip     bool // line numbers shown the other way (\ held, or toggled with \)

	single  bool // 単体編集モード: 1 つのファイルだけを開き、閉じたら終了する
	prevTab *tab // the file shown before the current one (ga / Ctrl+Tab)
	// side by side editor panes (see window.go): the focused pane shows cur(), the
	// other one shows other, and pane is the side (0 = left) the focused one is on
	split     bool
	pane      int
	other     *tab
	otherPrev *tab // prevTab of the other pane
	seenTab   *tab // the file shown when the last event was handled
	commands  []command
	mermaid   map[string]*previewJob
	mdImages  map[string]*previewJob // images in the Markdown preview, by file path
	// previewGen counts finished jobs, so the preview is drawn again when one
	// finishes
	previewGen int
}

// App is the root Bubble Tea model: a Core and what draws it to a terminal.
type App struct {
	*Core

	caret       *caret.Writer     // the terminal output; nil in tests
	cellSize    func() (w, h int) // terminal cell size in pixels; nil in tests
	w, h        int
	term        *terminal
	mux         muxPane // the multiplexer pane that replaces term inside a multiplexer
	showSide    bool
	showTerm    bool
	sideWPref   int
	panelHPref  int
	previewPct  int
	diffLeftPct int // base pane share of a split diff; 0 = half
	explorerPct int // file tree share of the diff mode sidebar; 0 = 7/10
	resizeMode  bool
	resizing    resizeTarget // border being dragged with the mouse
	showAgent   bool
	agentWPref  int    // panel width picked by the user (0 = automatic)
	capture     Region // the region a held pointer button is captured by (see locate)
	keyReleases bool   // the terminal reports key releases
	askReleases bool   // key releases were asked for in the last View
	quitting    bool   // tea.Quit was returned (see exit)
}

// New creates the app rooted at dir, optionally opening file.
func New(dir, file string) *App {
	abs, _ := filepath.Abs(dir)
	cfg := config.Load(abs)
	i18n.SetLang(cfg.Editor.UILanguage)
	a := &App{
		Core: &Core{
			root:     abs,
			cfg:      cfg,
			lsp:      newLSPManager(cfg, abs),
			explorer: newExplorer(abs),
			proxy:    proxy.NewManager(),
			pal:      newPalette(),
			search:   newSearchPanel(cfg.Search),
			focus:    focusExplorer,
			helix:    cfg.Editor.Keymap != "vscode",
			gd:       gitDiff{split: true},
			ag:       newAgentState(),
		},
		term:     newTerminal(abs),
		showSide: true,
	}
	a.explorer.icons = cfg.Editor.FileIcons
	if cfg.Editor.Theme != "" {
		applyTheme(themeOrDefault(cfg.Editor.Theme))
	}
	a.findInput = newInputField(0)
	a.findInput.Placeholder = i18n.T("app.find")
	a.cmdInput = newInputField(0)
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
	a.loadKeymap()
	return a
}

// NewSingle creates the app in single-file mode: only file is edited, with no
// file tree, language server, terminal, diff mode or agent panel, and the app
// ends when the file is closed. A file that does not exist opens as an empty
// buffer, which creates the file when saved. It is meant for $EDITOR.
func NewSingle(file string) (*App, error) {
	abs, _ := filepath.Abs(file)
	dir := filepath.Dir(abs)
	cfg := config.Load(dir)
	i18n.SetLang(cfg.Editor.UILanguage)
	a := &App{
		Core: &Core{
			root:     dir,
			cfg:      cfg,
			lsp:      newLSPManager(cfg, dir),
			explorer: &explorer{root: &node{name: filepath.Base(dir), path: dir, isDir: true, expanded: true, depth: -1}}, // not loaded: no tree is shown
			proxy:    proxy.NewManager(),
			pal:      newPalette(),
			search:   newSearchPanel(cfg.Search),
			focus:    focusEditor,
			helix:    cfg.Editor.Keymap != "vscode",
			gd:       gitDiff{split: true},
			ag:       newAgentState(),
			single:   true,
		},
		term: newTerminal(dir),
	}
	if cfg.Editor.Theme != "" {
		applyTheme(themeOrDefault(cfg.Editor.Theme))
	}
	a.findInput = newInputField(0)
	a.findInput.Placeholder = i18n.T("app.find")
	a.cmdInput = newInputField(0)
	// The command palette is not reachable in this mode; with no commands, it
	// cannot open the views single-file mode leaves out even if it were.
	if !a.openFile(abs) {
		msg := i18n.T("app.cantopen")
		if a.msg != "" {
			msg = a.msg
		}
		return nil, errors.New(msg)
	}
	a.loadKeymap()
	return a, nil
}

// typing reports whether the editor accepts typed text right now: VS Code
// mode, or Helix insert mode.
func (a *App) typing() bool {
	return a.focus == focusEditor && a.curText() != nil && (!a.helix || a.mode == modeInsert)
}

// singleBlocked reports whether key opens something single-file mode does not
// have: the file palette, tree, terminal, diff mode, agent panel, or tabs.
func singleBlocked(key string, typing bool) bool {
	switch key {
	case "ctrl+p", "f1", "ctrl+b", "ctrl+`", "ctrl+j", "ctrl+g", "ctrl+e", "ctrl+n",
		"ctrl+tab", "ctrl+\\", "ctrl+1", "ctrl+2", "alt+a", "alt+i", "alt+r":
		return true
	case "ctrl+k":
		return true
	case "ctrl+space":
		return !typing
	}
	return false
}

// singleBlockedCommand reports whether the ":" command is unavailable in
// single-file mode.
func singleBlockedCommand(name string) bool {
	switch name {
	case "o", "open", "e", "edit", "n", "new", "vsplit", "vs", "split", "sp", "only", "on", "sh", "run", "!", "gitdiff", "gd", "review",
		"pr", "grep", "rg", "replace", "rep", "browser", "agent", "tree", "explorer", "lsp-restart":
		return true
	}
	return false
}

// Close stops language servers; call after the program exits.
func (a *App) Close() {
	a.lsp.stopAll()
	if a.ag.client != nil {
		a.stopAgent()
	}
	a.muxClose()
	core.RemoveShellIntegration()
	removeMuxShellIntegration()
}

func (a *App) Init() tea.Cmd {
	if a.single {
		return a.effectCmd(watchTimer())
	}
	return a.effectCmd(append([]Effect{WaitLSP{a.lsp.events}, watchTimer()}, a.probeRepo()...)...)
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

func (a *App) setFocus(f focus) []Effect {
	if a.single {
		f = focusEditor
	}
	a.focus = f
	a.termBlur()
	if f == focusTerminal {
		return a.termFocus()
	}
	return nil
}

func (a *App) rel(p string) string { return core.RelTo(a.root, p) }

func (a *App) addTab(t *tab) {
	a.tabs = append(a.tabs, t)
	a.active = len(a.tabs) - 1
	a.follow = true
	a.mode = modeNormal
}

// reveal selects path in the file tree, which single-file mode does not have.
func (a *App) reveal(path string) {
	if !a.single {
		a.explorer.reveal(path)
	}
}

func (a *App) openFile(path string) bool {
	abs, _ := filepath.Abs(path)
	for i, t := range a.tabs {
		if t.path() == abs {
			a.active = i
			t.transient = false
			a.reveal(abs)
			return true
		}
	}
	if st, err := os.Stat(abs); err == nil {
		if st.IsDir() {
			a.reveal(abs)
			return false
		}
		if isImage(abs) {
			a.addTab(&tab{kind: tabImage, img: newImageView(abs)})
			a.reveal(abs)
			return true
		}
		if st.Size() > 10<<20 {
			a.fail(fmt.Errorf(i18n.T("app.toolarge"), a.rel(abs)))
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
	a.reveal(abs)
	return true
}

// openTransient opens path as the transient tab, which takes the place of the
// previous transient tab. A file already open is just shown as it is.
func (a *App) openTransient(path string) bool {
	abs, _ := filepath.Abs(path)
	for i, t := range a.tabs {
		if t.path() == abs {
			a.active = i
			a.reveal(abs)
			return true
		}
	}
	old := slices.IndexFunc(a.tabs, func(t *tab) bool { return t.transient })
	n := len(a.tabs)
	if !a.openFile(abs) || len(a.tabs) == n {
		return false
	}
	t := a.tabs[a.active]
	t.transient = true
	if t.kind == tabText {
		t.transientVer = t.buf.Version
	}
	if old >= 0 {
		a.dropTab(old)
		a.tabs = slices.Insert(a.tabs[:len(a.tabs)-1], old, t)
		a.active = old
	}
	return true
}

// keepTransients turns transient tabs that have been edited into normal tabs.
func (a *App) keepTransients() {
	for _, t := range a.tabs {
		if t.transient && t.kind == tabText && t.buf.Version != t.transientVer {
			t.transient = false
		}
	}
}

func (a *App) save() []Effect {
	t := a.curText()
	if t == nil {
		return nil
	}
	if t.buf.Path == "" {
		return a.saveAs()
	}
	return a.saveTab(t)
}

func (a *App) saveTab(t *tab) []Effect {
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
	a.info(i18n.T("app.saved"), a.rel(t.buf.Path))
	return a.afterSave()
}

func (a *App) saveAll() []Effect {
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
	a.info(i18n.T("app.savedmany"), n)
	return nil
}

func (a *App) saveAs() []Effect {
	t := a.curText()
	if t == nil {
		return nil
	}
	initial := a.explorer.targetDir() + string(filepath.Separator)
	if t.buf.Path != "" {
		initial = t.buf.Path
	}
	return a.pal.ask(i18n.T("app.saveas"), initial, func(v string) []Effect {
		return a.saveTo(t, v)
	})
}

func (a *App) saveTo(t *tab, v string) []Effect {
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

func (a *App) newFile() []Effect {
	if a.single {
		return nil
	}
	a.addTab(newTab(buffer.New()))
	return a.setFocus(focusEditor)
}

func (a *App) newFileIn(dir string) []Effect {
	return a.pal.ask(i18n.T("app.newfile"), dir+string(filepath.Separator), func(v string) []Effect {
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

func (a *App) newFolder() []Effect {
	return a.pal.ask(i18n.T("app.newfolder"), a.explorer.targetDir()+string(filepath.Separator), func(v string) []Effect {
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

func (a *App) renameFile() []Effect {
	n := a.explorer.selected()
	if n == nil {
		return nil
	}
	old := n.path
	return a.pal.ask(i18n.T("app.newname"), old, func(v string) []Effect {
		v = strings.TrimSpace(v)
		if v == "" || v == old {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(filepath.Dir(old), v)
		}
		if _, err := os.Stat(v); err == nil {
			a.fail(fmt.Errorf(i18n.T("app.exists"), a.rel(v)))
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
		a.info(i18n.T("app.renamed"), a.rel(v))
		return nil
	})
}

func (a *App) deleteFile() []Effect {
	n := a.explorer.selected()
	if n == nil {
		return nil
	}
	target := n.path
	return a.pal.ask(fmt.Sprintf(i18n.T("app.confirmdelete"), a.rel(target)), "", func(v string) []Effect {
		if strings.TrimSpace(strings.ToLower(v)) != "y" {
			return nil
		}
		if err := os.RemoveAll(target); err != nil {
			a.fail(err)
			return nil
		}
		a.explorer.refresh()
		a.info(i18n.T("app.deleted"), a.rel(target))
		return nil
	})
}

func (a *App) closeTab(force bool) []Effect {
	t := a.cur()
	if t == nil {
		return nil
	}
	if !force && t.dirty() && !a.confirmClose {
		a.confirmClose = true
		a.msg, a.msgErr = i18n.T("app.closeunsaved"), true
		return nil
	}
	a.confirmClose = false
	keep := a.other
	a.dropTab(a.active)
	switch {
	case a.split && keep != nil:
		// the pane of the closed file goes, and the other file fills the area
		a.split, a.pane, a.other, a.otherPrev = false, 0, nil, nil
		a.active = a.tabIndex(keep)
	case a.active >= len(a.tabs):
		a.active = len(a.tabs) - 1
	}
	a.comp = nil
	a.mode = modeNormal
	if a.single {
		return a.quit()
	}
	if len(a.tabs) == 0 && a.focus == focusEditor {
		a.focus = focusExplorer
	}
	return nil
}

// dropTab removes tab i and releases what it holds; the caller fixes active.
func (a *App) dropTab(i int) {
	t := a.tabs[i]
	if t.kind == tabText && t.lang != nil {
		a.detachLSP(t)
	}
	if t.img != nil {
		t.img.close()
	}
	a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
}

func (a *App) switchLast() {
	i := slices.Index(a.tabs, a.prevTab)
	if a.prevTab == nil || i < 0 || i == a.active {
		a.infoText(i18n.T("app.nolast"))
		return
	}
	a.pushJump()
	a.showInPane(i)
}

// trackLast remembers the file that was current before the active one changed.
// A file that was closed in the meantime is not remembered.
func (a *App) trackLast() {
	cur := a.cur()
	if cur != a.seenTab {
		if a.seenTab != nil && slices.Contains(a.tabs, a.seenTab) {
			a.prevTab = a.seenTab
		}
		a.seenTab = cur
	}
}

func (a *App) toggleSide() {
	if a.single {
		return
	}
	a.setSide(!a.sideShown())
	if !a.sideShown() && (a.focus == focusExplorer || a.focus == focusChanges) {
		a.focus = focusEditor
	}
}

func (a *App) toggleTerm() []Effect {
	if a.single {
		return nil
	}
	if a.muxOn() {
		return a.muxToggle()
	}
	if a.termShown() && a.focus == focusTerminal {
		a.setTerm(false)
		return a.setFocus(focusEditor)
	}
	a.setTerm(true)
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

func (a *App) quit() []Effect {
	if d := a.dirtyCount(); d > 0 && !a.confirmQuit {
		a.confirmQuit = true
		a.msg, a.msgErr = fmt.Sprintf(i18n.T("app.quitunsaved"), d), true
		return nil
	}
	a.termCloseAll()
	a.muxClose()
	a.proxy.StopAll()
	return a.exit()
}

// exit ends the program. From here on the View keeps asking the terminal for
// the same keyboard enhancements: when they change, Bubble Tea queries the
// terminal, and the reply (CSI ? flags u) can arrive after input is no longer
// read, so the shell would get it as typed text such as "3u".
func (a *App) exit() []Effect { return []Effect{Quit{}} }

func (a *App) openFind() []Effect {
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
	a.findInput.Focus()
	return nil
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
		a.msg, a.msgErr = i18n.T("app.notfound")+q, true
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
	if a.host != nil {
		a.host.WriteClipboard(s)
		return
	}
	core.WriteSystemClipboard(s)
	// Over SSH there is no local helper tool; the outer terminal can still
	// take the text through OSC 52.
	a.sendToTerminal([]byte(ansi.SetSystemClipboard(s)))
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
		a.fail(errors.New(i18n.T("app.nopathfile")))
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
	a.info(i18n.T("app.pathcopied"), p)
}

func (a *App) pasteText() string {
	if s, ok := core.ReadSystemClipboard(); ok {
		return s
	}
	return a.clipboard
}

// ---- update ----

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	effs := a.update(a.fromTea(msg))
	a.keepTransients()
	a.normalizeSplit()
	a.trackLast()
	a.syncLSP()
	a.notifySelection()
	return a, a.effectCmd(batch(effs, a.scheduleAutoSave(), a.syncPreview(), a.lexTabs(), a.semanticTabs())...)
}

// scheduleAutoSave starts a timer for every named tab edited since the last
// call. The timer is ignored if the buffer changed again before it fires.
func (a *App) scheduleAutoSave() []Effect {
	if !a.cfg.Editor.AutoSave {
		return nil
	}
	var effs []Effect
	for _, t := range a.tabs {
		if !t.dirty() || t.buf.Path == "" || t.autoVer == t.buf.Version {
			continue
		}
		t.autoVer = t.buf.Version
		effs = append(effs, StartTimer{After: a.cfg.Editor.AutoSaveDelay, Fire: AutoSaveDue{tab: t, ver: t.buf.Version}})
	}
	return effs
}

func (a *App) handleAutoSave(m AutoSaveDue) []Effect {
	t := m.tab
	if !t.dirty() || t.buf.Version != m.ver || t.buf.Path == "" || !slices.Contains(a.tabs, t) {
		return nil
	}
	if t.buf.DiskChanged() {
		return nil // never overwrite another process's change silently
	}
	return a.saveTab(t)
}

// update takes what fromTea (inputtea.go) made of a tea.Msg: an Input or an
// Event.
func (a *App) update(msg any) []Effect {
	switch msg := msg.(type) {
	case Event:
		return a.handleEvent(msg)
	case Resized:
		a.setScreen(msg.W, msg.H)
		a.follow = true
	case ReleasesReported:
		a.setReleasesReported(msg.On)
	case KeyReleased:
		a.holdScrollRelease(msg.Key)
		a.lineFlipRelease(msg.Key)
	case Input:
		return a.handleInput(msg)
	}
	return nil
}

// handleInput takes an Input from whichever front end made it.
func (a *App) handleInput(in Input) []Effect {
	switch msg := in.(type) {
	case Mouse:
		return a.handleMouse(msg)
	case Key:
		a.ag.sel = agentSel{}
		if a.hold.buf != nil && !a.hold.continues(msg) {
			a.stopHoldScroll() // another key ends the scroll
		}
		a.lineFlipOtherKey(msg)
		a.hoverSeq++ // a pending mouse hover is stale once the user types
		a.hoverWord = wordSpan{}
		// Fast typing can deliver several runes in one message; Helix-style
		// commands need them one at a time.
		if utf8.RuneCountInString(msg.Text) > 1 {
			var cmds [][]Effect
			for _, r := range msg.Text {
				cmds = append(cmds, a.handleKey(Key{Code: r, Text: string(r), Mod: msg.Mod}))
			}
			return batch(cmds...)
		}
		return a.handleKey(msg)
	case Paste:
		a.hoverSeq++
		a.hoverWord = wordSpan{}
		return a.handlePaste(msg)
	case Composition:
		// Nothing uses a composition yet: a terminal never composes, and a GUI sends
		// the committed text as input.key (its text), so taking it here too would
		// type it twice. B10 decides what a composition does.
	}
	return nil
}

func (a *App) handleLSPEvent(e lsp.Event) []Effect {
	switch {
	case e.Err != nil:
		a.fail(e.Err)
	case e.Message != "":
		a.info("[%s] %s", e.Server, firstLine(e.Message))
	case e.Edit != nil:
		if len(e.Edit) > 0 {
			a.applyWorkspaceEdit(e.Edit)
		}
	case e.Document != "":
		if strings.HasPrefix(e.Document, "file:") {
			if a.openFile(lsp.Path(e.Document)) {
				return a.setFocus(focusEditor)
			}
			return nil
		}
		return a.openURL(e.Document)
	case e.URI != "":
		p := lsp.Path(e.URI)
		if a.lsp.diags[p] == nil {
			a.lsp.diags[p] = map[string][]lsp.Diagnostic{}
		}
		a.lsp.diags[p][e.Server] = e.Diagnostics
	}
	return nil
}

func (a *App) handleKey(msg Key) []Effect {
	key := msg.String()
	a.msg = ""
	if a.sig != nil && (key == "esc" || !a.sigVisible()) {
		a.closeSignature()
	}
	if a.hoverAt != nil {
		// A popup the pointer opened never takes keys: it closes whatever has the
		// focus, and the key goes on to it.
		a.hoverText, a.hoverAt = "", nil
	}
	if a.card != nil && a.card.byPointer {
		a.card = nil // the same for a comment card the pointer opened
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
	if a.composer != nil {
		return a.handleComposerKey(msg)
	}
	if a.confirm != nil {
		return a.handleConfirmKey(key)
	}
	if a.props.Len() > 0 {
		return a.handleProposalKey(msg)
	}
	if a.ctx != nil {
		return a.handleContextKey(msg)
	}
	if a.cmdOpen {
		return a.handleCmdlineKey(msg)
	}
	if a.inResizeMode() {
		return a.handleResizeKey(key)
	}
	if a.card != nil {
		if cmd, handled := a.handleCardKey(msg); handled {
			return cmd
		}
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
	if a.snip != nil {
		if cmd, handled := a.handleSnippetKey(msg); handled {
			return cmd
		}
	}

	if a.focus == focusTerminal {
		// The shell owns the keyboard; only these stay global. Ctrl+J is not
		// one of them: many terminals send Shift+Enter as LF, which arrives as
		// ctrl+j, and the program in the shell takes it as a newline.
		switch key {
		case "ctrl+q":
			return a.quit()
		case "ctrl+`":
			return a.toggleTerm()
		case "alt+i":
			return a.toggleAgentPanel()
		case "alt+r":
			return a.startResize()
		case "f1":
			return a.pal.show(a.root, ">")
		}
		a.termKey(msg)
		return nil
	}

	insert := a.helix && a.mode == modeInsert && a.focus == focusEditor
	if a.single && singleBlocked(key, a.typing()) {
		return nil
	}
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
	case "ctrl+`":
		return a.toggleTerm()
	case "ctrl+j", "ctrl+space":
		// Ctrl+Space is completion wherever the editor accepts typing: VS Code
		// mode, or Helix insert mode.
		if key == "ctrl+j" && a.muxOn() {
			return a.muxResult(a.muxOpen())
		}
		typing := a.typing()
		if key == "ctrl+j" || !typing {
			return a.toggleTerm()
		}
	case "ctrl+g":
		return a.openDiff(baseHead)
	case "ctrl+e":
		if a.focus == focusExplorer && len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
		a.setSide(true)
		return a.setFocus(focusExplorer)
	case "ctrl+n":
		if !insert {
			return a.newFile()
		}
	case "ctrl+w":
		if !insert {
			return a.closeTab(false)
		}
	case "ctrl+tab":
		a.switchLast()
		return nil
	case "ctrl+\\":
		return a.toggleEditorSplit()
	case "ctrl+1":
		return a.focusPane(0)
	case "ctrl+2":
		return a.focusPane(1)
	case "alt+a":
		return a.sendContext()
	case "alt+i":
		return a.toggleAgentPanel()
	case "alt+r":
		return a.startResize()
	}

	switch a.focus {
	case focusExplorer:
		return a.handleExplorerKey(msg)
	case focusChanges:
		return a.handleChangesKey(msg)
	case focusAgent:
		return a.handleAgentKey(msg)
	}
	t := a.cur()
	if t == nil {
		if key == "esc" || key == "tab" || key == "space" {
			a.setSide(true)
			return a.setFocus(focusExplorer)
		}
		if key == ":" {
			return a.openCmdline(":")
		}
		return nil
	}
	switch t.kind {
	case tabImage:
		if a.pending == " " {
			a.pending = ""
			if key == "esc" {
				return nil
			}
			return a.spaceMode(key)
		}
		if a.pending == "space v" {
			a.pending = ""
			return a.reviewKey(key)
		}
		if a.pending == "space w" {
			a.pending = ""
			return a.windowKey(key)
		}
		switch key {
		case "q", "esc":
			return a.closeTab(true)
		case ":":
			return a.openCmdline(":")
		case "space":
			a.pending = " "
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
		a.infoText(i18n.T("app.fullpreview"))
		return nil
	}
	return a.handleEditorKey(msg)
}

func (a *App) handleCompletionKey(msg Key) ([]Effect, bool) {
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

func (a *App) handlePaletteKey(msg Key) []Effect {
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
	return a.paletteInput(msg)
}

// paletteInput passes a key or a paste to the palette's query.
func (a *App) paletteInput(msg Input) []Effect {
	p := a.pal
	old := p.input.Value()
	a.lineInputUpdate(&p.input, msg)
	p.sel = 0
	p.filter(a.commands)
	if p.picking && p.live && p.input.Value() != old {
		return p.queryLater()
	}
	return nil
}

func (a *App) submitPalette() []Effect {
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
			a.pushJump()
			if a.openFile(filepath.Join(a.root, f)) {
				return a.setFocus(focusEditor)
			}
		}
	}
	return nil
}

func (a *App) handleExplorerKey(msg Key) []Effect {
	e := a.explorer
	key := msg.String()
	if a.pending == "space v" {
		a.pending = ""
		return a.reviewKey(key)
	}
	if a.pending == "space w" {
		a.pending = ""
		return a.windowKey(key)
	}
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
		e.sel = max(0, e.sel-(a.screenH()-3)/2)
	case "pgdown", "ctrl+d":
		e.sel = min(len(e.flat)-1, e.sel+(a.screenH()-3)/2)
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
		if key != "enter" && key != "o" {
			a.openTransient(n.path)
		} else if a.openFile(n.path) {
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
		a.infoText(i18n.T("app.treerefreshed"))
	case "space":
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

func (a *App) handleFindKey(msg Key) []Effect {
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
	return a.findInputUpdate(msg)
}

// findInputUpdate passes a key or a paste to the find bar.
func (a *App) findInputUpdate(msg Input) []Effect {
	before := a.findInput.Value()
	a.lineInputUpdate(&a.findInput, msg)
	if a.findInput.Value() != before {
		a.findNext(a.findOrigin, false)
	}
	return nil
}

// handleEditorKey is the modeless (VS Code style) editing used by the
// vscode keymap and by Helix insert mode.
func (a *App) handleEditorKey(msg Key) []Effect {
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
func multiEditable(msg Key) bool {
	if typed(msg) != "" {
		return true
	}
	switch msg.String() {
	case "left", "right", "up", "down", "shift+left", "shift+right", "shift+up", "shift+down",
		"home", "end", "shift+home", "shift+end", "ctrl+left", "ctrl+right", "alt+b", "alt+f",
		"ctrl+shift+left", "ctrl+shift+right", "enter", "tab", "backspace", "ctrl+h", "delete", "space":
		return true
	}
	return false
}

func (a *App) editorKey(t *tab, msg Key) []Effect {
	if t == nil {
		return nil
	}
	b := t.buf
	a.follow = true
	page := a.pageSize()

	switch msg.String() {
	case "esc":
		b.ClearSelection()
		b.ClearExtra()
		a.comp = nil
	case "alt+left":
		return a.jumpMove(-1)
	case "alt+right":
		return a.jumpMove(1)
	case "alt+pgup":
		a.scrollHalfScreen(t, -1)
	case "alt+pgdown":
		a.scrollHalfScreen(t, 1)
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
		a.infoText(i18n.T("app.copied"))
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
	case "ctrl+space":
		return a.completion()
	case "f12":
		return a.gotoDefinition()
	case "shift+f12":
		return a.references()
	case "f2":
		return a.rename()
	case "space":
		b.Insert(" ")
		a.comp = nil
	default:
		if s := typed(msg); s != "" {
			if rs := []rune(s); len(rs) == 1 {
				b.TypeRune(rs[0])
			} else {
				b.Insert(s)
			}
			return a.afterTyping(t, []rune(s))
		}
	}
	return nil
}

// afterTyping opens or refreshes the completion and signature popups.
func (a *App) afterTyping(t *tab, typed []rune) []Effect {
	if len(typed) != 1 {
		return nil
	}
	return batch(a.signatureTyped(t, typed[0]), a.completionTyped(t, typed[0]))
}

func (a *App) completionTyped(t *tab, r rune) []Effect {
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

func (a *App) refreshCompletion() []Effect {
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

func (a *App) editorOpts(t *tab, focused bool) editorOpts {
	o := editorOpts{
		focused:      focused && !a.findOpen && !a.cmdOpen,
		cursorColor:  curTheme.Cursor,
		cursorAt:     t.buf.Cur,
		relative:     a.relativeLineNumbers(),
		indentGuides: a.cfg.Editor.IndentGuides,
		title:        a.titleOf(t),
		diags:        a.lsp.diagnostics(t.buf.Path),
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
		sel := [2]buffer.Pos{c, c}
		if r.HasAnchor && r.Anchor != r.Cur {
			sel = [2]buffer.Pos{r.Start(), r.End()}
		}
		o.extras = append(o.extras, extraCursor{cursor: c, sel: sel})
	}
	return o
}

func findLabel() string { return i18n.T("app.findlabel") }

// copyHover copies the hover markdown source to the clipboard.
func (a *App) copyHover() {
	a.copyText(a.hoverText)
	a.infoText(i18n.T("app.hovercopied"))
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

// overlayOpen reports whether a floating window (palette or project search)
// has the keyboard.
func (a *App) overlayOpen() bool { return a.pal.open || a.search.open }

// infoText shows an already-formatted message in the status line.
func (a *App) infoText(s string) {
	a.msg, a.msgErr = s, false
}
