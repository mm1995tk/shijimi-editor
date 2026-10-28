// Package ui implements the shijimi terminal IDE.
package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/caret"
	"github.com/mm1995tk/shijimi-editor/internal/config"
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

// App is the root Bubble Tea model.
type App struct {
	root      string
	gitTop    string
	cfg       *config.Config
	keymap    helixKeymap
	lastIns   *insertRecord     // what "." repeats
	recIns    *insertRecord     // the insert being recorded, until Esc
	replaying bool              // "." is running; do not record
	caret     *caret.Writer     // the terminal output; nil in tests
	caretAt   *tea.Cursor       // where the user is typing, for the IME
	cellSize  func() (w, h int) // terminal cell size in pixels; nil in tests
	lsp       *lspManager
	w, h      int
	focus     focus
	explorer  *explorer
	treeTicks int // watch ticks since the tree was last reloaded
	term      *terminal
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

	showSide bool
	showTerm bool

	// pane sizes picked by the user (0 = automatic), see resize.go
	sideWPref   int
	panelHPref  int
	previewPct  int
	diffLeftPct int // base pane share of a split diff; 0 = half
	explorerPct int // file tree share of the diff mode sidebar; 0 = 7/10
	resizeMode  bool
	resizing    resizeTarget // border being dragged with the mouse

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
	sig         *signaturePopup
	sigSeq      int // bumped on every request and close; stale replies are dropped
	snip        *snippetSession
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

	// suggestion panel and ACP agent (see suggest.go, agent.go)
	ag         *agentState
	sg         suggestState
	showAgent  bool
	agentWPref int // panel width picked by the user (0 = automatic)

	clipboard    string
	msg          string
	msgErr       bool
	confirmQuit  bool
	confirmClose bool
	follow       bool // scroll to the cursor on next render
	dragging     bool
	lastClick    click // the previous left press, to spot a double click
	keyReleases  bool  // the terminal reports key releases
	hold         holdScroll
	holdSeq      int // tells the current hold-scroll tick chain from stale ones

	single    bool // 単体編集モード: 1 つのファイルだけを開き、閉じたら終了する
	tabSpans  []span
	tabOffset int
	commands  []command
	mermaid   map[string]*previewJob
	mdImages  map[string]*previewJob // images in the Markdown preview, by file path
	// previewGen counts finished jobs, so the preview is drawn again when one
	// finishes
	previewGen int
}

// New creates the app rooted at dir, optionally opening file.
func New(dir, file string) *App {
	abs, _ := filepath.Abs(dir)
	cfg := config.Load(abs)
	i18n.SetLang(cfg.Editor.UILanguage)
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
		ag:       newAgentState(),
	}
	a.explorer.icons = cfg.Editor.FileIcons
	if cfg.Editor.Theme != "" {
		applyTheme(themeOrDefault(cfg.Editor.Theme))
	}
	a.findInput = textinput.New()
	a.findInput.Prompt = ""
	a.findInput.Placeholder = i18n.T("app.find")
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
		root:     dir,
		cfg:      cfg,
		lsp:      newLSPManager(cfg, dir),
		explorer: &explorer{root: &node{name: filepath.Base(dir), path: dir, isDir: true, expanded: true, depth: -1}}, // not loaded: no tree is shown
		term:     newTerminal(dir),
		proxy:    proxy.NewManager(),
		pal:      newPalette(),
		search:   newSearchPanel(cfg.Search),
		focus:    focusEditor,
		helix:    cfg.Editor.Keymap != "vscode",
		gd:       gitDiff{split: true},
		ag:       newAgentState(),
		single:   true,
		showSide: false,
	}
	if cfg.Editor.Theme != "" {
		applyTheme(themeOrDefault(cfg.Editor.Theme))
	}
	a.findInput = textinput.New()
	a.findInput.Prompt = ""
	a.findInput.Placeholder = i18n.T("app.find")
	a.cmdInput = textinput.New()
	a.cmdInput.Prompt = ""
	styleInput(&a.findInput, cPaletteBg)
	styleInput(&a.cmdInput, cEditorBg)
	styleInput(&a.pal.input, cPaletteBg)
	a.search.restyle()
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
		"ctrl+pgdown", "ctrl+pgup", "alt+a", "alt+i", "alt+r",
		"alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
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
	case "o", "open", "e", "edit", "n", "new", "sh", "run", "!", "gitdiff", "gd", "review",
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
	removeShellIntegration()
}

