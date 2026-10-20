package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/diff"
)

// Diff mode: the whole editor switches to showing changes against a base,
// HEAD (Space G) or the base of a pull request (Space g). A header line names
// the base, the sidebar gains a CHANGES tree below the file tree, a changed
// file opens as a full-file diff whose right side is the live (editable)
// buffer, and a help line lists the keys.

// emptyTree is the object id of git's empty tree, used as the base of a
// repository without commits.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

const (
	baseHead = "HEAD"
	basePR   = "PR"
)

// gitDiff is the state of diff mode.
type gitDiff struct {
	on      bool
	base    string  // baseHead or basePR
	rev     string  // revision the working tree is compared with
	pr      *prInfo // the pull request whose base is compared with (basePR)
	split   bool
	files   []*diff.File // in the order of the CHANGES tree
	rows    []changeRow
	byAbs   map[string]*diff.File
	sel     int // index into files
	scroll  int // first row of rows shown
	loading bool
	err     string
	gen     int
	ticks   int
	prSeq   int // bumped when the mode changes; older PR lookups are dropped

	// files (paths from the repository top) whose working tree differs from
	// the pull request's head commit; staleAll when that commit is missing
	stale    map[string]bool
	staleAll bool
	rv       prReview
}

type changesLoadedMsg struct {
	gen      int
	base     string
	rev      string
	pr       *prInfo
	files    []*diff.File
	stale    map[string]bool
	staleAll bool
	err      error
}

// prFoundMsg reports the pull request looked up to compare with. num is the
// number asked for, 0 for the current branch's. refresh is set for r, which
// reloads the pull request shown without switching branches.
type prFoundMsg struct {
	seq     int
	num     int
	refresh bool
	pr      *prInfo
	err     error
}

// branchPRsMsg reports the open pull requests of the current branch.
type branchPRsMsg struct {
	seq int
	prs []prSummary
	err error
}

// ---- loading ----

func (a *App) gitTopDir() string {
	if a.gitTop != "" {
		return a.gitTop
	}
	return a.root
}

// reloadChanges recomputes the CHANGES list for the current base.
func (a *App) reloadChanges() tea.Cmd {
	a.gd.gen++
	a.gd.loading = true
	gen, root, base, pr := a.gd.gen, a.root, a.gd.base, a.gd.pr
	return func() tea.Msg { return loadChanges(gen, root, base, pr) }
}

func loadChanges(gen int, root, base string, pr *prInfo) tea.Msg {
	m := changesLoadedMsg{gen: gen, base: base}
	m.rev = emptyTree
	if out, err := run(root, nil, "git", "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		m.rev = strings.TrimSpace(string(out))
	}
	if base == basePR {
		if pr == nil {
			m.err = errString("プルリクエストが選ばれていません")
			return m
		}
		ref := "origin/" + pr.BaseRefName
		if _, err := run(root, nil, "git", "rev-parse", "--verify", "-q", ref); err != nil {
			ref = pr.BaseRefName
		}
		mb, err := run(root, nil, "git", "merge-base", ref, "HEAD")
		if err != nil {
			m.err = err
			return m
		}
		m.pr, m.rev = pr, strings.TrimSpace(string(mb))
	}
	out, err := run(root, nil, "git", "diff", m.rev, "--no-color", "--no-ext-diff", "-M")
	if err != nil {
		m.err = err
		return m
	}
	files := diff.Parse(string(out))
	top, _ := run(root, nil, "git", "rev-parse", "--show-toplevel")
	topDir := strings.TrimSpace(string(top))
	if base == basePR {
		m.stale = map[string]bool{}
		if names, err := run(root, nil, "git", "diff", "--name-only", "--no-renames", "-z", pr.HeadRefOid); err == nil {
			for _, p := range strings.Split(string(names), "\x00") {
				if p != "" {
					m.stale[p] = true
				}
			}
		} else {
			m.staleAll = true // the head commit is not here (yet)
		}
	}
	if untracked, err := run(root, nil, "git", "ls-files", "--others", "--exclude-standard", "--full-name", "-z", ":/"); err == nil {
		for _, p := range strings.Split(string(untracked), "\x00") {
			if p == "" {
				continue
			}
			if f := untrackedFile(filepath.Join(topDir, p), p); f != nil {
				files = append(files, f)
				if m.stale != nil {
					m.stale[p] = true
				}
			}
		}
	}
	m.files = files
	return m
}

func (a *App) applyChanges(m changesLoadedMsg) {
	if m.gen != a.gd.gen {
		return
	}
	a.gd.loading = false
	if m.err != nil {
		a.gd.err = m.err.Error()
		if m.base == basePR {
			// fall back to HEAD so the mode stays usable
			a.gd.base, a.gd.pr = baseHead, nil
			a.fail(m.err)
			a.gd.err = ""
		}
		return
	}
	a.gd.err = ""
	a.gd.rev = m.rev
	if m.pr != nil {
		a.gd.pr = m.pr
	}
	a.gd.stale, a.gd.staleAll = m.stale, m.staleAll
	sel := ""
	if f := a.gd.file(); f != nil {
		sel = f.Path()
	}
	a.gd.files, a.gd.rows = changeTree(m.files)
	a.gd.byAbs = map[string]*diff.File{}
	top := a.gitTopDir()
	for i, f := range a.gd.files {
		a.gd.byAbs[filepath.Join(top, f.Path())] = f
		if f.Path() == sel {
			a.gd.sel = i
		}
	}
	a.gd.sel = clamp(a.gd.sel, 0, max(0, len(a.gd.files)-1))
}

func (g *gitDiff) file() *diff.File {
	if g.sel < 0 || g.sel >= len(g.files) {
		return nil
	}
	return g.files[g.sel]
}

// ---- the CHANGES tree ----

