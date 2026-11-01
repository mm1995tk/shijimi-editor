package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/proxy"
)

// Focus is the part of the screen that takes the keys.
type Focus int

const (
	FocusEditor Focus = iota
	FocusExplorer
	FocusTerminal
	FocusChanges // the CHANGES list in diff mode
	FocusAgent   // the suggestion panel on the right
)

type span struct{ x0, x1 int }

// Core is the state that decides what is shown: the files, the cursor, the
// keys' state, the language servers. The terminal front end (internal/tui.App)
// keeps what only a terminal has: the size of the screen, which regions are
// shown, the terminal panel and the pane sizes the user picked.
type Core struct {
	FrontEnd // what draws it (internal/tui.App)

	Root      string
	GitTop    string
	Cfg       *config.Config
	keymap    helixKeymap
	lastIns   *insertRecord // what "." repeats
	recIns    *insertRecord // the insert being recorded, until Esc
	replaying bool          // "." is running; do not record
	host      Host          // the GUI of `shj serve`; nil in the TUI
	Lsp       *lspManager
	Focus     Focus
	Explorer  *Explorer
	treeTicks int // watch ticks since the tree was last reloaded
	proxy     *proxy.Manager
	Pal       *Palette
	wsSyms    []lsp.Symbol // rows of the project symbol picker
	wsClient  *lsp.Client  // server the project symbol picker asks
	Search    *SearchPanel
	Tabs      []*Tab
	jumps     []jumpLoc // positions to return to (Ctrl+O / Ctrl+I)
	jumpIdx   int       // where in jumps the user is; len(jumps) = at the newest
	Active    int
	Branch    string
	Gd        gitDiff
	Confirm   *YesNo    // a y/n question on the status line
	Card      *PrCard   // a review thread shown like a hover
	Composer  *composer // the editor for a review comment

	// pane sizes picked by the user (0 = automatic), see resize.go

	// Helix modal state
	Helix          bool
	Mode           EditMode
	Pending        string
	count          int
	lastSearch     string
	searchBackward bool

	// bottom command line (":" commands and "/" search)
	CmdOpen  bool
	cmdKind  string
	cmdInput InputField
	cmdFrom  buffer.Pos

	// VS Code-style find bar
	FindOpen   bool
	FindInput  InputField
	findOrigin buffer.Pos

	Comp        *CompletionPopup
	sig         *SignaturePopup
	sigSeq      int // bumped on every request and close; stale replies are dropped
	snip        *snippetSession
	HoverText   string
	HoverScroll int
	HoverAt     *buffer.Pos // the character a mouse-triggered popup hangs from (a place in the file, not on the screen); nil = at the cursor
	HoverWord   WordSpan    // word the pointer's pending or open hover belongs to
	HoverCell   ScreenPt    // last cell the pointer rested on without a button
	HoverSeq    int         // bumped on every input; stale dwell timers and replies are dropped
	Ctx         *CtxPreview

	// Claude Code IDE integration (see ide.go)
	ide       *ide.Server
	props     Proposals     // agent changes waiting for the user's decision
	propShown *ProposalView // the first of them as drawn (PropView)
	selTrack  SelectionTracker

	// suggestion panel and ACP agent (see suggest.go, agent.go)
	Ag *agentState
	Sg suggestState

	Clipboard    string
	Msg          string
	MsgErr       bool
	confirmQuit  bool
	confirmClose bool
	Follow       bool // scroll to the cursor on next render
	dragging     bool
	Hold         holdScroll
	HoldSeq      int  // tells the current hold-scroll tick chain from stale ones
	LineFlip     bool // line numbers shown the other way (\ held, or toggled with \)

	single  bool // 単体編集モード: 1 つのファイルだけを開き、閉じたら終了する
	PrevTab *Tab // the file shown before the current one (ga / Ctrl+Tab)
	// side by side editor panes (see window.go): the focused pane shows cur(), the
	// other one shows other, and pane is the side (0 = left) the focused one is on
	Split     bool
	Pane      int
	Other     *Tab
	otherPrev *Tab // PrevTab of the other pane
	seenTab   *Tab // the file shown when the last event was handled
	Commands  []Command
	Mermaid   map[string]*previewJob
	MdImages  map[string]*previewJob // images in the Markdown preview, by file path
	// PreviewGen counts finished jobs, so the preview is drawn again when one
	// finishes
	PreviewGen int
}

// New creates the core rooted at dir, optionally opening file. fe is the front
// end it is drawn by.
func New(fe FrontEnd, dir, file string) *Core {
	abs, _ := filepath.Abs(dir)
	cfg := config.Load(abs)
	i18n.SetLang(cfg.Editor.UILanguage)
	a := &Core{
		FrontEnd: fe,
		Root:     abs,
		Cfg:      cfg,
		Lsp:      newLSPManager(cfg, abs),
		Explorer: NewExplorer(abs),
		proxy:    proxy.NewManager(),
		Pal:      newPalette(),
		Search:   newSearchPanel(cfg.Search),
		Focus:    FocusExplorer,
		Helix:    cfg.Editor.Keymap != "vscode",
		Gd:       gitDiff{Split: true},
		Ag:       newAgentState(),
	}
	a.Explorer.Icons = cfg.Editor.FileIcons
	if cfg.Editor.Theme != "" {
		a.SetPalette(ThemeOrDefault(cfg.Editor.Theme))
	}
	a.FindInput = NewInputField(0)
	a.FindInput.Placeholder = i18n.T("app.find")
	a.cmdInput = NewInputField(0)
	a.Commands = defaultCommands()
	if out, err := run(abs, nil, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		a.Branch = strings.TrimSpace(string(out))
	}
	if out, err := run(abs, nil, "git", "rev-parse", "--show-toplevel"); err == nil {
		a.GitTop = strings.TrimSpace(string(out))
	}
	if file != "" && a.OpenFile(file) {
		a.Focus = FocusEditor
	}
	a.loadKeymap()
	return a
}

