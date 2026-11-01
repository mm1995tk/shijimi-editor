package tui

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
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
	sel     int             // index into files
	selDir  string          // path of the selected folder row; "" when a file is selected
	closed  map[string]bool // folder paths (see changeRow.path) that are collapsed
	scroll  int             // first row of rows shown
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

func (a *Core) gitTopDir() string {
	if a.gitTop != "" {
		return a.gitTop
	}
	return a.root
}

// reloadChanges recomputes the CHANGES list for the current base. The list
// on screen stays as it is until the result arrives; only showDiff marks it
// as loading.
func (a *Core) reloadChanges() []Effect {
	a.gd.gen++
	gen, root, base, pr := a.gd.gen, a.root, a.gd.base, a.gd.pr
	return async(func() Event { return loadChanges(gen, root, base, pr) })
}

func loadChanges(gen int, root, base string, pr *prInfo) Event {
	m := changesLoadedMsg{gen: gen, base: base}
	m.rev = emptyTree
	if out, err := run(root, nil, "git", "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		m.rev = strings.TrimSpace(string(out))
	}
	if base == basePR {
		if pr == nil {
			m.err = errString(i18n.T("gd.nopr"))
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
			for p := range strings.SplitSeq(string(names), "\x00") {
				if p != "" {
					m.stale[p] = true
				}
			}
		} else {
			m.staleAll = true // the head commit is not here (yet)
		}
	}
	if untracked, err := run(root, nil, "git", "ls-files", "--others", "--exclude-standard", "--full-name", "-z", ":/"); err == nil {
		for p := range strings.SplitSeq(string(untracked), "\x00") {
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

func (a *Core) applyChanges(m changesLoadedMsg) {
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
	sel, selDir := "", a.gd.selDir
	if f := a.gd.file(); f != nil {
		sel = f.Path()
	}
	a.gd.files, a.gd.rows = changeTree(m.files)
	if !a.gd.hasFolder(selDir) {
		selDir = ""
	}
	a.gd.selDir = selDir
	a.gd.byAbs = map[string]*diff.File{}
	top := a.gitTopDir()
	for i, f := range a.gd.files {
		a.gd.byAbs[filepath.Join(top, f.Path())] = f
		if f.Path() == sel && selDir == "" {
			a.gd.sel = i
		}
	}
	a.gd.sel = clamp(a.gd.sel, 0, max(0, len(a.gd.files)-1))
	// a selection that fell into a collapsed folder moves to the folder
	if vis := a.gd.vis(); len(vis) > 0 {
		a.gd.selectRow(vis[visPos(vis, a.gd.selRow())])
	}
}

func (g *gitDiff) file() *diff.File {
	if g.selDir != "" || g.sel < 0 || g.sel >= len(g.files) {
		return nil
	}
	return g.files[g.sel]
}

// ---- the CHANGES tree ----

// changeRow is one row of the CHANGES tree: a folder, or a changed file.
type changeRow struct {
	depth int
	name  string // folder path (single-child folders joined) or file name
	path  string // folder path from the repository top; "" for a file
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
	var walk func(n *node, depth int, parent string)
	walk = func(n *node, depth int, parent string) {
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
			path := parent + name
			rows = append(rows, changeRow{depth: depth, name: name, path: path, file: -1})
			walk(c, depth+1, path+"/")
		}
		sort.SliceStable(n.files, func(i, j int) bool { return n.files[i].Path() < n.files[j].Path() })
		for _, f := range n.files {
			rows = append(rows, changeRow{depth: depth, name: filepath.Base(f.Path()), file: len(ordered)})
			ordered = append(ordered, f)
		}
	}
	walk(root, 0, "")
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

// hasFolder reports whether a folder row has the path.
func (g *gitDiff) hasFolder(path string) bool {
	return path != "" && g.folderRow(path) >= 0
}

func (g *gitDiff) folderRow(path string) int {
	for r, row := range g.rows {
		if row.file < 0 && row.path == path {
			return r
		}
	}
	return -1
}

// vis returns the rows shown: all rows except those inside a collapsed
// folder.
func (g *gitDiff) vis() []int {
	out := make([]int, 0, len(g.rows))
	hide := -1 // depth of the collapsed folder being skipped
	for r, row := range g.rows {
		if hide >= 0 {
			if row.depth > hide {
				continue
			}
			hide = -1
		}
		out = append(out, r)
		if row.file < 0 && g.closed[row.path] {
			hide = row.depth
		}
	}
	return out
}

// selRow is the row of the selection.
func (g *gitDiff) selRow() int {
	if g.selDir != "" {
		if r := g.folderRow(g.selDir); r >= 0 {
			return r
		}
	}
	return g.rowOf(g.sel)
}

// selectRow moves the selection onto row r.
func (g *gitDiff) selectRow(r int) {
	if row := g.rows[r]; row.file >= 0 {
		g.sel, g.selDir = row.file, ""
	} else {
		g.selDir = row.path
	}
}

// visPos is the position in vis of row r, or of the nearest shown row above.
func visPos(vis []int, r int) int {
	p := 0
	for i, v := range vis {
		if v <= r {
			p = i
		}
	}
	return p
}

// move moves the selection by d shown rows.
func (g *gitDiff) move(d int) {
	vis := g.vis()
	if len(vis) == 0 {
		return
	}
	g.selectRow(vis[clamp(visPos(vis, g.selRow())+d, 0, len(vis)-1)])
}

// setFolder opens or closes the folder at row r. When the selection was
// inside the folder just closed, it moves to the folder.
func (g *gitDiff) setFolder(r int, open bool) {
	if g.closed == nil {
		g.closed = map[string]bool{}
	}
	if open {
		delete(g.closed, g.rows[r].path)
		return
	}
	g.closed[g.rows[r].path] = true
	for i := r + 1; i < len(g.rows) && g.rows[i].depth > g.rows[r].depth; i++ {
		if i == g.selRow() {
			g.selectRow(r)
			return
		}
	}
}

// parentRow is the folder row that holds row r, or -1.
func (g *gitDiff) parentRow(r int) int {
	for i := r - 1; i >= 0; i-- {
		if g.rows[i].file < 0 && g.rows[i].depth < g.rows[r].depth {
			return i
		}
	}
	return -1
}

// keepSelVisible moves the selection onto a row inside the h rows shown from
// scroll, after the list was scrolled.
func (g *gitDiff) keepSelVisible(h int) {
	vis := g.vis()
	if len(vis) == 0 {
		return
	}
	p := visPos(vis, g.selRow())
	if p >= g.scroll && p < g.scroll+h {
		return
	}
	g.selectRow(vis[clamp(g.scroll+h-1, 0, len(vis)-1)])
	if p < g.scroll {
		g.selectRow(vis[clamp(g.scroll, 0, len(vis)-1)])
	}
}

// ---- mode switch ----

// openDiff handles Space G (base HEAD) and Space g (base PR): it opens diff
// mode, closes it when it already compares with base, or switches the base.
func (a *Core) openDiff(base string) []Effect {
	if a.single {
		return nil
	}
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
func (a *Core) diffWithHead() []Effect {
	if a.gd.on && a.gd.base == baseHead {
		return a.reloadChanges()
	}
	return a.showDiff(baseHead, nil)
}

// toggleGitDiff opens diff mode against HEAD, or closes it.
func (a *Core) toggleGitDiff() []Effect {
	if a.gd.on {
		return a.closeDiff()
	}
	return a.showDiff(baseHead, nil)
}

func (a *Core) showDiff(base string, pr *prInfo) []Effect {
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
	a.setSide(true)
	a.follow = true
	cmd := a.reloadChanges()
	a.gd.loading = true // just opened or switched: nothing trustworthy to show yet
	return cmd
}

func (a *Core) closeDiff() []Effect {
	a.gd.prSeq++
	a.gd.on = false
	a.card = nil
	if a.focus == focusChanges {
		a.focus = focusExplorer
	}
	a.follow = true
	a.infoText(i18n.T("gd.closed"))
	return nil
}

// branchPR opens the current branch's pull request: the only one at once,
// a list of them when there are several, or the list of every open pull
// request when there is none.
func (a *Core) branchPR() []Effect {
	a.infoText(i18n.T("gd.findingpr"))
	a.gd.prSeq++
	root, seq, branch := a.root, a.gd.prSeq, a.branch
	return async(func() Event {
		prs, err := branchPRs(root, branch)
		return branchPRsMsg{seq: seq, prs: prs, err: err}
	})
}

func (a *Core) handleBranchPRs(m branchPRsMsg) []Effect {
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
	a.infoText(i18n.T("gd.nobranchpr"))
	return listPRs(a.root)
}

// findPR looks up pull request num to compare with its base.
func (a *Core) findPR(num int) []Effect { return a.lookupPR(num, false) }

// refreshPR looks up the pull request shown again (r).
func (a *Core) refreshPR() []Effect { return a.lookupPR(a.gd.pr.Number, true) }

func (a *Core) lookupPR(num int, refresh bool) []Effect {
	a.infoText(i18n.T("gd.findingpr"))
	a.gd.prSeq++
	root, seq := a.root, a.gd.prSeq
	return async(func() Event {
		pr, err := viewPR(root, num)
		return prFoundMsg{seq: seq, num: num, refresh: refresh, pr: pr, err: err}
	})
}

func (a *Core) handlePRFound(m prFoundMsg) []Effect {
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
	proceed := func() []Effect {
		if !m.refresh && pr.HeadRefName != a.branch {
			return a.switchToPR(pr)
		}
		if g := a.gd; m.refresh && g.on && g.base == basePR && g.pr != nil && g.pr.Number == pr.Number {
			// r on the pull request already shown: keep the list on screen
			g.pr = pr
			return batch(a.reloadChanges(), a.loadReview())
		}
		return a.openPR(pr)
	}
	if rv := a.gd.rv; len(rv.pending) > 0 && rv.num != pr.Number {
		a.confirm = &yesNo{
			prompt: fmt.Sprintf(i18n.T("gd.discardcomments"), rv.num, len(rv.pending), pr.Number),
			yes:    proceed,
			no:     func() []Effect { a.info(i18n.T("gd.cancelopen"), pr.Number); return nil },
		}
		return nil
	}
	return proceed()
}

// openPR compares with the base of pr, whose branch is checked out, and
// loads its review.
func (a *Core) openPR(pr *prInfo) []Effect {
	return batch(a.showDiff(basePR, pr), a.loadReview())
}

// switchToPR checks out the branch of pr with `gh pr checkout`, first asking
// to stash uncommitted changes, then opens it.
func (a *Core) switchToPR(pr *prInfo) []Effect {
	if n := a.dirtyCount(); n > 0 {
		a.fail(fmt.Errorf(i18n.T("gd.saveswitch"), n, pr.Number))
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
		prompt: i18n.T("gd.askstash"),
		yes:    func() []Effect { return a.checkoutPR(pr, true) },
		no:     func() []Effect { a.info(i18n.T("gd.cancelswitch"), pr.Number); return nil },
	}
	return nil
}

// prCheckedOutMsg reports the end of switching to a pull request's branch.
type prCheckedOutMsg struct {
	seq int
	pr  *prInfo
	err error
}

func (a *Core) checkoutPR(pr *prInfo, stash bool) []Effect {
	a.info(i18n.T("gd.checkout"), pr.Number)
	a.gd.prSeq++
	root, seq := a.root, a.gd.prSeq
	return async(func() Event {
		head := func() string {
			out, _ := run(root, nil, "git", "rev-parse", "HEAD")
			return string(out)
		}
		before := head()
		if stash {
			msg := fmt.Sprintf(i18n.T("gd.stashmsg"), pr.Number)
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
				err = fmt.Errorf(i18n.T("gd.stashed"), err, pr.Number)
			}
		}
		return prCheckedOutMsg{seq: seq, pr: pr, err: err}
	})
}

func (a *Core) handlePRCheckedOut(m prCheckedOutMsg) []Effect {
	if m.seq != a.gd.prSeq {
		return nil
	}
	a.refreshBranch()
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	cmd := a.openPR(m.pr)
	a.info(i18n.T("gd.switched"), m.pr.Number, a.branch)
	return cmd
}

// refreshBranch rereads the branch name and what depends on the checkout.
func (a *Core) refreshBranch() {
	if out, err := run(a.root, nil, "git", "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		a.branch = strings.TrimSpace(string(out))
	}
	// the checkout rewrote files: reload the tabs without unsaved edits, even
	// when watching files is off
	for _, t := range a.tabs {
		if t.kind == tabText && !t.dirty() && t.buf.DiskChanged() {
			if err := t.reload(); err != nil {
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
func (a *Core) reloadPR() []Effect {
	s := a.gd.rv.sync
	if !(s.known && s.behind > 0 && s.ahead == 0) {
		return a.refreshPR() // also refreshes the title, state and comments
	}
	a.infoText(i18n.T("gd.pulling"))
	a.gd.prSeq++
	root, seq := a.root, a.gd.prSeq
	return async(func() Event {
		_, err := run(root, nil, "git", "pull", "--ff-only")
		return prPulledMsg{seq: seq, err: err}
	})
}

func (a *Core) handlePRPulled(m prPulledMsg) []Effect {
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
func (a *Core) prPicker() []Effect {
	a.infoText(i18n.T("gd.fetchingprs"))
	return listPRs(a.root)
}

func (a *Core) showPRList(m prListMsg) []Effect {
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	if len(m.prs) == 0 {
		a.infoText(i18n.T("gd.noopenprs"))
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
	return a.pal.pick(i18n.T("gd.prs"), items, func(i int) []Effect { return a.findPR(m.prs[i].Number) })
}

func (a *Core) toggleSplit() {
	a.gd.split = !a.gd.split
	a.follow = true
}

// gitDiffCommand handles ":gitdiff [on|off|split|unified]".
func (a *Core) gitDiffCommand(arg string) []Effect {
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
		a.fail(errString(i18n.T("gd.usage")))
	}
	return nil
}

// ---- opening files from the list ----

func (a *Core) openChange(f *diff.File) []Effect {
	abs := filepath.Join(a.gitTopDir(), f.Path())
	if f.Status == "deleted" {
		a.info(i18n.T("gd.deleted"), f.Path())
		return nil
	}
	n := len(a.tabs)
	if !a.openFile(abs) {
		return nil
	}
	if len(a.tabs) > n {
		// a file already open keeps where the cursor was
		a.showFirstChange(a.curText())
	}
	return a.setFocus(focusEditor)
}

// showFirstChange puts the cursor on the first change of t, with a few rows
// above it on screen. A change that only deletes lines puts the cursor on
// the line after it.
func (a *Core) showFirstChange(t *tab) {
	dv := a.diffFor(t)
	if dv == nil {
		return
	}
	first := slices.IndexFunc(dv.rows, func(r dispRow) bool { return r.changed })
	if first < 0 {
		return
	}
	row := len(dv.newIdx) - 1
	if j := slices.IndexFunc(dv.rows[first:], func(r dispRow) bool { return r.new >= 0 }); j >= 0 {
		row = dv.rows[first+j].new
	}
	b := t.buf
	b.Anchor = nil
	b.Cur = b.ClampPos(buffer.Pos{Row: row})
	b.ScrollRow = dv.scrollFor(first - 3)
	a.follow = true
}

func (a *Core) handleChangesKey(msg Key) []Effect {
	g := &a.gd
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
		g.move(-1)
	case "down", "j":
		g.move(1)
	case "home", "g":
		g.move(-len(g.rows))
	case "end", "G":
		g.move(len(g.rows))
	case "h", "left":
		if len(g.rows) == 0 {
			break
		}
		r := g.selRow()
		if g.selDir != "" && !g.closed[g.selDir] {
			g.setFolder(r, false)
		} else if p := g.parentRow(r); p >= 0 {
			g.selectRow(p)
		}
	case "l", "right":
		if g.selDir != "" {
			g.setFolder(g.selRow(), true)
		} else if f := g.file(); f != nil {
			return a.openChange(f)
		}
	case "enter", "o":
		if g.selDir != "" {
			g.setFolder(g.selRow(), g.closed[g.selDir])
		} else if f := g.file(); f != nil {
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
func (a *Core) bodyH() int {
	if a.gd.on {
		return max(0, a.screenH()-3)
	}
	return max(0, a.screenH()-1)
}

// explorerH is the height of the file tree: the user's share of the sidebar
// (7/10 by default) in diff mode, all of it otherwise. The tree and CHANGES
// each keep at least minSideSectionH rows.
func (a *Core) explorerH() int {
	if a.gd.on {
		body := a.bodyH()
		if a.explorerShare() <= 0 {
			return body * 7 / 10
		}
		return clamp(body*a.explorerShare()/100, minSideSectionH, max(minSideSectionH, body-minSideSectionH))
	}
	return a.bodyH()
}

func statusLetter(f *diff.File) (string, screen.Style) {
	switch f.Status {
	case "added":
		return "A", screen.StyleDiffAddSign
	case "deleted":
		return "D", screen.StyleDiffDelSign
	case "renamed":
		return "R", screen.StyleGitRenamed
	case "binary":
		return "B", screen.StyleLineNo
	}
	return "M", screen.StyleDirty
}

// baseChip is the short name of the base: DIFF for HEAD, PR for a pull
// request's base.
func (a *Core) baseChip() string {
	if a.gd.base == basePR {
		return "PR"
	}
	return "DIFF"
}

// fitChanges scrolls the list so that the selected row is in view, in a list
// whose body is body rows high. It is the part of drawing the list that
// changes state, so it runs before drawing (fit).
func (a *Core) fitChanges(body int) {
	g := &a.gd
	vis := g.vis()
	if p := visPos(vis, g.selRow()); len(g.rows) > 0 {
		if p < g.scroll {
			g.scroll = p
		}
		if p >= g.scroll+body {
			g.scroll = p - body + 1
		}
	}
	g.scroll = clamp(g.scroll, 0, max(0, len(vis)-body))
}

// changesScreen is the list of changed files as a screen.Changes, w columns
// wide and h rows high with the title, scrolled as fitChanges left it. It
// changes nothing.
func (a *Core) changesScreen(w, h int) screen.Changes {
	g := &a.gd
	title := fmt.Sprintf(" CHANGES (%d)", len(g.files))
	pr := a.prShown()
	if pr {
		n := g.rv.viewedCount(g.files)
		if long := title + fmt.Sprintf("  viewed %d/%d", n, len(g.files)); ansi.StringWidth(long+a.baseChip()+" ") <= w {
			title = long
		} else {
			title += fmt.Sprintf(" ✓%d/%d", n, len(g.files))
		}
	}
	vis := g.vis()
	ch := screen.Changes{Title: title, Chip: a.baseChip() + " ", Top: g.scroll, Total: len(vis)}
	if len(g.rows) == 0 {
		switch {
		case g.loading:
			ch.Empty = i18n.T("gd.loading")
		case g.err != "":
			ch.Empty = i18n.T("gd.error") + g.err
		default:
			ch.Empty = i18n.T("gd.nochanges")
		}
	}
	for i := range max(0, h-1) {
		idx := g.scroll + i
		if idx >= len(vis) {
			break
		}
		row := g.rows[vis[idx]]
		r := screen.ChangesRow{Depth: row.depth, Name: row.name}
		if row.file < 0 {
			r.Dir, r.Closed = true, g.closed[row.path]
			r.Selected = g.selDir == row.path
			ch.Rows = append(ch.Rows, r)
			continue
		}
		f := g.files[row.file]
		r.Selected = row.file == g.sel && g.selDir == ""
		r.Letter, r.LetterStyle = statusLetter(f)
		r.Added, r.Deleted = f.Added, f.Deleted
		if pr {
			r.Viewed = g.rv.viewed[f.Path()]
			if a.isStale(f.Path()) {
				r.Name += " ✎"
			}
			r.Threads = len(g.rv.threadsIn(f.Path()))
		}
		ch.Rows = append(ch.Rows, r)
	}
	return ch
}

// ---- header and help lines ----

// compareBase is the branch checked out and the pull request that diff mode
// compares with (nil when it compares with HEAD).
func (a *Core) compareBase() (branch string, pr *prInfo) {
	if a.gd.base == basePR {
		pr = a.gd.pr
	}
	return a.branch, pr
}

// diffHelp lists the keys usable in diff mode right now.
func (a *Core) diffHelp() string {
	bases := i18n.T("gd.bar.pr")
	if a.gd.base == basePR {
		bases = i18n.T("gd.bar.prreview")
	}
	if !a.helix && a.focus != focusChanges {
		// the VS Code keymap has no Space menu in the editor
		bases = i18n.T("gd.bar.head")
		if a.gd.base == basePR {
			bases = i18n.T("gd.bar.headvsc")
		}
	}
	if a.focus == focusChanges {
		return bases + i18n.T("gd.bar.list")
	}
	return bases + i18n.T("gd.bar.editable")
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

	single  bool // a new file shown in one pane, with no line marked
	ver     int
	split   bool
	valid   bool
	rows    []dispRow
	newIdx  []int  // display row of every buffer line
	changed []bool // buffer lines that are part of a change

	// changed parts of the lines paired up in a hunk, by line index
	oldSpans, newSpans map[int][]diff.Span
	inline             map[[2]string]inlinePair // reused across builds

	hunks []screen.Hunk // runs of rows of one kind of change, for the change bar
}

// diffFile returns the changed file shown by tab t in diff mode, or nil
// when t is shown as a plain editor.
func (a *Core) diffFile(t *tab) *diff.File {
	if !a.gd.on || t == nil || t.kind != tabText || t.buf.Path == "" {
		return nil
	}
	f := a.gd.byAbs[t.buf.Path]
	if f == nil || f.Status == "binary" || f.Status == "deleted" {
		return nil
	}
	return f
}

// diffSingle reports whether f is shown in one pane: a file that is not in
// the base (HEAD or a pull request's base) has nothing to put on the left.
func (a *Core) diffSingle(f *diff.File) bool {
	return f != nil && f.Status == "added"
}

// splitShown reports whether the current tab's diff has a base pane.
func (a *Core) splitShown() bool {
	return a.gd.split && !a.diffSingle(a.diffFile(a.curText()))
}

// diffFor returns the up-to-date diff view of t, or nil outside diff mode.
func (a *Core) diffFor(t *tab) *diffView {
	f := a.diffFile(t)
	if f == nil {
		return nil
	}
	if t.dv == nil || t.dv.rev != a.gd.rev || t.dv.single != a.diffSingle(f) {
		dv := &diffView{rev: a.gd.rev, single: a.diffSingle(f)}
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
	defer func() { dv.hunks = hunksOf(dv.rows) }()
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
	if dv.single {
		// the whole file is an addition: the bar shows it, the lines are not marked
		ctx(len(lines))
		for i := range dv.rows {
			dv.rows[i].hunk = hunkAdd
		}
		return
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
			for k := range dels {
				add(b.OldStart+k, -1)
			}
			for k := range adds {
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

// ensureDiffVisible scrolls so the cursor is on screen, counting the rows
// that exist only on the base side.
func (a *Core) ensureDiffVisible(t *tab, dv *diffView, w, h int) {
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

// barKinds returns the kind of change in every cell of a bar h cells high
// over total display rows ("" = none). A cell holding several kinds counts
// as a rewrite.
func barKinds(total, h int, hunks []screen.Hunk) []screen.MarkKind {
	kinds := make([]screen.MarkKind, h)
	for i := range kinds {
		lo, hi := barRange(i, h, total)
		for j := sort.Search(len(hunks), func(j int) bool { return hunks[j].End > lo }); j < len(hunks) && hunks[j].Start < hi; j++ {
			switch {
			case kinds[i] == "":
				kinds[i] = hunks[j].Kind
			case kinds[i] != hunks[j].Kind:
				kinds[i] = screen.MarkModified
			}
		}
	}
	return kinds
}

// hunksOf is the runs of rows of one kind of change, as the Pane tells them.
func hunksOf(rows []dispRow) []screen.Hunk {
	var out []screen.Hunk
	for i, r := range rows {
		if r.hunk == hunkNone {
			continue
		}
		k := hunkMark(r.hunk)
		if n := len(out); n > 0 && out[n-1].End == i && out[n-1].Kind == k {
			out[n-1].End++
			continue
		}
		out = append(out, screen.Hunk{Start: i, End: i + 1, Kind: k})
	}
	return out
}

func hunkMark(k hunkKind) screen.MarkKind {
	switch k {
	case hunkAdd:
		return screen.MarkAdded
	case hunkDel:
		return screen.MarkDeleted
	}
	return screen.MarkModified
}

// diffBarClick scrolls the diff view to the part of the file that the bar
// cell at row y stands for.
func (a *Core) diffBarClick(t *tab, dv *diffView, g dvGeom, y int) {
	if y < g.top || y >= g.top+g.h {
		return
	}
	lo, _ := barRange(y-g.top, g.h, len(dv.rows))
	t.buf.ScrollRow = dv.scrollFor(max(0, lo-g.h/2))
}

// ---- the diff Panes ----

// buildDiffPanes makes the Panes of the diff view of t that g lays out: the
// base and the buffer side by side when the diff is split (rows that have no
// line on one side are a Blank line there), otherwise one Pane with the
// deleted lines between the current ones. It changes nothing.
func (a *Core) buildDiffPanes(t *tab, dv *diffView, g dvGeom, o paneOpts) []screen.Pane {
	b := t.buf
	two := dv.split && !dv.single
	start := dv.start(b.ScrollRow)
	info := func(side screen.DiffSide) *screen.DiffInfo {
		return &screen.DiffInfo{Side: side, Top: start, Rows: len(dv.rows), Hunks: dv.hunks}
	}

	right := textPaneHead(t, g.rightW, g.h, o)
	right.Kind = screen.KindDiff
	right.Title = screen.Title{Text: a.rel(b.Path)}
	if f := a.diffFile(t); f != nil {
		right.Title.Added, right.Title.Deleted = f.Added, f.Deleted
	}
	right.Title.Note = a.reviewNote(t)
	right.Title.NoteWarn = a.prShown() && a.isStale(a.repoPath(b.Path))
	right.Diff = info(screen.SideUnified)
	var left screen.Pane
	if two {
		right.Diff.Side = screen.SideNew
		left = screen.Pane{
			ID: b.Path + ":base", Kind: screen.KindDiff,
			Title:    screen.Title{Text: dv.path, ReadOnly: true},
			Viewport: screen.Viewport{Rows: g.h, Cols: max(0, g.leftW-gutterWidthFor(len(dv.old))), Left: b.ScrollCol, Total: len(dv.old)},
			Diff:     info(screen.SideOld),
		}
	}

	colors := t.hl.Colors(b)
	unit := guideUnit(b, o)
	topOld := true
	for i := 0; i < g.h && start+i < len(dv.rows); i++ {
		r := dv.rows[start+i]
		if topOld && r.old >= 0 {
			left.Viewport.Top, topOld = r.old, false
		}
		switch {
		case r.new >= 0:
			ln := textLine(b, colors, r.new, o, unit)
			if r.changed {
				m := screen.Mark{Kind: hunkMark(r.hunk), Style: screen.StyleDiffAddSign}
				if m.Kind == screen.MarkModified {
					m.Style = screen.StyleDiffModSign
				}
				ln.Marks = append(ln.Marks, m)
				ln.Words = colsOf(dv.newSpans[r.new])
			}
			right.Lines = append(right.Lines, ln)
		case two:
			right.Lines = append(right.Lines, screen.Line{No: -1, Blank: true})
		default:
			right.Lines = append(right.Lines, dv.oldLine(r))
		}
		if two {
			if r.old >= 0 {
				left.Lines = append(left.Lines, dv.oldLine(r))
			} else {
				left.Lines = append(left.Lines, screen.Line{No: -1, Blank: true})
			}
		}
	}
	if two {
		return []screen.Pane{left, right}
	}
	return []screen.Pane{right}
}

// oldLine is the line of the base that row r shows, marked deleted when it
// is part of a change.
func (dv *diffView) oldLine(r dispRow) screen.Line {
	ln := screen.Line{No: r.old, Spans: lexSpans(dv.old[r.old], dv.lexer), Words: colsOf(dv.oldSpans[r.old])}
	if r.changed {
		ln.Marks = []screen.Mark{{Kind: screen.MarkDeleted, Style: screen.StyleDiffDelSign}}
	}
	return ln
}

func colsOf(spans []diff.Span) []screen.Cols {
	var out []screen.Cols
	for _, s := range spans {
		out = append(out, screen.Cols{Start: s.Start, End: s.End})
	}
	return out
}

// lexSpans splits text into runs of one syntax color. With no lexer (or when
// lexing fails) it is one run of the plain text color.
func lexSpans(text string, lex chroma.Lexer) []screen.Span {
	var spans []screen.Span
	if lex != nil {
		if it, err := lex.Tokenise(nil, text); err == nil {
			for tok := it(); tok != chroma.EOF; tok = it() {
				v := strings.TrimRight(tok.Value, "\n")
				if v == "" {
					continue
				}
				st := syntaxStyleOf(tokenColor(tok.Type))
				if n := len(spans); n > 0 && spans[n-1].Style == st {
					spans[n-1].Text += v
				} else {
					spans = append(spans, screen.Span{Text: v, Style: st})
				}
			}
			return spans
		}
	}
	return []screen.Span{{Text: text}}
}

// ---- rendering ----

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

// afterSave refreshes what depends on the files on disk.
func (a *Core) afterSave() []Effect {
	if a.gd.on {
		return a.reloadChanges()
	}
	return nil
}

func (a *Core) focusChangesList() []Effect {
	if !a.gd.on {
		return a.toggleGitDiff()
	}
	a.setSide(true)
	return a.setFocus(focusChanges)
}