// changeRow is one row of the CHANGES tree: a folder, or a changed file.
type changeRow struct {
	depth int
	name  string // folder path (single-child folders joined) or file name
	file  int    // index into gitDiff.files; -1 for a folder
}

// changeTree orders files as a folder tree (folders first, then files, each
// sorted by name) and returns them with the rows that show the tree. A folder
// whose only child is a folder shares its row, as in "internal/config".
func changeTree(files []*diff.File) ([]*diff.File, []changeRow) {
	type node struct {
		dirs  map[string]*node
		files []*diff.File
	}
	newNode := func() *node { return &node{dirs: map[string]*node{}} }
	root := newNode()
	for _, f := range files {
		n := root
		parts := strings.Split(f.Path(), "/")
		for _, d := range parts[:len(parts)-1] {
			if n.dirs[d] == nil {
				n.dirs[d] = newNode()
			}
			n = n.dirs[d]
		}
		n.files = append(n.files, f)
	}
	var ordered []*diff.File
	var rows []changeRow
	var walk func(n *node, depth int)
	walk = func(n *node, depth int) {
		names := make([]string, 0, len(n.dirs))
		for d := range n.dirs {
			names = append(names, d)
		}
		sort.Strings(names)
		for _, d := range names {
			c, name := n.dirs[d], d
			for len(c.files) == 0 && len(c.dirs) == 1 {
				for sub, cc := range c.dirs {
					name, c = name+"/"+sub, cc
				}
			}
			rows = append(rows, changeRow{depth: depth, name: name, file: -1})
			walk(c, depth+1)
		}
		sort.SliceStable(n.files, func(i, j int) bool { return n.files[i].Path() < n.files[j].Path() })
		for _, f := range n.files {
			rows = append(rows, changeRow{depth: depth, name: filepath.Base(f.Path()), file: len(ordered)})
			ordered = append(ordered, f)
		}
	}
	walk(root, 0)
	return ordered, rows
}

// rowOf returns the row that shows file i.
func (g *gitDiff) rowOf(i int) int {
	for r, row := range g.rows {
		if row.file == i {
			return r
		}
	}
	return 0
}

// keepSelVisible moves the selection onto a file row inside the h rows shown
// from scroll, after the list was scrolled.
func (g *gitDiff) keepSelVisible(h int) {
	r := g.rowOf(g.sel)
	if r >= g.scroll && r < g.scroll+h {
		return
	}
	lo, hi := g.scroll, min(len(g.rows), g.scroll+h)
	if r >= hi {
		lo, hi = hi-1, lo-1
	}
	step := 1
	if lo > hi {
		step = -1
	}
	for i := lo; i != hi; i += step {
		if g.rows[i].file >= 0 {
			g.sel = g.rows[i].file
			return
		}
	}
}

// ---- mode switch ----

// openDiff handles Space G (base HEAD) and Space g (base PR): it opens diff
// mode, closes it when it already compares with base, or switches the base.
func (a *App) openDiff(base string) tea.Cmd {
	if a.gd.on && a.gd.base == base {
		return a.closeDiff()
	}
	if base == basePR {
		return a.branchPR()
	}
	return a.showDiff(baseHead, nil)
}

// diffWithHead opens diff mode against HEAD, or refreshes it when it is
// already showing HEAD.
func (a *App) diffWithHead() tea.Cmd {
	if a.gd.on && a.gd.base == baseHead {
		return a.reloadChanges()
	}
	return a.showDiff(baseHead, nil)
}

// toggleGitDiff opens diff mode against HEAD, or closes it.
func (a *App) toggleGitDiff() tea.Cmd {
	if a.gd.on {
		return a.closeDiff()
	}
	return a.showDiff(baseHead, nil)
}

func (a *App) showDiff(base string, pr *prInfo) tea.Cmd {
	a.gd.prSeq++
	a.gd.on = true
	a.card = nil
	if pr != nil && a.gd.rv.num != pr.Number {
		// another pull request: what was read or written for the last one goes.
		// Comparing with HEAD keeps it for coming back.
		a.gd.rv = prReview{num: pr.Number}
	}
	a.gd.base, a.gd.pr = base, pr
	a.gd.stale, a.gd.staleAll = nil, false
	a.showSide = true
	a.follow = true
	return a.reloadChanges()
}

func (a *App) closeDiff() tea.Cmd {
	a.gd.prSeq++
	a.gd.on = false
	a.card = nil
	if a.focus == focusChanges {
		a.focus = focusExplorer
	}
	a.follow = true
	a.info("差分モードを終了しました")
	return nil
}

// branchPR opens the current branch's pull request: the only one at once,
// a list of them when there are several, or the list of every open pull
// request when there is none.
func (a *App) branchPR() tea.Cmd {
	a.info("プルリクエストを探しています…")
	a.gd.prSeq++
	root, seq, branch := a.root, a.gd.prSeq, a.branch
	return func() tea.Msg {
		prs, err := branchPRs(root, branch)
		return branchPRsMsg{seq: seq, prs: prs, err: err}
	}
}

func (a *App) handleBranchPRs(m branchPRsMsg) tea.Cmd {
	if m.seq != a.gd.prSeq {
		return nil
	}
	switch {
	case m.err != nil:
		a.fail(m.err)
		return nil
	case len(m.prs) == 1:
		return a.findPR(m.prs[0].Number)
	case len(m.prs) > 1:
		return a.showPRList(prListMsg{prs: m.prs})
	}
	a.info("このブランチのプルリクエストが見つかりません。一覧から選んでください")
	return listPRs(a.root)
}

// findPR looks up pull request num to compare with its base.
func (a *App) findPR(num int) tea.Cmd { return a.lookupPR(num, false) }

// refreshPR looks up the pull request shown again (r).
func (a *App) refreshPR() tea.Cmd { return a.lookupPR(a.gd.pr.Number, true) }