func styleInput(ti *textinput.Model, bg string) {
	st := bgfg(bg, cEditorFg)
	s := ti.Styles()
	s.Focused = textinput.StyleState{Text: st, Placeholder: bgfg(bg, curTheme.Dim), Prompt: st}
	s.Blurred = s.Focused
	// bubbles' cursor applies Reverse on top of this colour, so it ends up as
	// the cell background.
	s.Cursor.Color = lipgloss.Color(curTheme.Cursor)
	s.Cursor.Blink = false
	ti.SetStyles(s)
}

func (a *App) Init() tea.Cmd {
	if a.single {
		return watchTick()
	}
	return tea.Batch(a.lsp.wait(), watchTick(), a.probeRepo())
}

// ---- layout ----

type layout struct {
	top, bodyH     int // rows between the diff mode header and the help/status lines
	sideW          int
	rightX, rightW int
	agentX, agentW int // the suggestion panel; agentW is 0 when it is not shown
	editY, editH   int
	panelY, panelH int
	statusY        int
}

func (a *App) layout() layout {
	var l layout
	l.statusY = a.h - 1
	l.bodyH = a.bodyH()
	if a.gd.on {
		l.top = 1
	}
	if a.showSide {
		l.sideW = a.sideWidth()
		l.rightX = l.sideW + 1
	}
	l.rightW = max(0, a.w-l.rightX)
	if a.agentShown(l.sideW) {
		l.agentW = a.agentWidth()
		l.agentX = a.w - l.agentW
		l.rightW = max(0, l.agentX-1-l.rightX)
	}
	l.editY = l.top + 1
	if a.findOpen {
		l.editY++
	}
	if a.showTerm {
		l.panelH = a.panelHeight()
	}
	l.editH = max(0, l.top+l.bodyH-l.editY-l.panelH)
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
	if a.single {
		f = focusEditor
	}
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
	a.info(i18n.T("app.saved"), a.rel(t.buf.Path))
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
	a.info(i18n.T("app.savedmany"), n)
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
	return a.pal.ask(i18n.T("app.saveas"), initial, func(v string) tea.Cmd {
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
	if a.single {
		return nil
	}
	a.addTab(newTab(buffer.New()))
	return a.setFocus(focusEditor)
}

func (a *App) newFileIn(dir string) tea.Cmd {
	return a.pal.ask(i18n.T("app.newfile"), dir+string(filepath.Separator), func(v string) tea.Cmd {
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
	return a.pal.ask(i18n.T("app.newfolder"), a.explorer.targetDir()+string(filepath.Separator), func(v string) tea.Cmd {
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
	return a.pal.ask(i18n.T("app.newname"), old, func(v string) tea.Cmd {
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

func (a *App) deleteFile() tea.Cmd {
	n := a.explorer.selected()
	if n == nil {
		return nil
	}
	target := n.path
	return a.pal.ask(fmt.Sprintf(i18n.T("app.confirmdelete"), a.rel(target)), "", func(v string) tea.Cmd {
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

func (a *App) closeTab(force bool) tea.Cmd {
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
	a.dropTab(a.active)
	if a.active >= len(a.tabs) {
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
	if a.single {
		return
	}
	a.showSide = !a.showSide
	if !a.showSide && (a.focus == focusExplorer || a.focus == focusChanges) {
		a.focus = focusEditor
	}
}

func (a *App) toggleTerm() tea.Cmd {
	if a.single {
		return nil
	}
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
		a.msg, a.msgErr = fmt.Sprintf(i18n.T("app.quitunsaved"), d), true
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
	writeSystemClipboard(s)
	// Over SSH there is no local helper tool; the outer terminal can still
	// take the text through OSC 52.
	if a.caret != nil {
		_ = a.caret.Send([]byte(ansi.SetSystemClipboard(s)))
	}
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
	a.keepTransients()
	a.syncLSP()
	a.notifySelection()
	return a, tea.Batch(cmd, a.scheduleAutoSave(), a.syncPreview(), a.lexTabs(), a.semanticTabs())
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
	if cmd, ok := a.handleAgentMsg(msg); ok {
		return cmd
	}
	switch msg := msg.(type) {
	case autoSaveMsg:
		return a.handleAutoSave(msg)
	case watchMsg:
		if a.single {
			a.checkDiskChanges()
			return watchTick()
		}
		reload := a.checkDiskChanges()
		sg := a.suggestTick()
		a.treeTicks++
		if reload || a.treeTicks >= treeRefreshTicks {
			a.treeTicks = 0
			a.explorer.refresh()
		}
		if a.gd.on {
			a.gd.ticks++
			if reload || a.gd.ticks%gitDiffRefreshTicks == 0 {
				return tea.Batch(watchTick(), sg, a.reloadChanges())
			}
		}
		return tea.Batch(watchTick(), sg)
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		if a.cellSize != nil {
			updateCellSize(a.cellSize())
		}
		a.follow = true
	case termOutputMsg, termDoneMsg:
		cmd := a.term.update(msg)
		// A program in the terminal panel (Claude Code, tmux, …) copies with
		// OSC 52; pass the text on to the clipboard. Any program there can do
		// this unasked, so every copy is shown.
		for _, tab := range a.term.tabs {
			c, ok := tab.vt.TakeClipboard()
			switch {
			case !ok:
			case c.TooLarge:
				a.fail(errors.New(i18n.T("app.termcopybig")))
			default:
				a.copyText(c.Text)
				a.info(i18n.T("app.termcopied"), utf8.RuneCountInString(c.Text))
			}
		}
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
		return tea.Batch(a.handleLSPEvent(lsp.Event(msg)), a.lsp.wait())
	case mermaidStartMsg:
		return a.handleMermaidStart(msg)
	case mermaidDoneMsg:
		a.handleMermaidDone(msg)
	case mdImageDoneMsg:
		a.handleMDImageDone(msg)
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
			a.infoText(i18n.T("app.nohover"))
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
	case signatureMsg:
		a.showSignature(msg)
	case workspaceEditMsg:
		a.applyWorkspaceEdit(msg.edit)
	case symbolsMsg:
		return a.showSymbols(msg)
	case symbolQueryMsg:
		return a.runSymbolQuery(msg)
	case workspaceSymbolsMsg:
		a.showWorkspaceSymbols(msg)
	case codeActionsMsg:
		return a.showCodeActions(msg)
	case codeActionResolvedMsg:
		return a.runCodeAction(msg)
	case formatMsg:
		if len(msg.edits) > 0 {
			msg.tab.buf.ApplyEdits(toEdits(msg.tab.buf, msg.edits))
		}
		if msg.save {
			return a.saveTab(msg.tab)
		}
		a.infoText(i18n.T("app.formatted"))
	case formatterDoneMsg:
		return a.handleFormatterDone(msg)
	case lspErrMsg:
		a.fail(msg.err)
	case openURLMsg:
		if msg.err != nil {
			a.fail(msg.err)
		} else {
			a.info(i18n.T("app.opened"), msg.url)
		}
	case repoProbedMsg:
		a.handleRepoProbed(msg)
	case changesLoadedMsg:
		a.applyChanges(msg)
	case highlightMsg:
		msg.h.apply(msg)
	case semanticMsg:
		msg.h.sem.apply(msg)
	case holdScrollMsg:
		return a.holdScrollTick(msg.seq)
	case tea.KeyboardEnhancementsMsg:
		a.keyReleases = msg.SupportsEventTypes()
	case tea.KeyReleaseMsg:
		a.holdScrollRelease(msg)
	case searchTickMsg:
		return a.searchTick(msg)
	case searchDoneMsg:
		a.searchDone(msg)
	case prFoundMsg:
		return a.handlePRFound(msg)
	case branchPRsMsg:
		return a.handleBranchPRs(msg)
	case prCheckedOutMsg:
		return a.handlePRCheckedOut(msg)
	case prPulledMsg:
		return a.handlePRPulled(msg)
	case prReviewMsg:
		return a.handlePRReview(msg)
	case viewedMsg:
		a.handleViewed(msg)
	case reviewSentMsg:
		return a.handleReviewSent(msg)
	case prListMsg:
		return a.showPRList(msg)
	case mouseEvent:
		return a.handleMouse(msg)
	case tea.MouseMsg:
		return a.handleMouse(toMouseEvent(msg))
	case tea.KeyPressMsg:
		a.ag.sel = agentSel{}
		if a.hold.buf != nil && !(msg.Code == a.hold.code && msg.Mod.Contains(tea.ModCtrl)) {
			a.stopHoldScroll() // another key ends the scroll
		}
		a.hoverSeq++ // a pending mouse hover is stale once the user types
		a.hoverWord = wordSpan{}
		// Fast typing can deliver several runes in one message; Helix-style
		// commands need them one at a time.
		if utf8.RuneCountInString(msg.Text) > 1 {
			var cmds []tea.Cmd
			for _, r := range msg.Text {
				cmds = append(cmds, a.handleKey(tea.KeyPressMsg{Code: r, Text: string(r), Mod: msg.Mod}))
			}
			return tea.Batch(cmds...)
		}
		return a.handleKey(msg)
	case tea.PasteMsg:
		a.hoverSeq++
		a.hoverWord = wordSpan{}
		return a.handlePaste(msg)
	}
	return nil
}

func (a *App) handleLSPEvent(e lsp.Event) tea.Cmd {
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

func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
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
		return a.term.update(msg)
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
		a.showSide = true
		return a.setFocus(focusExplorer)
	case "ctrl+n":
		if !insert {
			return a.newFile()
		}
	case "ctrl+w":
		if !insert {
			return a.closeTab(false)
		}
	case "ctrl+pgdown":
		a.switchTab(1)
		return nil
	case "ctrl+pgup":
		a.switchTab(-1)
		return nil
	case "alt+a":
		return a.sendContext()
	case "alt+i":
		return a.toggleAgentPanel()
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
	case focusAgent:
		return a.handleAgentKey(msg)
	}
	t := a.cur()
	if t == nil {
		if key == "esc" || key == "tab" || key == "space" {
			a.showSide = true
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

func (a *App) handleCompletionKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
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

func (a *App) handlePaletteKey(msg tea.KeyPressMsg) tea.Cmd {
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
func (a *App) paletteInput(msg tea.Msg) tea.Cmd {
	p := a.pal
	old := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.sel = 0
	p.filter(a.commands)
	if p.picking && p.live && p.input.Value() != old {
		return tea.Batch(cmd, p.queryLater())
	}
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
			a.pushJump()
			if a.openFile(filepath.Join(a.root, f)) {
				return a.setFocus(focusEditor)
			}
		}
	}
	return nil
}

func (a *App) handleExplorerKey(msg tea.KeyPressMsg) tea.Cmd {
	e := a.explorer
	key := msg.String()
	if a.pending == "space v" {
		a.pending = ""
		return a.reviewKey(key)
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

func (a *App) handleFindKey(msg tea.KeyPressMsg) tea.Cmd {
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
func (a *App) findInputUpdate(msg tea.Msg) tea.Cmd {
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
func (a *App) handleEditorKey(msg tea.KeyPressMsg) tea.Cmd {
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
func multiEditable(msg tea.KeyPressMsg) bool {
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

func (a *App) editorKey(t *tab, msg tea.KeyPressMsg) tea.Cmd {
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
func (a *App) afterTyping(t *tab, typed []rune) tea.Cmd {
	if len(typed) != 1 {
		return nil
	}
	return tea.Batch(a.signatureTyped(t, typed[0]), a.completionTyped(t, typed[0]))
}

func (a *App) completionTyped(t *tab, r rune) tea.Cmd {
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
// the program, its drag and release do too, wherever the pointer is. Otherwise
// (or with Shift held) a left drag selects text and copies it on release. ok
// is false when the event is not for the terminal.
func (a *App) termMouse(l layout, msg mouseEvent) (cmd tea.Cmd, ok bool) {
	t := a.term
	if !a.showTerm {
		return nil, false
	}
	x, y := msg.X-l.rightX, msg.Y-l.panelY-2
	if t.sel.dragging {
		switch msg.Action {
		case mouseMotion:
			t.dragTo(x, y)
		case mouseRelease:
			t.dragTo(x, y)
			if s := t.endSelect(); s != "" {
				a.copyText(s)
				a.info(i18n.T("app.termcopied"), utf8.RuneCountInString(s))
			}
		default:
			t.sel.dragging = false // the release was lost
			return nil, false
		}
		return nil, true
	}
	inBody := x >= 0 && x < l.rightW && y >= 0 && msg.Y < l.statusY
	grabbed := t.grab
	if !grabbed && !inBody {
		return nil, false
	}
	sent := false
	if grabbed || !msg.Shift {
		sent = t.mouse(msg, x, y)
	}
	switch msg.Action {
	case mousePress:
		if sent {
			t.sel = termSel{}
			t.grab = true
			return a.setFocus(focusTerminal), true
		}
		if msg.Button == tea.MouseLeft {
			t.selectAt(x, y)
			return a.setFocus(focusTerminal), true
		}
	case mouseRelease:
		t.grab = false
	}
	return nil, sent || grabbed
}

func (a *App) handleMouse(msg mouseEvent) tea.Cmd {
	if a.search.open {
		return a.searchMouse(msg)
	}
	if a.pal.open {
		if msg.Action == mousePress && msg.Button == tea.MouseLeft {
			a.pal.close()
		}
		return nil
	}
	l := a.layout()
	if a.ag.sel.dragging {
		if cmd, ok := a.agentDrag(l, msg); ok {
			return cmd
		}
	}
	if a.resizing == resizeNone && !a.dragging && !a.term.sel.dragging {
		if cmd, ok := a.agentMouse(l, msg); ok {
			return cmd
		}
	}
	// Some terminals report a drag as motion with no button; while a border
	// or a selection is being dragged, motion is the drag whatever it says.
	if msg.Action == mouseMotion && msg.Button == tea.MouseNone && a.resizing == resizeNone && !a.dragging && !a.term.sel.dragging {
		return a.passiveMotion(l, msg)
	}
	x, y := msg.X, msg.Y
	wheel := 0
	switch msg.Button {
	case tea.MouseWheelUp:
		wheel = -3
	case tea.MouseWheelDown:
		wheel = 3
	}
	// Sideways: the horizontal wheel, or the vertical one with Shift held.
	hwheel := 0
	switch {
	case msg.Button == tea.MouseWheelLeft:
		hwheel = -hscrollWheel
	case msg.Button == tea.MouseWheelRight:
		hwheel = hscrollWheel
	}
	// A popup the pointer opened only owns the wheel while the pointer is on it.
	if wheel != 0 && a.hoverText != "" && (a.hoverAt == nil || a.inHoverBox(l, screenPt{x, y})) {
		if msg.Action == mousePress {
			a.scrollHover(wheel)
		}
		return nil
	}
	if a.hoverText != "" && msg.Button == tea.MouseLeft {
		// The button copies on press; the matching release must not close the hover.
		if bx, by, ok := a.hoverPos(l); ok && a.hoverView(l).copyHit(x-bx, y-by) {
			if msg.Action == mousePress {
				a.copyHover()
			}
			return nil
		}
	}
	if a.card != nil && msg.Action == mousePress {
		if box, bx, by := a.cardBox(l); box != nil && x >= bx && y >= by && y < by+len(box) && x < bx+ansi.StringWidth(box[0]) {
			if wheel != 0 {
				a.card.scroll = max(0, a.card.scroll+wheel)
			}
			return nil // reading the card
		}
		if wheel == 0 {
			a.card = nil
		}
	}
	// Past the branches that keep the popup open: the pointer's hover is over.
	a.hoverSeq++
	a.hoverWord = wordSpan{}
	a.hoverCell = screenPt{-1, -1}
	a.hoverText, a.hoverAt = "", nil
	if msg.Action == mousePress && hwheel == 0 && wheel != 0 && msg.Shift && a.inEditorText(l, x, y) {
		hwheel = wheel / 3 * hscrollWheel // Shift turns the vertical wheel sideways, over the text only
	}
	if hwheel != 0 && msg.Action == mousePress {
		if a.inEditorText(l, x, y) {
			a.scrollCols(a.cur(), hwheel)
			return nil
		}
	}
	if wheel != 0 && msg.Action == mousePress {
		switch {
		case y < l.top || y >= l.top+l.bodyH:
			// the diff mode header and help lines
		case a.showSide && x < l.sideW:
			e := a.explorer
			if a.gd.on && y-l.top >= a.explorerH() {
				g := &a.gd
				body := l.bodyH - a.explorerH() - 1
				g.scroll = clamp(g.scroll+wheel, 0, max(0, len(g.vis())-body))
				g.keepSelVisible(body)
			} else {
				eh := a.explorerH()
				e.scroll = clamp(e.scroll+wheel, 0, max(0, len(e.flat)-(eh-1)))
				e.sel = clamp(e.sel, e.scroll, e.scroll+eh-2)
			}
		case a.showTerm && y >= l.panelY:
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
				}
			}
		}
		return nil
	}

	if a.resizing != resizeNone {
		switch msg.Action {
		case mouseMotion:
			a.dragResize(l, x, y)
			return nil
		case mouseRelease:
			a.resizing = resizeNone
			return nil
		}
		// A press means the release was lost (pointer left the window).
		a.resizing = resizeNone
	}
	if msg.Action == mousePress && msg.Button == tea.MouseLeft {
		if r := a.resizeHit(l, x, y); r != resizeNone {
			a.resizing = r
			if r == resizeExplorer {
				return a.setFocus(focusChanges)
			}
			return nil
		}
	}

	if cmd, ok := a.termMouse(l, msg); ok {
		return cmd
	}

	left := msg.Button == tea.MouseLeft
	switch msg.Action {
	case mouseRelease:
		a.dragging = false
		return nil
	case mouseMotion:
		if a.dragging {
			a.mouseToCursor(l, x, y, true)
		}
		return nil
	}
	if !left {
		return nil
	}
	double := a.isDoubleClick(x, y)

	switch {
	case y < l.top || y >= l.top+l.bodyH:
		return nil // the diff mode header and help lines
	case a.showSide && x < l.sideW:
		sy := y - l.top
		if a.gd.on && sy >= a.explorerH() {
			g := &a.gd
			if vis, i := g.vis(), sy-1-a.explorerH()+g.scroll; sy > a.explorerH() && i < len(vis) {
				r := vis[i]
				g.selectRow(r)
				if g.rows[r].file < 0 {
					g.setFolder(r, g.closed[g.rows[r].path])
					return a.setFocus(focusChanges)
				}
				return a.openChange(g.files[g.sel])
			}
			return a.setFocus(focusChanges)
		}
		row := sy - 1 + a.explorer.scroll
		if sy >= 1 && row < len(a.explorer.flat) {
			a.explorer.sel = row
			n := a.explorer.flat[row]
			switch {
			case n.isDir:
				a.explorer.toggle(n)
			case !double:
				a.openTransient(n.path)
			case a.openFile(n.path):
				return a.setFocus(focusEditor)
			}
		}
		return a.setFocus(focusExplorer)
	case y == l.top:
		for i, s := range a.tabSpans {
			if x >= s.x0 && x < s.x1 {
				a.active = i
				if double {
					a.tabs[i].transient = false
				}
				a.follow = true
				a.mode = modeNormal
				return a.setFocus(focusEditor)
			}
		}
	case a.showTerm && y >= l.panelY:
		if y == l.panelY+1 {
			a.term.clickTab(x - l.rightX)
		}
		return a.setFocus(focusTerminal)
	case y >= l.editY && y < l.editY+l.editH:
		t := a.cur()
		if t == nil {
			return nil
		}
		if t.kind == tabText {
			if a.diffFile(t) != nil {
				if y == l.editY {
					if !a.diffSingle(a.diffFile(t)) {
						a.toggleSplit()
					}
					return nil
				}
				g := a.dvGeom(l)
				if x >= g.barX {
					if dv := a.diffFor(t); dv != nil {
						a.diffBarClick(t, dv, g, y)
					}
					return nil
				}
				if x < g.rightX {
					return nil // the base side is read-only
				}
				if th := a.markerAt(l, x, y); th != nil {
					a.card = &prCard{thread: th, path: a.repoPath(t.buf.Path), at: &screenPt{x, y}}
					return nil
				}
			} else if t.preview != previewOff && x >= l.rightX+a.editW(t, l) {
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

// click is a left press, kept to tell a double click from two single ones.
type click struct {
	x, y int
	at   time.Time
}

const doubleClickTime = 400 * time.Millisecond

// isDoubleClick records a left press at (x, y) and reports whether it is the
// second press of a double click. A third press starts over.
func (a *App) isDoubleClick(x, y int) bool {
	now := time.Now()
	p := a.lastClick
	double := !p.at.IsZero() && p.x == x && p.y == y && now.Sub(p.at) < doubleClickTime
	a.lastClick = click{x: x, y: y, at: now}
	if double {
		a.lastClick.at = time.Time{}
	}
	return double
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

func (a *App) View() tea.View {
	v := tea.NewView(a.render())
	// Image data goes out before the frame whose placeholder cells show it.
	if g := takeGraphics(); g != "" && a.caret != nil {
		_ = a.caret.Send([]byte(g))
	}
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if a.MouseHover() {
		// A resting pointer is only noticed if motion without a button is reported.
		v.MouseMode = tea.MouseModeAllMotion
	}
	// Key releases let Ctrl+D / Ctrl+U scroll only while held. Some terminals
	// (herdr 0.9.5) then drop text committed by an IME, so they are asked
	// for only where those keys scroll.
	v.KeyboardEnhancements.ReportEventTypes = a.wantKeyReleases()
	// The terminal cursor stays hidden (see package caret), but an IME shows
	// its composition where it is.
	v.Cursor = a.caretAt
	return v
}

func (a *App) render() string {
	if a.w <= 0 || a.h <= 0 {
		return ""
	}
	if a.w < 30 || a.h < 8 {
		a.caretAt = nil
		return i18n.T("app.toosmall")
	}
	l := a.layout()
	if t := a.curText(); t != nil {
		if a.follow {
			if dv := a.diffFor(t); dv != nil {
				a.ensureDiffVisible(t, dv, a.editW(t, l), a.dvGeom(l).h)
			} else if !a.fullPreview(t) { // the hidden editor is 0 wide: following it would scroll to the cursor column
				ensureCursorVisible(t.buf, a.editW(t, l), l.editH)
			}
			a.follow = false
		}
		t.buf.ScrollRow = clamp(t.buf.ScrollRow, 0, max(0, len(t.buf.Lines)-1))
	}

	right := make([]string, 0, l.bodyH)
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
	default:
		right = append(right, a.renderText(t, l, editorFocused)...)
	}
	if a.showTerm {
		right = append(right, a.term.render(l.rightW, l.panelH, a.focus == focusTerminal && !a.overlayOpen())...)
	}

	rows := make([]string, l.bodyH)
	var side []string
	activePath := ""
	if t != nil {
		activePath = t.path()
	}
	if a.showSide {
		side = a.explorer.render(l.sideW, a.explorerH(), a.focus == focusExplorer && !a.overlayOpen(), activePath)
		if a.gd.on {
			side = append(side, a.renderChanges(l.sideW, l.bodyH-a.explorerH(), a.focus == focusChanges && !a.overlayOpen())...)
		}
	}
	var agent []string
	if l.agentW > 0 {
		agent = a.renderAgent(l.agentW, l.bodyH, a.focus == focusAgent && !a.overlayOpen())
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
		if agent != nil {
			if i < len(right) {
				if rw := ansi.StringWidth(right[i]); rw < l.rightW {
					sb.WriteString(stEmpty.Render(strings.Repeat(" ", l.rightW-rw)))
				}
			} else {
				sb.WriteString(stEmpty.Render(strings.Repeat(" ", l.rightW)))
			}
			sb.WriteString(stSeparator.Render("│"))
			sb.WriteString(agent[i])
		}
		rows[i] = sb.String()
	}
	if a.gd.on {
		rows = append([]string{a.renderDiffHeader(a.w)}, rows...)
		rows = append(rows, a.renderDiffHelp(a.w))
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

	if a.sig != nil && a.comp == nil && a.hoverText == "" && a.sigVisible() {
		if cx, cy := a.cursorScreen(a.sig.tab, l); a.cursorShown(a.sig.tab, l, cx, cy) {
			box := a.sig.render(max(10, a.w-l.rightX))
			x, y := signaturePos(l, a.w, cx, cy, box)
			rows = overlay(rows, box, x, y)
		}
	}

	if a.card != nil && a.composer == nil {
		if box, x, y := a.cardBox(l); box != nil {
			rows = overlay(rows, box, x, y)
		}
	}
	if a.composer != nil {
		box, x, y := a.composerBox(l)
		rows = overlay(rows, box, x, y)
	}

	if a.pending == " " && !a.pal.open && !a.cmdOpen {
		rows = a.renderSpaceMenu(rows, l)
	}
	if a.pending == "space v" && !a.pal.open && !a.cmdOpen {
		rows = a.renderMenu(rows, l, i18n.T("app.reviewhint"), reviewItems)
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
	a.updateCaret(l)
	return strings.Join(rows, "\n")
}

func (a *App) editorOpts(t *tab, focused bool) editorOpts {
	o := editorOpts{
		focused:      focused && !a.findOpen && !a.cmdOpen,
		cursorColor:  curTheme.Cursor,
		cursorAt:     t.buf.Cur,
		relative:     a.cfg.Editor.LineNumber == "relative",
		indentGuides: a.cfg.Editor.IndentGuides,
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
		o.extraCursors = append(o.extraCursors, c)
		if r.HasAnchor && r.Anchor != r.Cur {
			o.extraSels = append(o.extraSels, [2]buffer.Pos{r.Start(), r.End()})
		}
	}
	return o
}

func (a *App) renderTabs(w int) string {
	if a.single {
		return a.renderSingleTitle(w)
	}
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
		st := stTabInactive
		if i == a.active {
			st = stTabActive
		}
		var s string
		switch {
		case t.transient:
			s = st.Render(" ") + st.Italic(true).Render(name) + st.Render(mark+" ")
		case i == a.active && t.dirty():
			s = stTabActive.Render(" "+name) + stTabDirty.Render(mark) + stTabActive.Render(" ")
		default:
			s = st.Render(label)
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

func findLabel() string { return i18n.T("app.findlabel") }

func (a *App) renderFind(w int) string {
	label := stFindLabel.Render(findLabel())
	hint := stFind.Render(i18n.T("app.findhint"))
	iw := w - lipgloss.Width(label) - lipgloss.Width(hint)
	a.findInput.SetWidth(max(1, iw-1))
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
		leftParts = append(leftParts, i18n.T("app.pane.tree"))
	case focusChanges:
		leftParts = append(leftParts, i18n.T("app.pane.changes"))
	case focusTerminal:
		leftParts = append(leftParts, i18n.T("app.pane.terminal"))
	case focusAgent:
		leftParts = append(leftParts, i18n.T("app.pane.suggest"))
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
	if l := a.layout(); l.agentW == 0 {
		if n := len(a.suggestions()); n > 0 {
			leftParts = append(leftParts, fmt.Sprintf(i18n.T("app.suggestions"), n))
		}
	}
	left := " " + strings.Join(leftParts, "  ")
	right := ""
	if t := a.cur(); t != nil && t.kind == tabText {
		b := t.buf
		indent := fmt.Sprintf(i18n.T("app.spaces"), len(b.IndentUnit()))
		if b.IndentUnit() == "\t" {
			indent = i18n.T("app.tab")
		}
		pos := fmt.Sprintf("%d:%d", b.Cur.Row+1, buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)+1)
		if b.HasExtra() {
			pos = fmt.Sprintf(i18n.T("app.cursors"), len(b.Extra())+1) + pos
		}
		if s := b.SelectedText(); s != "" {
			pos += fmt.Sprintf(i18n.T("app.selected"), len([]rune(s)))
		}
		lang := t.hl.Language()
		if st := a.lspStatus(); st != "" {
			lang += " [" + st + "]"
		}
		right = fmt.Sprintf("%s   %s   %s   ", pos, indent, lang)
	}
	if n := len(a.gd.rv.pending); n > 0 && a.prShown() {
		right = fmt.Sprintf(i18n.T("app.pendingcomments"), n) + right
	}
	right += versionLabel() + " "
	st := stStatus
	switch {
	case a.confirm != nil:
		st, right = stStatusError, "" // the question must not be cut off
		left += "   " + a.confirm.prompt
	case a.msg != "":
		if a.msgErr {
			st = stStatusError
		}
		left += "   " + a.msg
	case a.prShown() && a.gd.rv.sync.text() != "":
		left += "   " + a.gd.rv.sync.text()
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
	a.infoText(i18n.T("app.hovercopied"))
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

// renderSingleTitle is the bar above the editor in single-file mode: the name
// of the file being edited, with no tabs to switch between.
func (a *App) renderSingleTitle(w int) string {
	a.tabSpans = nil
	t := a.cur()
	if t == nil {
		return stTabBar.Render(strings.Repeat(" ", w))
	}
	mark := "  "
	if t.dirty() {
		mark = " ●"
	}
	s := stTabActive.Render(" " + t.title() + mark + " ")
	if n := ansi.StringWidth(s); n < w {
		s += stTabBar.Render(strings.Repeat(" ", w-n))
	}
	return fitStyled(s, w, stTabBar)
}

// infoText shows an already-formatted message in the status line.
func (a *App) infoText(s string) {
	a.msg, a.msgErr = s, false
}