// NewSingle creates the core in single-file mode: only file is edited, with no
// file tree, language server, terminal, diff mode or agent panel, and the app
// ends when the file is closed. A file that does not exist opens as an empty
// buffer, which creates the file when saved. It is meant for $EDITOR.
func NewSingle(fe FrontEnd, file string) (*Core, error) {
	abs, _ := filepath.Abs(file)
	dir := filepath.Dir(abs)
	cfg := config.Load(dir)
	i18n.SetLang(cfg.Editor.UILanguage)
	a := &Core{
		FrontEnd: fe,
		Root:     dir,
		Cfg:      cfg,
		Lsp:      newLSPManager(cfg, dir),
		Explorer: &Explorer{root: &Node{Name: filepath.Base(dir), Path: dir, isDir: true, expanded: true, depth: -1}}, // not loaded: no tree is shown
		proxy:    proxy.NewManager(),
		Pal:      newPalette(),
		Search:   newSearchPanel(cfg.Search),
		Focus:    FocusEditor,
		Helix:    cfg.Editor.Keymap != "vscode",
		Gd:       gitDiff{Split: true},
		Ag:       newAgentState(),
		single:   true,
	}
	if cfg.Editor.Theme != "" {
		a.SetPalette(ThemeOrDefault(cfg.Editor.Theme))
	}
	a.FindInput = NewInputField(0)
	a.FindInput.Placeholder = i18n.T("app.find")
	a.cmdInput = NewInputField(0)
	// The command palette is not reachable in this mode; with no commands, it
	// cannot open the views single-file mode leaves out even if it were.
	if !a.OpenFile(abs) {
		msg := i18n.T("app.cantopen")
		if a.Msg != "" {
			msg = a.Msg
		}
		return nil, errors.New(msg)
	}
	a.loadKeymap()
	return a, nil
}