func (a *App) lookupPR(num int, refresh bool) tea.Cmd {
	a.info("プルリクエストを探しています…")
	a.gd.prSeq++
	root, seq := a.root, a.gd.prSeq
	return func() tea.Msg {
		pr, err := viewPR(root, num)
		return prFoundMsg{seq: seq, num: num, refresh: refresh, pr: pr, err: err}
	}
}

func (a *App) handlePRFound(m prFoundMsg) tea.Cmd {
	if m.seq != a.gd.prSeq {
		return nil // the mode was closed or switched meanwhile
	}
	if m.err != nil {
		a.fail(m.err)
		if g := a.gd; m.refresh && g.on && g.base == basePR && g.pr != nil && g.pr.Number == m.num {
			return a.reloadChanges() // r: the list still updates without gh
		}
		return nil
	}
	a.msg = ""
	pr := m.pr
	proceed := func() tea.Cmd {
		if !m.refresh && pr.HeadRefName != a.branch {
			return a.switchToPR(pr)
		}
		return a.openPR(pr)
	}
	if rv := a.gd.rv; len(rv.pending) > 0 && rv.num != pr.Number {
		a.confirm = &yesNo{
			prompt: fmt.Sprintf("PR #%d の未送信コメントが %d 件あります。破棄して PR #%d を開きますか？ (y/n)", rv.num, len(rv.pending), pr.Number),
			yes:    proceed,
			no:     func() tea.Cmd { a.info("PR #%d を開くのをやめました", pr.Number); return nil },
		}
		return nil
	}
	return proceed()
}

// openPR compares with the base of pr, whose branch is checked out, and
// loads its review.
func (a *App) openPR(pr *prInfo) tea.Cmd {
	return tea.Batch(a.showDiff(basePR, pr), a.loadReview())
}

// switchToPR checks out the branch of pr with `gh pr checkout`, first asking
// to stash uncommitted changes, then opens it.
func (a *App) switchToPR(pr *prInfo) tea.Cmd {
	if n := a.dirtyCount(); n > 0 {
		a.fail(fmt.Errorf("未保存のファイルが %d 個あります。保存してから PR #%d に切り替えてください", n, pr.Number))
		return nil
	}
	out, err := run(a.root, nil, "git", "status", "--porcelain")
	if err != nil {
		a.fail(err)
		return nil
	}
	if strings.TrimSpace(string(out)) == "" {
		return a.checkoutPR(pr, false)
	}
	a.confirm = &yesNo{
		prompt: "未コミットの変更を stash して切り替えますか？ (y/n)",
		yes:    func() tea.Cmd { return a.checkoutPR(pr, true) },
		no:     func() tea.Cmd { a.info("PR #%d への切り替えを中止しました", pr.Number); return nil },
	}
	return nil
}

// prCheckedOutMsg reports the end of switching to a pull request's branch.
type prCheckedOutMsg struct {
	seq int
	pr  *prInfo
	err error
}

func (a *App) checkoutPR(pr *prInfo, stash bool) tea.Cmd {
	a.info("PR #%d をチェックアウトしています…", pr.Number)
	a.gd.prSeq++
	root, seq := a.root, a.gd.prSeq
	return func() tea.Msg {
		head := func() string {
			out, _ := run(root, nil, "git", "rev-parse", "HEAD")
			return string(out)
		}
		before := head()
		if stash {
			msg := fmt.Sprintf("shijimi: PR #%d に切り替える前の変更", pr.Number)
			if _, err := run(root, nil, "git", "stash", "push", "-u", "-m", msg); err != nil {
				return prCheckedOutMsg{seq: seq, pr: pr, err: err}
			}
		}
		_, err := run(root, nil, "gh", "pr", "checkout", fmt.Sprint(pr.Number))
		if err != nil && stash {
			// put the changes back where they were, if still there
			popped := false
			if head() == before {
				_, perr := run(root, nil, "git", "stash", "pop")
				popped = perr == nil
			}
			if !popped {
				err = fmt.Errorf("%w（変更は stash の「shijimi: PR #%d に切り替える前の変更」にあります）", err, pr.Number)
			}
		}
		return prCheckedOutMsg{seq: seq, pr: pr, err: err}
	}
}

func (a *App) handlePRCheckedOut(m prCheckedOutMsg) tea.Cmd {
	if m.seq != a.gd.prSeq {
		return nil
	}
	a.refreshBranch()
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	cmd := a.openPR(m.pr)
	a.info("PR #%d のブランチ %s に切り替えました", m.pr.Number, a.branch)
	return cmd
}

// refreshBranch rereads the branch name and what depends on the checkout.
func (a *App) refreshBranch() {
	if out, err := run(a.root, nil, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		a.branch = strings.TrimSpace(string(out))
	}
	// the checkout rewrote files: reload the tabs without unsaved edits, even
	// when watching files is off
	for _, t := range a.tabs {
		if t.kind == tabText && !t.dirty() && t.buf.DiskChanged() {
			if err := t.buf.Reload(); err != nil {
				a.fail(err)
			}
		}
	}
	a.explorer.refresh()
	a.explorer.loadGitStatus()
}

// prPulledMsg reports the end of `git pull` for r.
type prPulledMsg struct {
	seq int
	err error
}

// reloadPR handles r with a pull request shown: it pulls first when only
// the pull request has moved on.
func (a *App) reloadPR() tea.Cmd {
	s := a.gd.rv.sync
	if !(s.known && s.behind > 0 && s.ahead == 0) {
		return a.refreshPR() // also refreshes the title, state and comments
	}
	a.info("git pull しています…")
	a.gd.prSeq++
	root, seq := a.root, a.gd.prSeq
	return func() tea.Msg {
		_, err := run(root, nil, "git", "pull", "--ff-only")
		return prPulledMsg{seq: seq, err: err}
	}
}

func (a *App) handlePRPulled(m prPulledMsg) tea.Cmd {
	if m.seq != a.gd.prSeq {
		return nil
	}
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	a.refreshBranch()
	return a.refreshPR()
}

