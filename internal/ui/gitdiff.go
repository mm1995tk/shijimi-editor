package ui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/diff"
)

// git-diff mode: the whole editor switches to showing changes. The sidebar
// gains a CHANGES list below the file tree, and a changed file opens as a
// full-file diff whose right side is the live (editable) buffer.

// emptyTree is the object id of git's empty tree, used as the base of a
// repository without commits.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

const (
	baseHead = "HEAD"
	basePR   = "PR"
)

type gdPR struct {
	Number      int    `json:"number"`
	BaseRefName string `json:"baseRefName"`
}

// gitDiff is the state of git-diff mode.
type gitDiff struct {
	on      bool
	base    string // baseHead or basePR
	rev     string // revision the working tree is compared with
	pr      *gdPR  // the current branch's pull request when base is basePR
	split   bool
	files   []*diff.File
	byAbs   map[string]*diff.File
	sel     int
	scroll  int
	loading bool
	err     string
	gen     int
	ticks   int
}

type changesLoadedMsg struct {
	gen   int
	base  string
	rev   string
	pr    *gdPR
	files []*diff.File
	err   error
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
	gen, root, base := a.gd.gen, a.root, a.gd.base
	return func() tea.Msg { return loadChanges(gen, root, base) }
}

func loadChanges(gen int, root, base string) tea.Msg {
	m := changesLoadedMsg{gen: gen, base: base}
	m.rev = emptyTree
	if out, err := run(root, nil, "git", "rev-parse", "--verify", "-q", "HEAD"); err == nil {
		m.rev = strings.TrimSpace(string(out))
	}
	if base == basePR {
		out, err := run(root, nil, "gh", "pr", "view", "--json", "number,baseRefName")
		if err != nil {
			m.err = fmt.Errorf("このブランチのプルリクエストが見つかりません: %w", err)
			return m
		}
		var pr gdPR
		if err := json.Unmarshal(out, &pr); err != nil {
			m.err = err
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
		m.pr, m.rev = &pr, strings.TrimSpace(string(mb))
	}
	out, err := run(root, nil, "git", "diff", m.rev, "--no-color", "--no-ext-diff", "-M")
	if err != nil {
		m.err = err
		return m
	}
	files := diff.Parse(string(out))
	top, _ := run(root, nil, "git", "rev-parse", "--show-toplevel")
	topDir := strings.TrimSpace(string(top))
	if untracked, err := run(root, nil, "git", "ls-files", "--others", "--exclude-standard", "--full-name", "-z", ":/"); err == nil {
		for _, p := range strings.Split(string(untracked), "\x00") {
			if p == "" {
				continue
			}
			if f := untrackedFile(filepath.Join(topDir, p), p); f != nil {
				files = append(files, f)
			}
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })
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
	a.gd.rev, a.gd.pr = m.rev, m.pr
	sel := ""
	if f := a.gd.file(); f != nil {
		sel = f.Path()
	}
	a.gd.files = m.files
	a.gd.byAbs = map[string]*diff.File{}
	top := a.gitTopDir()
	for i, f := range m.files {
		a.gd.byAbs[filepath.Join(top, f.Path())] = f
		if f.Path() == sel {
			a.gd.sel = i
		}
	}
	a.gd.sel = clamp(a.gd.sel, 0, max(0, len(m.files)-1))
}

func (g *gitDiff) file() *diff.File {
	if g.sel < 0 || g.sel >= len(g.files) {
		return nil
	}
	return g.files[g.sel]
}

// ---- mode switch ----

func (a *App) toggleGitDiff() tea.Cmd {
	if a.gd.on {
		a.gd.on = false
		if a.focus == focusChanges {
			a.focus = focusExplorer
		}
		a.follow = true
		a.info("git-diff モードを終了しました")
		return nil
	}
	a.gd.on = true
	if a.gd.base == "" {
		a.gd.base = baseHead
	}
	a.showSide = true
	a.follow = true
	return a.reloadChanges()
}

// setGitDiffBase switches the comparison base ("head" or "pr").
func (a *App) setGitDiffBase(base string) tea.Cmd {
	a.gd.base = base
	a.gd.on = true
	a.showSide = true
	return a.reloadChanges()
}

func (a *App) cycleGitDiffBase() tea.Cmd {
	if a.gd.base == basePR {
		return a.setGitDiffBase(baseHead)
	}
	return a.setGitDiffBase(basePR)
}

func (a *App) toggleSplit() {
	a.gd.split = !a.gd.split
	a.follow = true
}

func (a *App) baseLabel() string {
	if a.gd.base == basePR {
		if a.gd.pr != nil {
			return fmt.Sprintf("PR #%d (%s)", a.gd.pr.Number, a.gd.pr.BaseRefName)
		}
		return "PR"
	}
	return "HEAD"
}

// gitDiffCommand handles ":gitdiff [base [head|pr] | split | unified]".
func (a *App) gitDiffCommand(arg string) tea.Cmd {
	f := strings.Fields(arg)
	if len(f) == 0 {
		return a.toggleGitDiff()
	}
	switch f[0] {
	case "on":
		if !a.gd.on {
			return a.toggleGitDiff()
		}
		return a.reloadChanges()
	case "off":
		if a.gd.on {
			return a.toggleGitDiff()
		}
	case "split":
		a.gd.split = true
		a.follow = true
	case "unified":
		a.gd.split = false
		a.follow = true
	case "base":
		if len(f) == 1 {
			return a.cycleGitDiffBase()
		}
		switch strings.ToLower(f[1]) {
		case "head":
			return a.setGitDiffBase(baseHead)
		case "pr":
			return a.setGitDiffBase(basePR)
		}
		a.fail(errString("使い方: :gitdiff base [head|pr]"))
	default:
		a.fail(errString("使い方: :gitdiff [on|off|split|unified|base [head|pr]]"))
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

func (a *App) handleChangesKey(msg tea.KeyMsg) tea.Cmd {
	g := &a.gd
	key := msg.String()
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
	case "b":
		return a.cycleGitDiffBase()
	case "s":
		a.toggleSplit()
	case "r", "R":
		return a.reloadChanges()
	case " ":
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

// explorerH is the height of the file tree: 7/10 of the sidebar in git-diff
// mode, all of it otherwise.
func (a *App) explorerH() int {
	h := a.h - 1
	if a.gd.on {
		return h * 7 / 10
	}
	return h
}

func (a *App) renderChanges(w, h int, focused bool) []string {
	g := &a.gd
	out := make([]string, 0, h)
	title := " CHANGES  base: " + a.baseLabel()
	out = append(out, stSideTitle.Render(fit(title, w)))
	body := h - 1
	if g.sel < g.scroll {
		g.scroll = g.sel
	}
	if g.sel >= g.scroll+body {
		g.scroll = g.sel - body + 1
	}
	g.scroll = max(0, g.scroll)
	for i := 0; i < body; i++ {
		idx := g.scroll + i
		if idx >= len(g.files) {
			msg := ""
			if i == 0 && len(g.files) == 0 {
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
		f := g.files[idx]
		bg := cSideBg
		if idx == g.sel {
			bg = cListSelUnf
			if focused {
				bg = cListSel
			}
		}
		letter, col := statusLetter(f)
		stats := fmt.Sprintf(" +%d -%d", f.Added, f.Deleted)
		nameW := w - 3 - ansi.StringWidth(stats)
		name := f.Path()
		if ansi.StringWidth(name) > nameW {
			name = "…" + ansi.TruncateLeft(name, ansi.StringWidth(name)-nameW+1, "")
		}
		line := bgfg(bg, col).Bold(true).Render(" "+letter) + bgfg(bg, cSideFg).Render(" "+fit(name, max(0, nameW))) +
			bgfg(bg, cAddSign).Render(fmt.Sprintf(" +%d", f.Added)) + bgfg(bg, cDelSign).Render(fmt.Sprintf(" -%d", f.Deleted))
		out = append(out, fitStyled(line, w, bgfg(bg, cSideFg)))
	}
	return out
}

// ---- per-tab diff view ----

// dispRow is one screen row of a diff view. old/new are line indexes into the
// base and the buffer, or -1 when that side has no line on this row.
type dispRow struct {
	old, new int
	changed  bool
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
}

// diffFile returns the changed file shown by tab t in git-diff mode, or nil
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

// diffFor returns the up-to-date diff view of t, or nil outside git-diff mode.
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
	dv.rows = dv.rows[:0]
	dv.newIdx = make([]int, len(lines))
	dv.changed = make([]bool, len(lines))
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
		add := func(o, n int) {
			if n >= 0 {
				dv.newIdx[n] = len(dv.rows)
				dv.changed[n] = true
			}
			dv.rows = append(dv.rows, dispRow{old: o, new: n, changed: true})
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

// dvGeom is where the parts of a diff view sit on screen.
type dvGeom struct {
	top, h         int // first content row and number of content rows
	leftW          int // width of the base pane (0 when unified)
	rightX, rightW int // the editable pane
}

func (a *App) dvGeom(l layout) dvGeom {
	g := dvGeom{top: l.editY + 1, h: max(0, l.editH-1), rightX: l.rightX, rightW: l.rightW}
	if a.gd.split {
		g.leftW = (l.rightW - 1) / 2
		g.rightX = l.rightX + g.leftW + 1
		g.rightW = l.rightW - g.leftW - 1
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

// ---- rendering ----

func (a *App) renderDiffText(t *tab, dv *diffView, l layout, focused bool) []string {
	g := a.dvGeom(l)
	b := t.buf
	out := make([]string, 0, l.editH)

	hint := "[分割 ⇄ 統合]"
	head := " " + a.rel(t.buf.Path) + "  比較元: " + a.baseLabel()
	hw := ansi.StringWidth(hint) + 1
	out = append(out, bgfg(cTabBarBg, curTheme.Strong).Bold(true).Render(fit(head, max(0, l.rightW-hw)))+
		bgfg(cTabBarBg, curTheme.Dim).Render(fit(hint, hw)))

	opts := a.editorOpts(t, focused)
	opts.rowBg = func(row int) string {
		if row >= 0 && row < len(dv.changed) && dv.changed[row] {
			return cAddBg
		}
		return ""
	}
	nw := len(fmt.Sprint(len(dv.old))) + 4 // same width as gutterWidth

	start := dv.start(b.ScrollRow)
	saved := b.ScrollRow
	defer func() { b.ScrollRow = saved }()
	for i := 0; i < g.h; {
		idx := start + i
		if idx >= len(dv.rows) {
			for ; i < g.h; i++ {
				out = append(out, a.diffBlank(l.rightW))
			}
			break
		}
		r := dv.rows[idx]
		if r.new < 0 {
			var line string
			if a.gd.split {
				line = a.diffOld(dv, r, nw, g.leftW) + stSeparator.Render("│") + bgfg(cSideBg, cLineNo).Render(fit("", g.rightW))
			} else {
				line = a.diffOldUnified(dv, r, gutterWidth(b), l.rightW)
			}
			out = append(out, line)
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
				out = append(out, a.diffOld(dv, dv.rows[idx+k], nw, g.leftW)+stSeparator.Render("│")+edit[k])
			} else {
				out = append(out, edit[k])
			}
		}
		i += n
	}
	return out
}

func (a *App) diffBlank(w int) string { return stEmpty.Render(fit("", w)) }

// diffOld draws the base side of a row in split mode.
func (a *App) diffOld(dv *diffView, r dispRow, nw, w int) string {
	if r.old < 0 {
		return bgfg(cSideBg, cLineNo).Render(fit("", w))
	}
	bg, sc := cEditorBg, cLineNo
	sign := " "
	if r.changed {
		bg, sc, sign = cDelBg, cDelSign, "-"
	}
	gutter := bgfg(bg, sc).Render(sign) + bgfg(bg, cLineNo).Render(fmt.Sprintf("%*d  ", nw-3, r.old+1))
	return gutter + highlightText(dv.old[r.old], dv.lexer, bg, w-nw)
}

// diffOldUnified draws a deleted line between the current lines.
func (a *App) diffOldUnified(dv *diffView, r dispRow, nw, w int) string {
	gutter := bgfg(cDelBg, cDelSign).Render("-") + bgfg(cDelBg, cLineNo).Render(fmt.Sprintf("%*d  ", nw-3, r.old+1))
	return gutter + highlightText(dv.old[r.old], dv.lexer, cDelBg, w-nw)
}

// diffMouseToCursor moves the cursor to the buffer line under (x, y) in a
// diff view. Rows that exist only on the base side snap to the next line.
func (a *App) diffMouseToCursor(t *tab, dv *diffView, l layout, x, y int, extend bool) {
	g := a.dvGeom(l)
	b := t.buf
	x = max(x, g.rightX)
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