// typing reports whether the editor accepts typed text right now: VS Code
// mode, or Helix insert mode.
func (a *Core) typing() bool {
	return a.Focus == FocusEditor && a.CurText() != nil && (!a.Helix || a.Mode == ModeInsert)
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
func (a *Core) Close() {
	a.Lsp.stopAll()
	if a.Ag.Client != nil {
		a.stopAgent()
	}
	a.MuxClose()
	RemoveShellIntegration()
}

// ---- helpers ----

// Cur is the tab shown in the focused pane, or nil when there is none.
func (a *Core) Cur() *Tab {
	if a.Active < 0 || a.Active >= len(a.Tabs) {
		return nil
	}
	return a.Tabs[a.Active]
}

// CurText is the current tab when it holds text, or nil.
func (a *Core) CurText() *Tab {
	if t := a.Cur(); t != nil && t.Kind == TabText {
		return t
	}
	return nil
}

func (a *Core) withBuf(f func(b *buffer.Buffer)) {
	if t := a.CurText(); t != nil {
		f(t.Buf)
		a.Follow = true
	}
}

// Info shows a message on the status line.
func (a *Core) Info(format string, args ...any) {
	a.Msg, a.MsgErr = fmt.Sprintf(format, args...), false
}

// Fail shows err on the status line as an error.
func (a *Core) Fail(err error) {
	a.Msg, a.MsgErr = err.Error(), true
}

// SetFocus moves the keys to f (always the editor in single-file mode).
func (a *Core) SetFocus(f Focus) []Effect {
	if a.single {
		f = FocusEditor
	}
	a.Focus = f
	a.TermBlur()
	if f == FocusTerminal {
		return a.TermFocus()
	}
	return nil
}

// Rel is p relative to the project root.
func (a *Core) Rel(p string) string { return RelTo(a.Root, p) }

func (a *Core) addTab(t *Tab) {
	a.Tabs = append(a.Tabs, t)
	a.Active = len(a.Tabs) - 1
	a.Follow = true
	a.Mode = ModeNormal
}

// reveal selects path in the file tree, which single-file mode does not have.
func (a *Core) reveal(path string) {
	if !a.single {
		a.Explorer.Reveal(path)
	}
}

// OpenFile shows the file at path, opening a tab for it unless one is open. It
// reports whether the file is shown.
func (a *Core) OpenFile(path string) bool {
	abs, _ := filepath.Abs(path)
	for i, t := range a.Tabs {
		if t.Path() == abs {
			a.Active = i
			t.Transient = false
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
			a.addTab(&Tab{Kind: TabImage, Img: NewImageView(abs)})
			a.reveal(abs)
			return true
		}
		if st.Size() > 10<<20 {
			a.Fail(fmt.Errorf(i18n.T("app.toolarge"), a.Rel(abs)))
			return false
		}
	}
	b, err := buffer.Open(abs)
	if err != nil {
		a.Fail(err)
		return false
	}
	t := NewTab(b)
	a.addTab(t)
	a.attachLSP(t)
	a.reveal(abs)
	return true
}

// openTransient opens path as the transient tab, which takes the place of the
// previous transient tab. A file already open is just shown as it is.
func (a *Core) openTransient(path string) bool {
	abs, _ := filepath.Abs(path)
	for i, t := range a.Tabs {
		if t.Path() == abs {
			a.Active = i
			a.reveal(abs)
			return true
		}
	}
	old := slices.IndexFunc(a.Tabs, func(t *Tab) bool { return t.Transient })
	n := len(a.Tabs)
	if !a.OpenFile(abs) || len(a.Tabs) == n {
		return false
	}
	t := a.Tabs[a.Active]
	t.Transient = true
	if t.Kind == TabText {
		t.transientVer = t.Buf.Version
	}
	if old >= 0 {
		a.dropTab(old)
		a.Tabs = slices.Insert(a.Tabs[:len(a.Tabs)-1], old, t)
		a.Active = old
	}
	return true
}

// KeepTransients turns transient tabs that have been edited into normal tabs.
func (a *Core) KeepTransients() {
	for _, t := range a.Tabs {
		if t.Transient && t.Kind == TabText && t.Buf.Version != t.transientVer {
			t.Transient = false
		}
	}
}

func (a *Core) save() []Effect {
	t := a.CurText()
	if t == nil {
		return nil
	}
	if t.Buf.Path == "" {
		return a.saveAs()
	}
	return a.saveTab(t)
}

func (a *Core) saveTab(t *Tab) []Effect {
	if err := t.Buf.Save(); err != nil {
		a.Fail(err)
		return nil
	}
	cs, _ := a.Lsp.clientsFor(t)
	a.SyncLSP()
	for _, c := range cs {
		c.DidSave(t.Buf.Path)
	}
	a.Explorer.loadGitStatus()
	a.Info(i18n.T("app.saved"), a.Rel(t.Buf.Path))
	return a.afterSave()
}

func (a *Core) saveAll() []Effect {
	n := 0
	for _, t := range a.Tabs {
		if t.dirty() && t.Buf.Path != "" {
			a.saveTab(t)
			if a.MsgErr {
				return nil
			}
			n++
		}
	}
	a.Info(i18n.T("app.savedmany"), n)
	return nil
}

func (a *Core) saveAs() []Effect {
	t := a.CurText()
	if t == nil {
		return nil
	}
	initial := a.Explorer.targetDir() + string(filepath.Separator)
	if t.Buf.Path != "" {
		initial = t.Buf.Path
	}
	return a.Pal.Ask(i18n.T("app.saveas"), initial, func(v string) []Effect {
		return a.saveTo(t, v)
	})
}

func (a *Core) saveTo(t *Tab, v string) []Effect {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if !filepath.IsAbs(v) {
		v = filepath.Join(a.Root, v)
	}
	isNew := t.Buf.Path != v
	t.Buf.Path = v
	t.Hl = NewHighlighter(v)
	cmd := a.saveTab(t)
	if isNew {
		a.attachLSP(t)
	}
	a.Explorer.Refresh()
	a.Explorer.Reveal(v)
	return cmd
}

func (a *Core) newFile() []Effect {
	if a.single {
		return nil
	}
	a.addTab(NewTab(buffer.New()))
	return a.SetFocus(FocusEditor)
}

func (a *Core) newFileIn(dir string) []Effect {
	return a.Pal.Ask(i18n.T("app.newfile"), dir+string(filepath.Separator), func(v string) []Effect {
		v = strings.TrimSpace(v)
		if v == "" || strings.HasSuffix(v, string(filepath.Separator)) {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(a.Root, v)
		}
		if _, err := os.Stat(v); err != nil {
			if err := os.MkdirAll(filepath.Dir(v), 0o755); err != nil {
				a.Fail(err)
				return nil
			}
			if err := os.WriteFile(v, nil, 0o644); err != nil {
				a.Fail(err)
				return nil
			}
		}
		a.Explorer.Refresh()
		if a.OpenFile(v) {
			return a.SetFocus(FocusEditor)
		}
		return nil
	})
}

func (a *Core) newFolder() []Effect {
	return a.Pal.Ask(i18n.T("app.newfolder"), a.Explorer.targetDir()+string(filepath.Separator), func(v string) []Effect {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(a.Root, v)
		}
		if err := os.MkdirAll(v, 0o755); err != nil {
			a.Fail(err)
			return nil
		}
		a.Explorer.Refresh()
		a.Explorer.Reveal(v)
		return nil
	})
}

func (a *Core) renameFile() []Effect {
	n := a.Explorer.selected()
	if n == nil {
		return nil
	}
	old := n.Path
	return a.Pal.Ask(i18n.T("app.newname"), old, func(v string) []Effect {
		v = strings.TrimSpace(v)
		if v == "" || v == old {
			return nil
		}
		if !filepath.IsAbs(v) {
			v = filepath.Join(filepath.Dir(old), v)
		}
		if _, err := os.Stat(v); err == nil {
			a.Fail(fmt.Errorf(i18n.T("app.exists"), a.Rel(v)))
			return nil
		}
		if err := os.Rename(old, v); err != nil {
			a.Fail(err)
			return nil
		}
		for _, t := range a.Tabs {
			switch {
			case t.Kind == TabText && t.Buf.Path == old:
				t.Buf.Path = v
				t.Hl = NewHighlighter(v)
			case t.Kind == TabText && strings.HasPrefix(t.Buf.Path, old+string(filepath.Separator)):
				t.Buf.Path = v + t.Buf.Path[len(old):]
			case t.Kind == TabImage && t.Img.Path == old:
				t.Img.Path = v
			}
		}
		a.Explorer.Refresh()
		a.Explorer.Reveal(v)
		a.Info(i18n.T("app.renamed"), a.Rel(v))
		return nil
	})
}