// prPicker lists the repository's pull requests to compare with.
func (a *App) prPicker() tea.Cmd {
	a.info("プルリクエストを取得しています…")
	return listPRs(a.root)
}

func (a *App) showPRList(m prListMsg) tea.Cmd {
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	if len(m.prs) == 0 {
		a.info("オープンなプルリクエストはありません")
		return nil
	}
	a.msg = ""
	items := make([]paletteItem, len(m.prs))
	for i, p := range m.prs {
		draft := ""
		if p.IsDraft {
			draft = " [draft]"
		}
		items[i] = paletteItem{label: fmt.Sprintf("#%d %s%s", p.Number, p.Title, draft), detail: "@" + p.Author.Login + "  " + p.HeadRefName}
	}
	return a.pal.pick("プルリクエスト", items, func(i int) tea.Cmd { return a.findPR(m.prs[i].Number) })
}

func (a *App) toggleSplit() {
	a.gd.split = !a.gd.split
	a.follow = true
}

// gitDiffCommand handles ":gitdiff [on|off|split|unified]".
func (a *App) gitDiffCommand(arg string) tea.Cmd {
	f := strings.Fields(arg)
	if len(f) == 0 {
		return a.openDiff(baseHead)
	}
	switch f[0] {
	case "on":
		return a.diffWithHead()
	case "off":
		if a.gd.on {
			return a.closeDiff()
		}
	case "split":
		a.gd.split = true
		a.follow = true
	case "unified":
		a.gd.split = false
		a.follow = true
	default:
		a.fail(errString("使い方: :gitdiff [on|off|split|unified]"))
	}
	return nil
}

// ---- opening files from the list ----

func (a *App) openChange(f *diff.File) tea.Cmd {
	abs := filepath.Join(a.gitTopDir(), f.Path())
	if f.Status == "deleted" {
		a.info("削除されたファイルです: %s", f.Path())
		return nil
	}
	if !a.openFile(abs) {
		return nil
	}
	return a.setFocus(focusEditor)
}

func (a *App) handleChangesKey(msg tea.KeyPressMsg) tea.Cmd {
	g := &a.gd
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
		g.sel = max(0, g.sel-1)
	case "down", "j":
		g.sel = min(max(0, len(g.files)-1), g.sel+1)
	case "home", "g":
		g.sel = 0
	case "end", "G":
		g.sel = max(0, len(g.files)-1)
	case "enter", "l", "right", "o":
		if f := g.file(); f != nil {
			return a.openChange(f)
		}
	case "s":
		a.toggleSplit()
	case "r", "R":
		if g.base == basePR && g.pr != nil {
			return a.reloadPR()
		}
		return a.reloadChanges()
	case "space":
		a.pending = " "
	case ":":
		return a.openCmdline(":")
	case "tab":
		return a.setFocus(focusExplorer)
	case "esc", "q":
		if len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
		return a.setFocus(focusExplorer)
	}
	return nil
}

// ---- sidebar ----

// bodyH is the number of rows between the diff mode header and the help and
// status lines.
func (a *App) bodyH() int {
	if a.gd.on {
		return max(0, a.h-3)
	}
	return max(0, a.h-1)
}

// explorerH is the height of the file tree: 7/10 of the sidebar in diff
// mode, all of it otherwise.
func (a *App) explorerH() int {
	if a.gd.on {
		return a.bodyH() * 7 / 10
	}
	return a.bodyH()
}

func statusLetter(f *diff.File) (string, string) {
	switch f.Status {
	case "added":
		return "A", cAddSign
	case "deleted":
		return "D", cDelSign
	case "renamed":
		return "R", curTheme.Renamed
	case "binary":
		return "B", cLineNo
	}
	return "M", cDirty
}

// baseChip is the short name of the base: DIFF for HEAD, PR for a pull
// request's base.
func (a *App) baseChip() string {
	if a.gd.base == basePR {
		return "PR"
	}
	return "DIFF"
}

func (a *App) renderChanges(w, h int, focused bool) []string {
	g := &a.gd
	out := make([]string, 0, h)
	title := fmt.Sprintf(" CHANGES (%d)", len(g.files))
	pr := a.prShown()
	chip := a.baseChip() + " "
	if pr {
		n := g.rv.viewedCount(g.files)
		if long := title + fmt.Sprintf("  viewed %d/%d", n, len(g.files)); ansi.StringWidth(long+chip) <= w {
			title = long
		} else {
			title += fmt.Sprintf(" ✓%d/%d", n, len(g.files))
		}
	}
	tw := max(0, w-ansi.StringWidth(chip))
	out = append(out, stSideTitle.Render(fit(title, tw))+bgfg(cSideBg, cAccent).Bold(true).Render(fit(chip, w-tw)))
	body := h - 1
	if r := g.rowOf(g.sel); len(g.files) > 0 {
		if r < g.scroll {
			g.scroll = r
		}
		if r >= g.scroll+body {
			g.scroll = r - body + 1
		}
	}
	g.scroll = clamp(g.scroll, 0, max(0, len(g.rows)-body))
	for i := 0; i < body; i++ {
		idx := g.scroll + i
		if idx >= len(g.rows) {
			msg := ""
			if i == 0 && len(g.rows) == 0 {
				switch {
				case g.loading:
					msg = " 読み込み中…"
				case g.err != "":
					msg = " エラー: " + g.err
				default:
					msg = " 変更はありません"
				}
			}
			out = append(out, stSide.Render(fit(msg, w)))
			continue
		}
		row := g.rows[idx]
		indent := " " + strings.Repeat("  ", row.depth)
		if row.file < 0 {
			out = append(out, bgfg(cSideBg, curTheme.SubSel).Render(fit(indent+"▾ "+row.name, w)))
			continue
		}
		f := g.files[row.file]
		bg := cSideBg
		if row.file == g.sel {
			bg = cListSelUnf
			if focused {
				bg = cListSel
			}
		}
		letter, col := statusLetter(f)
		add, del := fmt.Sprintf(" +%d", f.Added), fmt.Sprintf(" -%d ", f.Deleted)
		name, viewed, notes := row.name, " ", ""
		if pr {
			if g.rv.viewed[f.Path()] {
				viewed = "✓"
			}
			if a.isStale(f.Path()) {
				name += " ✎"
			}
			if n := len(g.rv.threadsIn(f.Path())); n > 0 {
				notes = fmt.Sprintf(" ●%d", n)
			}
		}
		nameW := max(0, w-ansi.StringWidth(indent)-2-ansi.StringWidth(notes+add+del))
		line := bgfg(bg, cAddSign).Render(viewed) + bgfg(bg, cSideFg).Render(indent[1:]) + bgfg(bg, col).Bold(true).Render(letter) +
			bgfg(bg, cSideFg).Render(" "+fit(name, nameW)) + bgfg(bg, curTheme.Renamed).Render(notes) +
			bgfg(bg, cAddSign).Render(add) + bgfg(bg, cDelSign).Render(del)
		out = append(out, fitStyled(line, w, bgfg(bg, cSideFg)))
	}
	return out
}