func (a *Core) deleteFile() []Effect {
	n := a.Explorer.selected()
	if n == nil {
		return nil
	}
	target := n.Path
	return a.Pal.Ask(fmt.Sprintf(i18n.T("app.confirmdelete"), a.Rel(target)), "", func(v string) []Effect {
		if strings.TrimSpace(strings.ToLower(v)) != "y" {
			return nil
		}
		if err := os.RemoveAll(target); err != nil {
			a.Fail(err)
			return nil
		}
		a.Explorer.Refresh()
		a.Info(i18n.T("app.deleted"), a.Rel(target))
		return nil
	})
}

// CloseTab closes the current tab; force closes it without asking about unsaved
// changes.
func (a *Core) CloseTab(force bool) []Effect {
	t := a.Cur()
	if t == nil {
		return nil
	}
	if !force && t.dirty() && !a.confirmClose {
		a.confirmClose = true
		a.Msg, a.MsgErr = i18n.T("app.closeunsaved"), true
		return nil
	}
	a.confirmClose = false
	keep := a.Other
	a.dropTab(a.Active)
	switch {
	case a.Split && keep != nil:
		// the pane of the closed file goes, and the other file fills the area
		a.Split, a.Pane, a.Other, a.otherPrev = false, 0, nil, nil
		a.Active = a.tabIndex(keep)
	case a.Active >= len(a.Tabs):
		a.Active = len(a.Tabs) - 1
	}
	a.Comp = nil
	a.Mode = ModeNormal
	if a.single {
		return a.Quit()
	}
	if len(a.Tabs) == 0 && a.Focus == FocusEditor {
		a.Focus = FocusExplorer
	}
	return nil
}

// dropTab removes tab i and releases what it holds; the caller fixes active.
func (a *Core) dropTab(i int) {
	t := a.Tabs[i]
	if t.Kind == TabText && t.Lang != nil {
		a.detachLSP(t)
	}
	if t.Img != nil {
		t.Img.Close()
	}
	a.Tabs = append(a.Tabs[:i], a.Tabs[i+1:]...)
}

// SwitchLast shows the file shown before the current one (ga / Ctrl+Tab).
func (a *Core) SwitchLast() {
	i := slices.Index(a.Tabs, a.PrevTab)
	if a.PrevTab == nil || i < 0 || i == a.Active {
		a.InfoText(i18n.T("app.nolast"))
		return
	}
	a.pushJump()
	a.showInPane(i)
}

// TrackLast remembers the file that was current before the active one changed.
// A file that was closed in the meantime is not remembered.
func (a *Core) TrackLast() {
	cur := a.Cur()
	if cur != a.seenTab {
		if a.seenTab != nil && slices.Contains(a.Tabs, a.seenTab) {
			a.PrevTab = a.seenTab
		}
		a.seenTab = cur
	}
}

// ToggleSide shows or hides the sidebar.
func (a *Core) ToggleSide() {
	if a.single {
		return
	}
	a.SetSide(!a.SideShown())
	if !a.SideShown() && (a.Focus == FocusExplorer || a.Focus == FocusChanges) {
		a.Focus = FocusEditor
	}
}

// ToggleTerm shows or hides the terminal panel and moves the focus with it.
func (a *Core) ToggleTerm() []Effect {
	if a.single {
		return nil
	}
	if a.MuxOn() {
		return a.MuxToggle()
	}
	if a.TermShown() && a.Focus == FocusTerminal {
		a.SetTerm(false)
		return a.SetFocus(FocusEditor)
	}
	a.SetTerm(true)
	return a.SetFocus(FocusTerminal)
}

func (a *Core) dirtyCount() int {
	n := 0
	for _, t := range a.Tabs {
		if t.dirty() {
			n++
		}
	}
	return n
}

// Quit ends the program, asking first when files have unsaved changes.
func (a *Core) Quit() []Effect {
	if d := a.dirtyCount(); d > 0 && !a.confirmQuit {
		a.confirmQuit = true
		a.Msg, a.MsgErr = fmt.Sprintf(i18n.T("app.quitunsaved"), d), true
		return nil
	}
	a.TermCloseAll()
	a.MuxClose()
	a.proxy.StopAll()
	return a.exit()
}

// exit ends the program. From here on the View keeps asking the terminal for
// the same keyboard enhancements: when they change, Bubble Tea queries the
// terminal, and the reply (CSI ? flags u) can arrive after input is no longer
// read, so the shell would get it as typed text such as "3u".
func (a *Core) exit() []Effect { return []Effect{Quit{}} }

// OpenFind opens the find bar on the current file.
func (a *Core) OpenFind() []Effect {
	t := a.CurText()
	if t == nil {
		return nil
	}
	a.FindOpen = true
	a.Focus = FocusEditor
	a.findOrigin = t.Buf.Cur
	if sel := t.Buf.SelectedText(); sel != "" && !strings.Contains(sel, "\n") {
		a.FindInput.SetValue(sel)
	}
	a.FindInput.CursorEnd()
	a.FindInput.Focus()
	return nil
}

func (a *Core) closeFind() {
	a.FindOpen = false
	a.FindInput.Blur()
}

func (a *Core) findNext(from buffer.Pos, backward bool) {
	t := a.CurText()
	q := a.FindInput.Value()
	if t == nil || q == "" {
		return
	}
	p, ok := t.Buf.Find(q, from, backward)
	if !ok {
		a.Msg, a.MsgErr = i18n.T("app.notfound")+q, true
		return
	}
	a.Msg = ""
	t.Buf.SelectRange(p, buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(q))})
	a.Follow = true
}

func (a *Core) toggleComment() {
	if t := a.CurText(); t != nil {
		t.Buf.ToggleComment(commentPrefix(t.Buf.Path))
		a.Follow = true
	}
}