// ---- header and help lines ----

// renderDiffHeader draws the line above everything in diff mode: what is
// compared with what.
func (a *App) renderDiffHeader(w int) string {
	bg := cTabBarBg
	dim := bgfg(bg, curTheme.Dim)
	text := bgfg(bg, cEditorFg)
	strong := bgfg(bg, curTheme.Strong).Bold(true)
	var sb strings.Builder
	sb.WriteString(text.Render(" "))
	tree := "作業ツリー"
	if a.branch != "" {
		tree += "（" + a.branch + "）"
	}
	if pr := a.gd.pr; a.gd.base == basePR && pr != nil {
		sb.WriteString(bgfg(curTheme.Renamed, curTheme.CursorFg).Bold(true).Render(fmt.Sprintf(" PR #%d ", pr.Number)))
		sb.WriteString(strong.Render(" " + pr.Title))
		state, col := pr.State, cAddSign
		switch {
		case pr.IsDraft && pr.State == "OPEN":
			state, col = "DRAFT", curTheme.Dim
		case pr.State == "MERGED":
			col = curTheme.Renamed
		case pr.State == "CLOSED":
			col = cDelSign
		}
		sb.WriteString(text.Render("  ") + bgfg(bg, col).Bold(true).Render(state))
		if pr.Author.Login != "" {
			sb.WriteString(dim.Render("  @" + pr.Author.Login))
		}
		sb.WriteString(dim.Render("   比較元 ") + strong.Render(pr.BaseRefName) + text.Render("（PR の base）↔ "+tree))
	} else {
		sb.WriteString(bgfg(cAccent, curTheme.AccentFg).Bold(true).Render(" DIFF "))
		sb.WriteString(dim.Render("   比較元 ") + strong.Render("HEAD") + text.Render(" ↔ "+tree))
	}
	return fitStyled(sb.String(), w, text)
}

// diffHelp lists the keys usable in diff mode right now.
func (a *App) diffHelp() string {
	bases := " Space G:閉じる  Space g:PR の base と比べる"
	if a.gd.base == basePR {
		bases = " Space G:HEAD と比べる  Space g:閉じる  Space v:レビュー（c:コメント  C:ファイル  t:スレッド  v:viewed  A/R/S:送信）"
	}
	if !a.helix && a.focus != focusChanges {
		// the VS Code keymap has no Space menu in the editor
		bases = " Ctrl+G:閉じる"
		if a.gd.base == basePR {
			bases = " Ctrl+G:HEAD と比べる  F1 の「レビュー:」:コメント・送信"
		}
	}
	if a.focus == focusChanges {
		return bases + "  j/k:移動  Enter:開く  s:分割/統合  r:再読み込み  Tab:ファイルツリー  Esc:エディタへ"
	}
	return bases + "  右側は編集・保存できます  上の行のクリック:分割/統合  右端のバーのクリック:その位置へ"
}

func (a *App) renderDiffHelp(w int) string {
	return bgfg(cSideBg, curTheme.Dim).Render(fit(a.diffHelp(), w))
}

// ---- per-tab diff view ----

// hunkKind is the kind of change a display row belongs to.
type hunkKind byte

const (
	hunkNone hunkKind = iota
	hunkAdd           // lines only added
	hunkDel           // lines only deleted
	hunkMod           // lines on both sides
)

// dispRow is one screen row of a diff view. old/new are line indexes into the
// base and the buffer, or -1 when that side has no line on this row.
type dispRow struct {
	old, new int
	changed  bool
	hunk     hunkKind
}

// inlineBudget is how many bytes of changed line pairs one build compares
// afresh; pairs past it are marked as whole lines until a later build.
const inlineBudget = 256 << 10

// inlinePair is the result of comparing one changed line pair rune by rune.
type inlinePair struct {
	old, cur []diff.Span
	ok       bool
}

// diffView caches the comparison of one tab's buffer with its base.
type diffView struct {
	rev   string
	path  string // base path
	old   []string
	lexer chroma.Lexer

	ver     int
	split   bool
	valid   bool
	rows    []dispRow
	newIdx  []int  // display row of every buffer line
	changed []bool // buffer lines that are part of a change

	// changed parts of the lines paired up in a hunk, by line index
	oldSpans, newSpans map[int][]diff.Span
	inline             map[[2]string]inlinePair // reused across builds

	barH     int        // height barKinds was computed for
	barKinds []hunkKind // per cell of the change bar
}

// diffFile returns the changed file shown by tab t in diff mode, or nil
// when t is shown as a plain editor.
func (a *App) diffFile(t *tab) *diff.File {
	if !a.gd.on || t == nil || t.kind != tabText || t.buf.Path == "" {
		return nil
	}
	f := a.gd.byAbs[t.buf.Path]
	if f == nil || f.Status == "binary" || f.Status == "deleted" {
		return nil
	}
	return f
}

// diffFor returns the up-to-date diff view of t, or nil outside diff mode.
func (a *App) diffFor(t *tab) *diffView {
	f := a.diffFile(t)
	if f == nil {
		return nil
	}
	if t.dv == nil || t.dv.rev != a.gd.rev {
		dv := &diffView{rev: a.gd.rev}
		dv.path = f.OldPath
		if dv.path == "" {
			dv.path = f.Path()
		}
		if f.Status != "added" {
			if out, err := run(a.gitTopDir(), nil, "git", "show", a.gd.rev+":"+dv.path); err == nil {
				dv.old = splitText(string(out))
			}
		}
		if m := lexers.Match(filepath.Base(f.Path())); m != nil {
			dv.lexer = chroma.Coalesce(m)
		}
		t.dv = dv
	}
	dv := t.dv
	if !dv.valid || dv.ver != t.buf.Version || dv.split != a.gd.split {
		dv.build(t, a.gd.split)
	}
	return dv
}

func splitText(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func (dv *diffView) build(t *tab, split bool) {
	lines := make([]string, len(t.buf.Lines))
	for i, l := range t.buf.Lines {
		lines[i] = string(l)
	}
	dv.ver, dv.split, dv.valid = t.buf.Version, split, true
	dv.barH = 0
	dv.rows = dv.rows[:0]
	dv.newIdx = make([]int, len(lines))
	dv.changed = make([]bool, len(lines))
	dv.oldSpans, dv.newSpans = map[int][]diff.Span{}, map[int][]diff.Span{}
	prevInline := dv.inline
	dv.inline = map[[2]string]inlinePair{}
	budget := inlineBudget
	oi, ni := 0, 0
	ctx := func(n int) {
		for ; n > 0; n-- {
			dv.newIdx[ni] = len(dv.rows)
			dv.rows = append(dv.rows, dispRow{old: oi, new: ni})
			oi++
			ni++
		}
	}
	for _, b := range diff.Lines(dv.old, lines) {
		ctx(b.OldStart - oi)
		dels, adds := b.OldEnd-b.OldStart, b.NewEnd-b.NewStart
		kind := hunkMod
		switch {
		case dels == 0:
			kind = hunkAdd
		case adds == 0:
			kind = hunkDel
		}
		add := func(o, n int) {
			if n >= 0 {
				dv.newIdx[n] = len(dv.rows)
				dv.changed[n] = true
			}
			dv.rows = append(dv.rows, dispRow{old: o, new: n, changed: true, hunk: kind})
		}
		// the k-th deleted line pairs with the k-th added line, as split
		// view shows them side by side
		for k := 0; k < min(dels, adds); k++ {
			o, n := b.OldStart+k, b.NewStart+k
			key := [2]string{dv.old[o], lines[n]}
			p, seen := prevInline[key]
			if !seen {
				if budget -= len(key[0]) + len(key[1]); budget < 0 {
					continue // too much to compare; whole lines are marked
				}
				p.old, p.cur, p.ok = diff.Inline(key[0], key[1])
			}
			dv.inline[key] = p
			if p.ok && len(p.old) > 0 {
				dv.oldSpans[o] = p.old
			}
			if p.ok && len(p.cur) > 0 {
				dv.newSpans[n] = p.cur
			}
		}
		if split {
			for k := 0; k < max(dels, adds); k++ {
				o, n := -1, -1
				if k < dels {
					o = b.OldStart + k
				}
				if k < adds {
					n = b.NewStart + k
				}
				add(o, n)
			}
		} else {
			for k := 0; k < dels; k++ {
				add(b.OldStart+k, -1)
			}
			for k := 0; k < adds; k++ {
				add(-1, b.NewStart+k)
			}
		}
		oi, ni = b.OldEnd, b.NewEnd
	}
	ctx(len(lines) - ni)
}

// start is the display row at the top of the view for the buffer's scroll.
func (dv *diffView) start(scrollRow int) int {
	if scrollRow <= 0 || len(dv.newIdx) == 0 {
		return 0
	}
	return dv.newIdx[min(scrollRow, len(dv.newIdx)-1)]
}

// scrollFor is the buffer scroll that puts display row r at the top, or
// as close to it as the rows that only exist on the base side allow.
func (dv *diffView) scrollFor(r int) int {
	for i, idx := range dv.newIdx {
		if idx >= r {
			return i
		}
	}
	return max(0, len(dv.newIdx)-1)
}

// dvGeom is where the parts of a diff view sit on screen.
type dvGeom struct {
	top, h         int // first content row and number of content rows
	leftW          int // width of the base pane (0 when unified)
	rightX, rightW int // the editable pane
	barX           int // the change bar at the right edge
}

func (a *App) dvGeom(l layout) dvGeom {
	w := max(0, l.rightW-1) // the last column is the change bar
	g := dvGeom{top: l.editY + 1, h: max(0, l.editH-1), rightX: l.rightX, rightW: w, barX: l.rightX + w}
	if a.gd.split {
		g.leftW = (w - 1) / 2
		g.rightX = l.rightX + g.leftW + 1
		g.rightW = w - g.leftW - 1
	}
	return g
}

// ensureDiffVisible scrolls so the cursor is on screen, counting the rows
// that exist only on the base side.
func (a *App) ensureDiffVisible(t *tab, dv *diffView, w, h int) {
	b := t.buf
	ensureCursorVisible(b, w, h)
	if h <= 0 || len(dv.newIdx) == 0 {
		return
	}
	margin := min(3, (h-1)/2)
	cur := dv.newIdx[min(b.Cur.Row, len(dv.newIdx)-1)]
	for b.ScrollRow < b.Cur.Row && cur-dv.start(b.ScrollRow) >= h-margin {
		b.ScrollRow++
	}
}

// cursorScreen returns the screen position of the cursor cell.
func (a *App) cursorScreen(t *tab, l layout) (x, y int) {
	b := t.buf
	x = l.rightX + gutterWidth(b) + buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col) - b.ScrollCol
	y = l.editY + b.Cur.Row - b.ScrollRow
	if dv := a.diffFor(t); dv != nil {
		g := a.dvGeom(l)
		x += g.rightX - l.rightX
		y = g.top + dv.newIdx[min(b.Cur.Row, len(dv.newIdx)-1)] - dv.start(b.ScrollRow)
	}
	return x, y
}