// CopyText puts s in the clipboard register and on the system clipboard (the
// GUI's under shj serve).
func (a *Core) CopyText(s string) {
	a.Clipboard = s
	if a.host != nil {
		a.host.WriteClipboard(s)
		return
	}
	WriteSystemClipboard(s)
	// Over SSH there is no local helper tool; the outer terminal can still
	// take the text through OSC 52.
	a.SendClipboard(s)
}

// activePath is the file of the current tab, or "" when it has none.
func (a *Core) activePath() string {
	if t := a.Cur(); t != nil {
		return t.Path()
	}
	return ""
}

// copyPath copies p to the clipboard (relative to the root unless abs) and
// shows what was copied.
func (a *Core) copyPath(p string, abs bool) {
	if p == "" {
		a.Fail(errors.New(i18n.T("app.nopathfile")))
		return
	}
	if abs {
		if ap, err := filepath.Abs(p); err == nil {
			p = ap
		}
	} else {
		p = a.Rel(p)
	}
	a.CopyText(p)
	a.Info(i18n.T("app.pathcopied"), p)
}

func (a *Core) pasteText() string {
	if s, ok := ReadSystemClipboard(); ok {
		return s
	}
	return a.Clipboard
}

// ---- update ----

// ScheduleAutoSave starts a timer for every named tab edited since the last
// call. The timer is ignored if the buffer changed again before it fires.
func (a *Core) ScheduleAutoSave() []Effect {
	if !a.Cfg.Editor.AutoSave {
		return nil
	}
	var effs []Effect
	for _, t := range a.Tabs {
		if !t.dirty() || t.Buf.Path == "" || t.AutoVer == t.Buf.Version {
			continue
		}
		t.AutoVer = t.Buf.Version
		effs = append(effs, StartTimer{After: a.Cfg.Editor.AutoSaveDelay, Fire: AutoSaveDue{tab: t, ver: t.Buf.Version}})
	}
	return effs
}

func (a *Core) handleAutoSave(m AutoSaveDue) []Effect {
	t := m.tab
	if !t.dirty() || t.Buf.Version != m.ver || t.Buf.Path == "" || !slices.Contains(a.Tabs, t) {
		return nil
	}
	if t.Buf.DiskChanged() {
		return nil // never overwrite another process's change silently
	}
	return a.saveTab(t)
}

// Update takes an Input or an Event (or one of the front end's notices: Resized,
// KeyReleased, ReleasesReported), brings the state up to date (transients,
// split, last tab, LSP, selection) and returns the Effects of the message and
// the ones every round owes (auto-save, preview, lexing).
func (a *Core) Update(msg any) []Effect {
	effs := a.handle(msg)
	a.KeepTransients()
	a.NormalizeSplit()
	a.TrackLast()
	a.SyncLSP()
	a.NotifySelection()
	return Batch(effs, a.ScheduleAutoSave(), a.SyncPreview(), a.LexTabs(), a.SemanticTabs())
}

func (a *Core) handle(msg any) []Effect {
	switch msg := msg.(type) {
	case Event:
		return a.HandleEvent(msg)
	case Resized:
		a.SetScreen(msg.W, msg.H)
		a.Follow = true
	case ReleasesReported:
		a.SetReleasesReported(msg.On)
	case KeyReleased:
		a.holdScrollRelease(msg.Key)
		a.lineFlipRelease(msg.Key)
	case Input:
		return a.handleInput(msg)
	}
	return nil
}

// handleInput takes an Input from whichever front end made it.
func (a *Core) handleInput(in Input) []Effect {
	switch msg := in.(type) {
	case Mouse:
		return a.HandleMouse(msg)
	case Key:
		a.Ag.Sel = AgentSel{}
		if a.Hold.Buf != nil && !a.Hold.continues(msg) {
			a.stopHoldScroll() // another key ends the scroll
		}
		a.lineFlipOtherKey(msg)
		a.HoverSeq++ // a pending mouse hover is stale once the user types
		a.HoverWord = WordSpan{}
		// Fast typing can deliver several runes in one message; Helix-style
		// commands need them one at a time.
		if utf8.RuneCountInString(msg.Text) > 1 {
			var cmds [][]Effect
			for _, r := range msg.Text {
				cmds = append(cmds, a.HandleKey(Key{Code: r, Text: string(r), Mod: msg.Mod}))
			}
			return Batch(cmds...)
		}
		return a.HandleKey(msg)
	case Paste:
		a.HoverSeq++
		a.HoverWord = WordSpan{}
		return a.HandlePaste(msg)
	case Composition:
		// Nothing uses a composition yet: a terminal never composes, and a GUI sends
		// the committed text as input.key (its text), so taking it here too would
		// type it twice. B10 decides what a composition does.
	}
	return nil
}

func (a *Core) handleLSPEvent(e lsp.Event) []Effect {
	switch {
	case e.Err != nil:
		a.Fail(e.Err)
	case e.Message != "":
		a.Info("[%s] %s", e.Server, FirstLine(e.Message))
	case e.Edit != nil:
		if len(e.Edit) > 0 {
			a.applyWorkspaceEdit(e.Edit)
		}
	case e.Document != "":
		if strings.HasPrefix(e.Document, "file:") {
			if a.OpenFile(lsp.Path(e.Document)) {
				return a.SetFocus(FocusEditor)
			}
			return nil
		}
		return a.openURL(e.Document)
	case e.URI != "":
		p := lsp.Path(e.URI)
		if a.Lsp.Diags[p] == nil {
			a.Lsp.Diags[p] = map[string][]lsp.Diagnostic{}
		}
		a.Lsp.Diags[p][e.Server] = e.Diagnostics
	}
	return nil
}

// HandleKey is the entry for every key press: it closes what a key closes (the
// pointer's popups, the signature help) and hands the key to whatever takes
// it: an open overlay, the command line, the resize mode, or the focused
// region.
func (a *Core) HandleKey(msg Key) []Effect {
	key := msg.String()
	a.Msg = ""
	if a.sig != nil && (key == "esc" || !a.sigVisible()) {
		a.closeSignature()
	}
	if a.HoverAt != nil {
		// A popup the pointer opened never takes keys: it closes whatever has the
		// focus, and the key goes on to it.
		a.HoverText, a.HoverAt = "", nil
	}
	if a.Card != nil && a.Card.ByPointer {
		a.Card = nil // the same for a comment card the pointer opened
	}
	if key != "ctrl+q" {
		a.confirmQuit = false
	}
	if key != "ctrl+w" && key != "q" && key != "esc" {
		a.confirmClose = false
	}

	if a.Search.Open {
		return a.HandleSearchKey(msg)
	}
	if a.Pal.Open {
		return a.handlePaletteKey(msg)
	}
	if a.Composer != nil {
		return a.HandleComposerKey(msg)
	}
	if a.Confirm != nil {
		return a.handleConfirmKey(key)
	}
	if a.props.Len() > 0 {
		return a.handleProposalKey(msg)
	}
	if a.Ctx != nil {
		return a.HandleContextKey(msg)
	}
	if a.CmdOpen {
		return a.handleCmdlineKey(msg)
	}
	if a.InResizeMode() {
		return a.HandleResizeKey(key)
	}
	if a.Card != nil {
		if cmd, handled := a.handleCardKey(msg); handled {
			return cmd
		}
	}
	if a.HoverText != "" {
		if d := hoverScrollKey(key); d != 0 {
			a.ScrollHover(d)
			return nil
		}
		if key == "y" {
			a.CopyHover()
			return nil
		}
		a.HoverText = ""
		if key == "esc" {
			return nil
		}
	}
	if a.Comp != nil {
		if cmd, handled := a.handleCompletionKey(msg); handled {
			return cmd
		}
	}
	if a.snip != nil {
		if cmd, handled := a.handleSnippetKey(msg); handled {
			return cmd
		}
	}

	if a.Focus == FocusTerminal {
		// The shell owns the keyboard; only these stay global. Ctrl+J is not
		// one of them: many terminals send Shift+Enter as LF, which arrives as
		// ctrl+j, and the program in the shell takes it as a newline.
		switch key {
		case "ctrl+q":
			return a.Quit()
		case "ctrl+`":
			return a.ToggleTerm()
		case "alt+i":
			return a.toggleAgentPanel()
		case "alt+r":
			return a.StartResize()
		case "f1":
			return a.Pal.Show(a.Root, ">")
		}
		a.TermKey(msg)
		return nil
	}

	insert := a.Helix && a.Mode == ModeInsert && a.Focus == FocusEditor
	if a.single && singleBlocked(key, a.typing()) {
		return nil
	}
	switch key {
	case "ctrl+q":
		return a.Quit()
	case "ctrl+s":
		return a.save()
	case "ctrl+p":
		return a.Pal.Show(a.Root, "")
	case "f1":
		return a.Pal.Show(a.Root, ">")
	case "ctrl+k":
		if !insert {
			return a.Pal.Show(a.Root, ">")
		}
	case "ctrl+b":
		a.ToggleSide()
		return nil
	case "ctrl+`":
		return a.ToggleTerm()
	case "ctrl+j", "ctrl+space":
		// Ctrl+Space is completion wherever the editor accepts typing: VS Code
		// mode, or Helix insert mode.
		if key == "ctrl+j" && a.MuxOn() {
			return a.MuxResult(a.MuxOpen())
		}
		typing := a.typing()
		if key == "ctrl+j" || !typing {
			return a.ToggleTerm()
		}
	case "ctrl+g":
		return a.OpenDiff(BaseHead)
	case "ctrl+e":
		if a.Focus == FocusExplorer && len(a.Tabs) > 0 {
			return a.SetFocus(FocusEditor)
		}
		a.SetSide(true)
		return a.SetFocus(FocusExplorer)
	case "ctrl+n":
		if !insert {
			return a.newFile()
		}
	case "ctrl+w":
		if !insert {
			return a.CloseTab(false)
		}
	case "ctrl+tab":
		a.SwitchLast()
		return nil
	case "ctrl+\\":
		return a.toggleEditorSplit()
	case "ctrl+1":
		return a.FocusPane(0)
	case "ctrl+2":
		return a.FocusPane(1)
	case "alt+a":
		return a.SendContext()
	case "alt+i":
		return a.toggleAgentPanel()
	case "alt+r":
		return a.StartResize()
	}

	switch a.Focus {
	case FocusExplorer:
		return a.HandleExplorerKey(msg)
	case FocusChanges:
		return a.handleChangesKey(msg)
	case FocusAgent:
		return a.handleAgentKey(msg)
	}
	t := a.Cur()
	if t == nil {
		if key == "esc" || key == "tab" || key == "space" {
			a.SetSide(true)
			return a.SetFocus(FocusExplorer)
		}
		if key == ":" {
			return a.openCmdline(":")
		}
		return nil
	}
	switch t.Kind {
	case TabImage:
		if a.Pending == " " {
			a.Pending = ""
			if key == "esc" {
				return nil
			}
			return a.SpaceMode(key)
		}
		if a.Pending == "space v" {
			a.Pending = ""
			return a.ReviewKey(key)
		}
		if a.Pending == "space w" {
			a.Pending = ""
			return a.windowKey(key)
		}
		switch key {
		case "q", "esc":
			return a.CloseTab(true)
		case ":":
			return a.openCmdline(":")
		case "space":
			a.Pending = " "
		}
		return nil
	}
	if a.FindOpen {
		return a.handleFindKey(msg)
	}
	if a.Helix {
		return a.HandleHelixKey(msg)
	}
	if t := a.CurText(); a.FullPreview(t) && !fullPreviewEditorKey(msg) {
		a.InfoText(i18n.T("app.fullpreview"))
		return nil
	}
	return a.handleEditorKey(msg)
}