// ---- the change bar ----

// barRange returns the display rows [lo, hi) that cell i of a bar h cells
// high stands for: the n rows of the whole file spread over the bar, one row
// per cell when they fit.
func barRange(i, h, n int) (lo, hi int) {
	if h <= 0 || n <= 0 {
		return 0, 0
	}
	if n <= h {
		return min(i, n), min(i+1, n)
	}
	lo, hi = i*n/h, (i+1)*n/h
	if hi <= lo {
		hi = lo + 1
	}
	return min(lo, n), min(hi, n)
}

// barCells returns the kind of change in every cell of a bar h cells high.
// A cell holding several kinds counts as a rewrite.
func (dv *diffView) barCells(h int) []hunkKind {
	if dv.barH == h && len(dv.barKinds) == h {
		return dv.barKinds
	}
	kinds := make([]hunkKind, h)
	for i := range kinds {
		lo, hi := barRange(i, h, len(dv.rows))
		for _, r := range dv.rows[lo:hi] {
			switch {
			case r.hunk == hunkNone:
			case kinds[i] == hunkNone:
				kinds[i] = r.hunk
			case kinds[i] != r.hunk:
				kinds[i] = hunkMod
			}
		}
	}
	dv.barH, dv.barKinds = h, kinds
	return kinds
}

// changeBar draws the bar: a mark per change, and a lighter background over
// the rows on screen, which start at display row start.
func (dv *diffView) changeBar(h, start int) []string {
	kinds := dv.barCells(h)
	out := make([]string, h)
	for i, k := range kinds {
		lo, hi := barRange(i, h, len(dv.rows))
		bg := cEditorBg
		if hi > start && lo < start+h {
			bg = cBarView
		}
		switch k {
		case hunkAdd:
			out[i] = bgfg(bg, cAddSign).Render("▐")
		case hunkDel:
			out[i] = bgfg(bg, cDelSign).Render("▐")
		case hunkMod:
			out[i] = bgfg(bg, cModSign).Render("▐")
		default:
			out[i] = bgfg(bg, cEditorFg).Render(" ")
		}
	}
	return out
}

// diffBarClick scrolls the diff view to the part of the file that the bar
// cell at row y stands for.
func (a *App) diffBarClick(t *tab, dv *diffView, g dvGeom, y int) {
	if y < g.top || y >= g.top+g.h {
		return
	}
	lo, _ := barRange(y-g.top, g.h, len(dv.rows))
	t.buf.ScrollRow = dv.scrollFor(max(0, lo-g.h/2))
}

// ---- rendering ----

func (a *App) renderDiffText(t *tab, dv *diffView, l layout, focused bool) []string {
	g := a.dvGeom(l)
	b := t.buf
	out := make([]string, 0, l.editH)

	hint := "[分割 ⇄ 統合]"
	f := a.diffFile(t)
	name := " " + a.rel(t.buf.Path)
	stats := fmt.Sprintf("  +%d", f.Added)
	dels := fmt.Sprintf(" -%d", f.Deleted)
	note, noteFg := a.reviewNote(t), curTheme.Renamed
	if a.prShown() && a.isStale(a.repoPath(t.buf.Path)) {
		noteFg = curTheme.DiagWarn
	}
	hw := ansi.StringWidth(hint) + 1
	nw := max(0, min(ansi.StringWidth(name), l.rightW-hw-ansi.StringWidth(stats+dels)))
	head := bgfg(cTabBarBg, curTheme.Strong).Bold(true).Render(ansi.Truncate(name, nw, "…")) +
		bgfg(cTabBarBg, cAddSign).Render(stats) + bgfg(cTabBarBg, cDelSign).Render(dels) +
		bgfg(cTabBarBg, noteFg).Render(note)
	out = append(out, fitStyled(head, max(0, l.rightW-hw), bgfg(cTabBarBg, curTheme.Strong))+
		bgfg(cTabBarBg, curTheme.Dim).Render(fit(" "+hint, hw)))

	opts := a.editorOpts(t, focused)
	opts.rowBg = func(row int) string {
		if row >= 0 && row < len(dv.changed) && dv.changed[row] {
			return cAddBg
		}
		return ""
	}
	opts.marks = func(row int) []diff.Span { return dv.newSpans[row] }
	opts.markBg = cAddWordBg
	opts.gutterMark = a.commentMark(t)
	gw := len(fmt.Sprint(len(dv.old))) + 4 // same width as gutterWidth

	start := dv.start(b.ScrollRow)
	bar := dv.changeBar(g.h, start)
	saved := b.ScrollRow
	defer func() { b.ScrollRow = saved }()
	for i := 0; i < g.h; {
		idx := start + i
		if idx >= len(dv.rows) {
			for ; i < g.h; i++ {
				out = append(out, a.diffBlank(g.rightX+g.rightW-l.rightX)+bar[i])
			}
			break
		}
		r := dv.rows[idx]
		if r.new < 0 {
			var line string
			if a.gd.split {
				line = a.diffOld(dv, r, gw, g.leftW) + stSeparator.Render("│") + bgfg(cSideBg, cLineNo).Render(fit("", g.rightW))
			} else {
				line = a.diffOldUnified(dv, r, gutterWidth(b), g.rightW)
			}
			out = append(out, line+bar[i])
			i++
			continue
		}
		// a run of rows that all show buffer lines
		n := 1
		for i+n < g.h && idx+n < len(dv.rows) && dv.rows[idx+n].new >= 0 {
			n++
		}
		b.ScrollRow = r.new
		edit := renderEditor(t, g.rightW, n, opts)
		for k := 0; k < n; k++ {
			if a.gd.split {
				out = append(out, a.diffOld(dv, dv.rows[idx+k], gw, g.leftW)+stSeparator.Render("│")+edit[k]+bar[i+k])
			} else {
				out = append(out, edit[k]+bar[i+k])
			}
		}
		i += n
	}
	return out
}