func (a *Core) handleCompletionKey(msg Key) ([]Effect, bool) {
	c := a.Comp
	switch msg.String() {
	case "up", "ctrl+p", "shift+tab":
		c.Sel = (c.Sel - 1 + len(c.Items)) % len(c.Items)
		return nil, true
	case "down", "ctrl+n":
		c.Sel = (c.Sel + 1) % len(c.Items)
		return nil, true
	case "tab", "enter":
		a.acceptCompletion()
		return nil, true
	case "esc":
		a.Comp = nil
		return nil, !a.Helix
	}
	return nil, false
}

func (a *Core) handlePaletteKey(msg Key) []Effect {
	p := a.Pal
	switch msg.String() {
	case "esc", "ctrl+c":
		p.Close()
		return nil
	case "up", "ctrl+p", "shift+tab":
		if p.Sel > 0 {
			p.Sel--
		}
		return nil
	case "down", "ctrl+n", "tab":
		if p.Sel < len(p.items)-1 {
			p.Sel++
		}
		return nil
	case "enter":
		return a.submitPalette()
	}
	return a.paletteInput(msg)
}

// paletteInput passes a key or a paste to the palette's query.
func (a *Core) paletteInput(msg Input) []Effect {
	p := a.Pal
	old := p.Input.Value()
	a.lineInputUpdate(&p.Input, msg)
	p.Sel = 0
	p.filter(a.Commands)
	if p.picking && p.live && p.Input.Value() != old {
		return p.queryLater()
	}
	return nil
}

func (a *Core) submitPalette() []Effect {
	p := a.Pal
	v := p.Input.Value()
	p.Close()
	if p.picking {
		if p.Sel < len(p.items) && p.OnPick != nil {
			return p.OnPick(p.items[p.Sel].index)
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
		return a.ExecCommand(strings.TrimSpace(v[1:]))
	case strings.HasPrefix(v, ">"):
		if p.Sel < len(p.items) {
			return a.Commands[p.items[p.Sel].index].run(a)
		}
	default:
		if p.Sel < len(p.items) {
			f := p.files[p.items[p.Sel].index]
			a.pushJump()
			if a.OpenFile(filepath.Join(a.Root, f)) {
				return a.SetFocus(FocusEditor)
			}
		}
	}
	return nil
}

// HandleExplorerKey handles a key press while the file tree has the focus.
func (a *Core) HandleExplorerKey(msg Key) []Effect {
	e := a.Explorer
	key := msg.String()
	if a.Pending == "space v" {
		a.Pending = ""
		return a.ReviewKey(key)
	}
	if a.Pending == "space w" {
		a.Pending = ""
		return a.windowKey(key)
	}
	if a.Pending == " " {
		a.Pending = ""
		return a.SpaceMode(key)
	}
	switch key {
	case "up", "k":
		e.Sel = max(0, e.Sel-1)
	case "down", "j":
		e.Sel = min(len(e.Flat)-1, e.Sel+1)
	case "pgup", "ctrl+u":
		e.Sel = max(0, e.Sel-(a.ScreenH()-3)/2)
	case "pgdown", "ctrl+d":
		e.Sel = min(len(e.Flat)-1, e.Sel+(a.ScreenH()-3)/2)
	case "home", "g":
		e.Sel = 0
	case "end", "G":
		e.Sel = len(e.Flat) - 1
	case "enter", "l", "right", "o":
		n := e.selected()
		if n == nil {
			return nil
		}
		if n.isDir {
			if key == "enter" || key == "o" || !n.expanded {
				e.Toggle(n)
			}
			return nil
		}
		if key != "enter" && key != "o" {
			a.openTransient(n.Path)
		} else if a.OpenFile(n.Path) {
			return a.SetFocus(FocusEditor)
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
			a.copyPath(n.Path, key == "Y")
		}
	case "R":
		e.Refresh()
		a.InfoText(i18n.T("app.treerefreshed"))
	case "space":
		a.Pending = " "
	case ":":
		return a.openCmdline(":")
	case "tab":
		if a.Gd.On {
			return a.SetFocus(FocusChanges)
		}
		if len(a.Tabs) > 0 {
			return a.SetFocus(FocusEditor)
		}
	case "esc", "q":
		if len(a.Tabs) > 0 {
			return a.SetFocus(FocusEditor)
		}
	}
	return nil
}

func (a *Core) handleFindKey(msg Key) []Effect {
	t := a.CurText()
	if t == nil {
		a.closeFind()
		return nil
	}
	switch msg.String() {
	case "esc":
		a.closeFind()
		return nil
	case "enter", "down", "f3":
		a.findNext(t.Buf.Cur, false)
		return nil
	case "up", "shift+tab", "shift+f3":
		s, _, ok := t.Buf.Selection()
		if !ok {
			s = t.Buf.Cur
		}
		a.findNext(s, true)
		return nil
	case "ctrl+f":
		return nil
	}
	return a.findInputUpdate(msg)
}

// findInputUpdate passes a key or a paste to the find bar.
func (a *Core) findInputUpdate(msg Input) []Effect {
	before := a.FindInput.Value()
	a.lineInputUpdate(&a.FindInput, msg)
	if a.FindInput.Value() != before {
		a.findNext(a.findOrigin, false)
	}
	return nil
}

// handleEditorKey is the modeless (VS Code style) editing used by the
// vscode keymap and by Helix insert mode.
func (a *Core) handleEditorKey(msg Key) []Effect {
	t := a.CurText()
	if t != nil && t.Buf.HasExtra() && multiEditable(msg) {
		a.Comp = nil
		t.Buf.ForEachCursor(func() { a.editorKey(t, msg) })
		a.Follow = true
		return nil
	}
	return a.editorKey(t, msg)
}

// multiEditable reports whether msg is a movement or editing key that
// applies to every cursor.
func multiEditable(msg Key) bool {
	if Typed(msg) != "" {
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

func (a *Core) editorKey(t *Tab, msg Key) []Effect {
	if t == nil {
		return nil
	}
	b := t.Buf
	a.Follow = true
	page := a.PageSize()

	switch msg.String() {
	case "esc":
		b.ClearSelection()
		b.ClearExtra()
		a.Comp = nil
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
			a.CopyText(s)
		} else {
			a.CopyText(string(b.Lines[b.Cur.Row]) + "\n")
		}
		a.InfoText(i18n.T("app.copied"))
	case "ctrl+x":
		if s := b.SelectedText(); s != "" {
			a.CopyText(s)
			b.DeleteSelection()
		} else {
			a.CopyText(b.CutLine())
		}
	case "ctrl+v":
		if s := a.pasteText(); s != "" {
			b.InsertBlock(s)
		}
	case "ctrl+f":
		return a.OpenFind()
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
		a.Comp = nil
	default:
		if s := Typed(msg); s != "" {
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
func (a *Core) afterTyping(t *Tab, typed []rune) []Effect {
	if len(typed) != 1 {
		return nil
	}
	return Batch(a.signatureTyped(t, typed[0]), a.completionTyped(t, typed[0]))
}

func (a *Core) completionTyped(t *Tab, r rune) []Effect {
	if a.Comp != nil {
		if isIdent(r) {
			a.Comp.filter()
			if len(a.Comp.Items) == 0 {
				a.Comp = nil
			}
			return nil
		}
		a.Comp = nil
	}
	// Typing must not report a missing or starting server; only an explicit
	// request does.
	if a.Lsp.client(t, "completionProvider") == nil {
		return nil
	}
	cs, _ := a.Lsp.clientsFor(t)
	for _, c := range cs {
		if c.IsCompletionTrigger(r) {
			return a.completion()
		}
	}
	if isIdent(r) {
		ws := wordStart(t.Buf)
		if t.Buf.Cur.Col-ws.Col == 2 {
			return a.completion()
		}
	}
	return nil
}

func (a *Core) refreshCompletion() []Effect {
	if a.Comp == nil {
		return nil
	}
	b := a.Comp.Tab.Buf
	if b.Cur.Less(a.Comp.start) || b.Cur.Row != a.Comp.start.Row {
		a.Comp = nil
		return nil
	}
	a.Comp.filter()
	if len(a.Comp.Items) == 0 {
		a.Comp = nil
	}
	return nil
}

// EditorOpts are the options a text pane of t is drawn with; focused says
// whether its pane has the focus.
func (a *Core) EditorOpts(t *Tab, focused bool) EditorOpts {
	o := EditorOpts{
		Focused:      focused && !a.FindOpen && !a.CmdOpen,
		CursorColor:  CurTheme.Cursor,
		cursorAt:     t.Buf.Cur,
		Relative:     a.relativeLineNumbers(),
		IndentGuides: a.Cfg.Editor.IndentGuides,
		title:        a.TitleOf(t),
		diags:        a.Lsp.diagnostics(t.Buf.Path),
	}
	if a.FindOpen {
		o.Query = a.FindInput.Value()
	}
	if a.CmdOpen && (a.cmdKind == "/" || a.cmdKind == "?") {
		o.Query = a.cmdInput.Value()
	}
	if a.Helix {
		switch a.Mode {
		case ModeInsert:
			o.CursorColor = CurTheme.ModeInsert
		case ModeSelect:
			o.CursorColor = CurTheme.ModeSelect
		}
		// Helix draws the cursor on the last selected character
		o.cursorAt = a.cursorCell(t.Buf, t.Buf.Anchor, t.Buf.Cur)
	}
	for _, r := range t.Buf.Extra() {
		var anchor *buffer.Pos
		if r.HasAnchor {
			anchor = &r.Anchor
		}
		c := r.Cur
		if a.Helix {
			c = a.cursorCell(t.Buf, anchor, r.Cur)
		}
		sel := [2]buffer.Pos{c, c}
		if r.HasAnchor && r.Anchor != r.Cur {
			sel = [2]buffer.Pos{r.Start(), r.End()}
		}
		o.extras = append(o.extras, extraCursor{cursor: c, sel: sel})
	}
	return o
}

// FindLabel is the label of the find bar.
func FindLabel() string { return i18n.T("app.findlabel") }

// CopyHover copies the hover markdown source to the clipboard.
func (a *Core) CopyHover() {
	a.CopyText(a.HoverText)
	a.InfoText(i18n.T("app.hovercopied"))
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
func (a *Core) overlayOpen() bool { return a.Pal.Open || a.Search.Open }

// InfoText shows an already-formatted message in the status line.
func (a *Core) InfoText(s string) {
	a.Msg, a.MsgErr = s, false
}