func (a *App) diffBlank(w int) string { return stEmpty.Render(fit("", w)) }

// diffOld draws the base side of a row in split mode.
func (a *App) diffOld(dv *diffView, r dispRow, gw, w int) string {
	if r.old < 0 {
		return bgfg(cSideBg, cLineNo).Render(fit("", w))
	}
	bg, sc := cEditorBg, cLineNo
	sign := " "
	var spans []diff.Span
	if r.changed {
		bg, sc, sign = cDelBg, cDelSign, "-"
		spans = dv.oldSpans[r.old]
	}
	gutter := bgfg(bg, sc).Render(sign) + bgfg(bg, cLineNo).Render(fmt.Sprintf("%*d  ", gw-3, r.old+1))
	return gutter + highlightText(dv.old[r.old], dv.lexer, bg, spans, cDelWordBg, w-gw)
}

// diffOldUnified draws a deleted line between the current lines.
func (a *App) diffOldUnified(dv *diffView, r dispRow, gw, w int) string {
	gutter := bgfg(cDelBg, cDelSign).Render("-") + bgfg(cDelBg, cLineNo).Render(fmt.Sprintf("%*d  ", gw-3, r.old+1))
	return gutter + highlightText(dv.old[r.old], dv.lexer, cDelBg, dv.oldSpans[r.old], cDelWordBg, w-gw)
}

// highlightText draws text with syntax colors on bg, w cells wide. The runes
// inside spans get markBg instead.
func highlightText(text string, lex chroma.Lexer, bg string, spans []diff.Span, markBg string, w int) string {
	if w <= 0 {
		return ""
	}
	var sb strings.Builder
	used, idx := 0, 0 // cells drawn, rune index into text
	var run strings.Builder
	var runKey cellStyle
	flush := func() {
		if run.Len() > 0 {
			sb.WriteString(styleFor(runKey).Render(run.String()))
			run.Reset()
		}
	}
	// emit draws s in fg and reports whether there is room for more.
	emit := func(s, fg string) bool {
		for _, r := range s {
			cell := bg
			for _, sp := range spans {
				if idx >= sp.Start && idx < sp.End {
					cell = markBg
					break
				}
			}
			idx++
			ch := string(r)
			if r == '\t' {
				ch = "    "
			}
			cw := ansi.StringWidth(ch)
			if used+cw > w {
				return false
			}
			if k := (cellStyle{fg: fg, bg: cell}); k != runKey {
				flush()
				runKey = k
			}
			run.WriteString(ch)
			used += cw
		}
		return used < w
	}
	drawn := false
	if lex != nil {
		if it, err := lex.Tokenise(nil, text); err == nil {
			drawn = true
			for tok := it(); tok != chroma.EOF; tok = it() {
				v := strings.TrimRight(tok.Value, "\n")
				if v == "" {
					continue
				}
				fg := tokenColor(tok.Type)
				if fg == "" {
					fg = cEditorFg
				}
				if !emit(v, fg) {
					break
				}
			}
		}
	}
	if !drawn {
		emit(text, cEditorFg)
	}
	flush()
	if used < w {
		sb.WriteString(styleFor(cellStyle{fg: cEditorFg, bg: bg}).Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}

var tokenColors = map[chroma.TokenType]string{} // cleared by applyTheme

func tokenColor(tt chroma.TokenType) string {
	if c, ok := tokenColors[tt]; ok {
		return c
	}
	c := ""
	if e := chromaStyle.Get(tt); e.Colour.IsSet() {
		c = e.Colour.String()
	}
	tokenColors[tt] = c
	return c
}

// diffMouseToCursor moves the cursor to the buffer line under (x, y) in a
// diff view. Rows that exist only on the base side snap to the next line.
func (a *App) diffMouseToCursor(t *tab, dv *diffView, l layout, x, y int, extend bool) {
	g := a.dvGeom(l)
	b := t.buf
	x = clamp(x, g.rightX, g.rightX+g.rightW-1)
	if y < g.top {
		b.MoveLines(-1, extend)
	} else if y >= g.top+g.h {
		b.MoveLines(1, extend)
	}
	idx := clamp(dv.start(b.ScrollRow)+clamp(y-g.top, 0, max(0, g.h-1)), 0, len(dv.rows)-1)
	row := -1
	for i := idx; i < len(dv.rows) && row < 0; i++ {
		row = dv.rows[i].new
	}
	for i := idx; i >= 0 && row < 0; i-- {
		row = dv.rows[i].new
	}
	if row < 0 {
		return
	}
	vis := max(0, x-g.rightX-gutterWidth(b)) + b.ScrollCol
	b.SetCursor(row, vis, extend)
	a.follow = true
}

// afterSave refreshes what depends on the files on disk.
func (a *App) afterSave() tea.Cmd {
	if a.gd.on {
		return a.reloadChanges()
	}
	return nil
}

func (a *App) focusChangesList() tea.Cmd {
	if !a.gd.on {
		return a.toggleGitDiff()
	}
	a.showSide = true
	return a.setFocus(focusChanges)
}
