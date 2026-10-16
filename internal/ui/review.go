package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/diff"
)

// Review colors; set by applyTheme.
var cAddBg, cDelBg, cAddSign, cDelSign, cHunkFg, cHunkBg, cNoteBg string

type prInfo struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Body        string `json:"body"`
	HeadRefOid  string `json:"headRefOid"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

type reviewComment struct {
	Path    string
	Line    int
	Side    string // "RIGHT" (new) or "LEFT" (old)
	Body    string
	Author  string
	Pending bool
}

type rowKind int

const (
	rowHunk rowKind = iota
	rowLine
	rowComment
	rowInfo
)

type reviewRow struct {
	kind        rowKind
	text        string
	line        *diff.Line // unified
	left, right *diff.Line // split
	comment     *reviewComment
}

// review is a tab showing a diff for review: either a GitHub pull request
// (via the gh CLI) or the local working tree (git diff HEAD).
type review struct {
	id         int
	root       string
	prNumber   int // 0 = local changes
	pr         *prInfo
	files      []*diff.File
	comments   []*reviewComment
	loading    bool
	err        string
	fileSel    int
	fileScroll int
	focusFiles bool
	cursor     int
	scroll     int
	split      bool
	viewed     map[string]bool
	want       string // path to select once files are loaded

	rows    []reviewRow
	rowsFor string
	lexers  map[string]chroma.Lexer
	pageH   int
}

var reviewSeq int

func newReview(root string, prNumber int) *review {
	reviewSeq++
	return &review{id: reviewSeq, root: root, prNumber: prNumber, loading: true, viewed: map[string]bool{}, lexers: map[string]chroma.Lexer{}, split: true}
}

func (r *review) tabTitle() string {
	if r.prNumber > 0 {
		return fmt.Sprintf("PR #%d", r.prNumber)
	}
	return "変更のレビュー"
}

func (r *review) file() *diff.File {
	if r.fileSel < 0 || r.fileSel >= len(r.files) {
		return nil
	}
	return r.files[r.fileSel]
}

// ---- loading ----

type reviewLoadedMsg struct {
	id       int
	pr       *prInfo
	files    []*diff.File
	comments []*reviewComment
	err      error
}

type reviewSubmittedMsg struct {
	id    int
	event string
	err   error
}

type prSummary struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	IsDraft     bool   `json:"isDraft"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

type prListMsg struct {
	prs []prSummary
	err error
}

func run(dir string, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("%s: %s", name, firstLine(msg))
	}
	return out, nil
}

func listPRs(root string) tea.Cmd {
	return func() tea.Msg {
		out, err := run(root, nil, "gh", "pr", "list", "--limit", "100", "--json", "number,title,headRefName,isDraft,author")
		if err != nil {
			return prListMsg{err: err}
		}
		var prs []prSummary
		err = json.Unmarshal(out, &prs)
		return prListMsg{prs: prs, err: err}
	}
}

func (r *review) load() tea.Cmd {
	id, root, num := r.id, r.root, r.prNumber
	if num == 0 {
		return func() tea.Msg { return loadLocal(id, root) }
	}
	return func() tea.Msg { return loadPR(id, root, num) }
}

func loadLocal(id int, root string) tea.Msg {
	out, err := run(root, nil, "git", "diff", "HEAD", "--no-color", "--no-ext-diff", "-M")
	if err != nil {
		// no commits yet: show staged + unstaged against the empty tree
		out, err = run(root, nil, "git", "diff", "--no-color", "--no-ext-diff")
		if err != nil {
			return reviewLoadedMsg{id: id, err: err}
		}
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
	return reviewLoadedMsg{id: id, files: files}
}

func untrackedFile(abs, rel string) *diff.File {
	f := &diff.File{NewPath: rel, Status: "added"}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() {
		return nil
	}
	if st.Size() > 1<<20 {
		f.Status = "binary"
		return f
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	if bytes.IndexByte(data, 0) >= 0 {
		f.Status = "binary"
		return f
	}
	text := strings.TrimSuffix(string(data), "\n")
	lines := strings.Split(text, "\n")
	h := diff.Hunk{Header: fmt.Sprintf("@@ -0,0 +1,%d @@ (未追跡ファイル)", len(lines))}
	for i, l := range lines {
		h.Lines = append(h.Lines, diff.Line{Kind: diff.Add, Text: l, NewNo: i + 1})
	}
	f.Hunks = []diff.Hunk{h}
	f.Added = len(lines)
	return f
}

func loadPR(id int, root string, num int) tea.Msg {
	n := strconv.Itoa(num)
	out, err := run(root, nil, "gh", "pr", "view", n, "--json", "number,title,url,body,headRefOid,headRefName,baseRefName,state,isDraft,author")
	if err != nil {
		return reviewLoadedMsg{id: id, err: err}
	}
	var pr prInfo
	if err := json.Unmarshal(out, &pr); err != nil {
		return reviewLoadedMsg{id: id, err: err}
	}
	out, err = run(root, nil, "gh", "pr", "diff", n, "--color", "never")
	if err != nil {
		return reviewLoadedMsg{id: id, err: err}
	}
	files := diff.Parse(string(out))

	var comments []*reviewComment
	if out, err := run(root, nil, "gh", "api", "--paginate", fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments", num)); err == nil {
		var raw []struct {
			Path         string `json:"path"`
			Line         *int   `json:"line"`
			OriginalLine *int   `json:"original_line"`
			Side         string `json:"side"`
			Body         string `json:"body"`
			User         struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		// --paginate concatenates JSON arrays; normalise "][" boundaries.
		data := bytes.ReplaceAll(out, []byte("]["), []byte(","))
		if json.Unmarshal(data, &raw) == nil {
			for _, c := range raw {
				line := c.Line
				if line == nil {
					continue // outdated comment
				}
				side := c.Side
				if side == "" {
					side = "RIGHT"
				}
				comments = append(comments, &reviewComment{Path: c.Path, Line: *line, Side: side, Body: c.Body, Author: c.User.Login})
			}
		}
	}
	return reviewLoadedMsg{id: id, pr: &pr, files: files, comments: comments}
}

// submit posts a review with all pending comments. event is APPROVE,
// REQUEST_CHANGES or COMMENT.
func (r *review) submit(event, body string) tea.Cmd {
	if r.pr == nil {
		return nil
	}
	type c struct {
		Path string `json:"path"`
		Line int    `json:"line"`
		Side string `json:"side"`
		Body string `json:"body"`
	}
	payload := struct {
		CommitID string `json:"commit_id"`
		Body     string `json:"body,omitempty"`
		Event    string `json:"event"`
		Comments []c    `json:"comments"`
	}{CommitID: r.pr.HeadRefOid, Body: body, Event: event, Comments: []c{}}
	for _, cm := range r.comments {
		if cm.Pending {
			payload.Comments = append(payload.Comments, c{cm.Path, cm.Line, cm.Side, cm.Body})
		}
	}
	data, _ := json.Marshal(payload)
	id, root, num := r.id, r.root, r.pr.Number
	return func() tea.Msg {
		_, err := run(root, data, "gh", "api", "-X", "POST", fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/reviews", num), "--input", "-")
		return reviewSubmittedMsg{id: id, event: event, err: err}
	}
}

// selectWanted selects the wanted file if the file list is already loaded.
func (r *review) selectWanted() {
	for i, f := range r.files {
		if f.Path() == r.want {
			r.fileSel, r.rowsFor, r.cursor, r.scroll, r.want = i, "", 0, 0, ""
			return
		}
	}
}

func (r *review) applyLoaded(m reviewLoadedMsg) {
	r.loading = false
	if m.err != nil {
		r.err = m.err.Error()
		return
	}
	r.err = ""
	r.pr = m.pr
	path := ""
	if f := r.file(); f != nil {
		path = f.Path()
	}
	if r.want != "" {
		path, r.want = r.want, ""
	}
	r.files = m.files
	// keep local pending comments across reloads
	var pending []*reviewComment
	for _, c := range r.comments {
		if c.Pending {
			pending = append(pending, c)
		}
	}
	r.comments = append(m.comments, pending...)
	r.fileSel = 0
	for i, f := range r.files {
		if f.Path() == path {
			r.fileSel = i
		}
	}
	r.rowsFor = ""
	r.cursor = clamp(r.cursor, 0, max(0, len(r.buildRows())-1))
}

func (r *review) pendingCount() int {
	n := 0
	for _, c := range r.comments {
		if c.Pending {
			n++
		}
	}
	return n
}

// ---- rows ----

func lineKey(l *diff.Line) (string, int) {
	if l.Kind == diff.Del {
		return "LEFT", l.OldNo
	}
	return "RIGHT", l.NewNo
}

func (r *review) commentsFor(path string, l *diff.Line) []*reviewComment {
	if l == nil {
		return nil
	}
	side, no := lineKey(l)
	var out []*reviewComment
	for _, c := range r.comments {
		if c.Path == path && c.Side == side && c.Line == no {
			out = append(out, c)
		}
	}
	return out
}

func (r *review) buildRows() []reviewRow {
	f := r.file()
	key := fmt.Sprintf("%d/%v/%d", r.fileSel, r.split, len(r.comments))
	if f != nil {
		key += f.Path()
	}
	if key == r.rowsFor {
		return r.rows
	}
	r.rowsFor = key
	r.rows = r.rows[:0]
	if f == nil {
		return r.rows
	}
	if f.Status == "binary" {
		r.rows = append(r.rows, reviewRow{kind: rowInfo, text: "バイナリファイルのため差分を表示できません"})
		return r.rows
	}
	if len(f.Hunks) == 0 {
		r.rows = append(r.rows, reviewRow{kind: rowInfo, text: "内容の変更はありません（名前の変更・権限の変更）"})
	}
	addComments := func(ls ...*diff.Line) {
		for _, l := range ls {
			for _, c := range r.commentsFor(f.Path(), l) {
				r.rows = append(r.rows, commentRows(c)...)
			}
		}
	}
	for hi := range f.Hunks {
		h := &f.Hunks[hi]
		r.rows = append(r.rows, reviewRow{kind: rowHunk, text: h.Header})
		if !r.split {
			for li := range h.Lines {
				l := &h.Lines[li]
				r.rows = append(r.rows, reviewRow{kind: rowLine, line: l})
				addComments(l)
			}
			continue
		}
		for i := 0; i < len(h.Lines); {
			l := &h.Lines[i]
			if l.Kind == diff.Context {
				r.rows = append(r.rows, reviewRow{kind: rowLine, left: l, right: l})
				addComments(l)
				i++
				continue
			}
			var dels, adds []*diff.Line
			for i < len(h.Lines) && h.Lines[i].Kind == diff.Del {
				dels = append(dels, &h.Lines[i])
				i++
			}
			for i < len(h.Lines) && h.Lines[i].Kind == diff.Add {
				adds = append(adds, &h.Lines[i])
				i++
			}
			for k := 0; k < max(len(dels), len(adds)); k++ {
				row := reviewRow{kind: rowLine}
				if k < len(dels) {
					row.left = dels[k]
				}
				if k < len(adds) {
					row.right = adds[k]
				}
				r.rows = append(r.rows, row)
				addComments(row.left, row.right)
			}
		}
	}
	return r.rows
}

func commentRows(c *reviewComment) []reviewRow {
	head := c.Author
	if c.Pending {
		head = "あなた (未送信)"
	}
	rows := []reviewRow{{kind: rowComment, text: "▍" + head, comment: c}}
	for _, l := range strings.Split(strings.TrimRight(c.Body, "\n"), "\n") {
		rows = append(rows, reviewRow{kind: rowComment, text: "▍  " + l, comment: c})
	}
	return rows
}

// target returns the diff line a new comment at the cursor attaches to.
func (r *review) target() *diff.Line {
	rows := r.buildRows()
	if r.cursor < 0 || r.cursor >= len(rows) {
		return nil
	}
	row := rows[r.cursor]
	if row.kind != rowLine {
		return nil
	}
	if row.line != nil {
		return row.line
	}
	if row.right != nil {
		return row.right
	}
	return row.left
}

func (r *review) addComment(body string) {
	l := r.target()
	f := r.file()
	if l == nil || f == nil {
		return
	}
	side, no := lineKey(l)
	r.comments = append(r.comments, &reviewComment{Path: f.Path(), Line: no, Side: side, Body: body, Pending: true})
}

func (r *review) deletePendingAtCursor() bool {
	rows := r.buildRows()
	if r.cursor >= len(rows) || rows[r.cursor].comment == nil || !rows[r.cursor].comment.Pending {
		return false
	}
	target := rows[r.cursor].comment
	for i, c := range r.comments {
		if c == target {
			r.comments = append(r.comments[:i], r.comments[i+1:]...)
			return true
		}
	}
	return false
}

// notesMarkdown renders pending comments as Markdown (to hand to an agent).
func (r *review) notesMarkdown() string {
	var sb strings.Builder
	for _, c := range r.comments {
		if !c.Pending {
			continue
		}
		side := ""
		if c.Side == "LEFT" {
			side = " (削除行)"
		}
		fmt.Fprintf(&sb, "- %s:%d%s\n", c.Path, c.Line, side)
		for _, l := range strings.Split(c.Body, "\n") {
			fmt.Fprintf(&sb, "  %s\n", l)
		}
	}
	return sb.String()
}

func (r *review) moveHunk(dir int) {
	rows := r.buildRows()
	for i := r.cursor + dir; i >= 0 && i < len(rows); i += dir {
		if rows[i].kind == rowHunk {
			r.cursor = min(i+1, len(rows)-1)
			r.scroll = max(0, i-2)
			return
		}
	}
	// past the last hunk: go to the next/previous file
	if dir > 0 && r.fileSel < len(r.files)-1 {
		r.selectFile(r.fileSel + 1)
	} else if dir < 0 && r.fileSel > 0 {
		r.selectFile(r.fileSel - 1)
		rows = r.buildRows()
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i].kind == rowHunk {
				r.cursor = min(i+1, len(rows)-1)
				r.scroll = max(0, i-2)
				return
			}
		}
	}
}

func (r *review) selectFile(i int) {
	if i < 0 || i >= len(r.files) {
		return
	}
	r.fileSel = i
	r.scroll = 0
	r.cursor = 0
	rows := r.buildRows()
	for j, row := range rows {
		if row.kind == rowLine {
			r.cursor = j
			break
		}
	}
}

func (r *review) moveCursor(d int) {
	rows := r.buildRows()
	r.cursor = clamp(r.cursor+d, 0, max(0, len(rows)-1))
}

// ---- rendering ----

func (r *review) lexer(path string) chroma.Lexer {
	if l, ok := r.lexers[path]; ok {
		return l
	}
	l := lexers.Match(filepath.Base(path))
	if l != nil {
		l = chroma.Coalesce(l)
	}
	r.lexers[path] = l
	return l
}

func (r *review) render(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	// header
	var head string
	switch {
	case r.pr != nil:
		state := r.pr.State
		if r.pr.IsDraft {
			state = "DRAFT"
		}
		head = fmt.Sprintf(" #%d %s  [%s]  @%s  %s ← %s", r.pr.Number, r.pr.Title, state, r.pr.Author.Login, r.pr.BaseRefName, r.pr.HeadRefName)
	case r.prNumber > 0:
		head = fmt.Sprintf(" PR #%d", r.prNumber)
	default:
		head = " ローカルの変更 (git diff HEAD + 未追跡ファイル)"
	}
	out = append(out, bgfg(cTabBarBg, curTheme.Strong).Bold(true).Render(fit(head, w)))
	body := h - 2
	if r.loading || r.err != "" || len(r.files) == 0 {
		msg := "  読み込み中…"
		if r.err != "" {
			msg = "  エラー: " + r.err
		} else if !r.loading {
			msg = "  変更はありません"
		}
		out = append(out, stEmpty.Render(fit(msg, w)))
		for len(out) < h-1 {
			out = append(out, stEmpty.Render(fit("", w)))
		}
		return append(out, r.helpLine(w))
	}

	listW := clamp(w/4, 20, 40)
	diffW := w - listW - 1
	left := r.renderFiles(listW, body, focused && r.focusFiles)
	right := r.renderDiff(diffW, body, focused && !r.focusFiles)
	for i := 0; i < body; i++ {
		out = append(out, left[i]+stSeparator.Render("│")+right[i])
	}
	return append(out, r.helpLine(w))
}

func (r *review) helpLine(w int) string {
	help := " j/k:移動  ]/[:ハンク  J/K:ファイル  Tab:一覧↔差分  s:分割/統合  v:確認済み  c:コメント  y:コメントをコピー  r:再読込  q:閉じる"
	if r.pr != nil {
		help = " j/k:移動  ]/[:ハンク  J/K:ファイル  c:コメント  d:下書き削除  A:承認  R:変更を要求  S:コメント送信  C:チェックアウト  r:再読込  q:閉じる"
	}
	if n := r.pendingCount(); n > 0 {
		help = fmt.Sprintf(" 未送信コメント %d 件 |", n) + help
	}
	return stPanelDim.Render(fit(help, w))
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

func (r *review) renderFiles(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	title := fmt.Sprintf(" ファイル (%d)", len(r.files))
	out = append(out, stSideTitle.Render(fit(title, w)))
	body := h - 1
	if r.fileSel < r.fileScroll {
		r.fileScroll = r.fileSel
	}
	if r.fileSel >= r.fileScroll+body {
		r.fileScroll = r.fileSel - body + 1
	}
	for i := 0; i < body; i++ {
		idx := r.fileScroll + i
		if idx >= len(r.files) {
			out = append(out, stSide.Render(fit("", w)))
			continue
		}
		f := r.files[idx]
		bg := cSideBg
		if idx == r.fileSel {
			bg = cListSelUnf
			if focused {
				bg = cListSel
			}
		}
		letter, col := statusLetter(f)
		mark := " "
		if r.viewed[f.Path()] {
			mark = "✓"
		}
		stats := fmt.Sprintf(" +%d -%d", f.Added, f.Deleted)
		nameW := w - 4 - ansi.StringWidth(stats)
		name := f.Path()
		if ansi.StringWidth(name) > nameW {
			name = "…" + ansi.TruncateLeft(name, ansi.StringWidth(name)-nameW+1, "")
		}
		fg := cSideFg
		if r.viewed[f.Path()] {
			fg = cLineNo
		}
		line := bgfg(bg, cAddSign).Render(mark) + bgfg(bg, col).Bold(true).Render(letter) + bgfg(bg, fg).Render(" "+fit(name, nameW)) +
			bgfg(bg, cAddSign).Render(fmt.Sprintf(" +%d", f.Added)) + bgfg(bg, cDelSign).Render(fmt.Sprintf(" -%d", f.Deleted))
		out = append(out, fitStyled(line, w, bgfg(bg, cSideFg)))
	}
	return out
}

func (r *review) renderDiff(w, h int, focused bool) []string {
	rows := r.buildRows()
	f := r.file()
	r.pageH = h - 1
	out := make([]string, 0, h)
	title := " " + f.Path()
	if f.Status == "renamed" {
		title = " " + f.OldPath + " → " + f.NewPath
	}
	out = append(out, bgfg(cEditorBg, curTheme.PanelTitle).Bold(true).Render(fit(title, w)))
	body := h - 1
	if r.cursor < r.scroll {
		r.scroll = r.cursor
	}
	if r.cursor >= r.scroll+body {
		r.scroll = r.cursor - body + 1
	}
	lex := r.lexer(f.Path())
	for i := 0; i < body; i++ {
		idx := r.scroll + i
		if idx >= len(rows) {
			out = append(out, stEmpty.Render(fit("", w)))
			continue
		}
		row := rows[idx]
		cur := focused && idx == r.cursor
		switch row.kind {
		case rowHunk:
			st := bgfg(cHunkBg, cHunkFg)
			if cur {
				st = bgfg(cListSel, cHunkFg)
			}
			out = append(out, st.Render(fit(" "+row.text, w)))
		case rowInfo:
			out = append(out, stPanelDim.Render(fit("  "+row.text, w)))
		case rowComment:
			st := bgfg(cNoteBg, cEditorFg)
			if row.comment.Pending {
				st = bgfg(cNoteBg, cDirty)
			}
			if cur {
				st = bgfg(cListSel, curTheme.Strong)
			}
			out = append(out, st.Render(fit("        "+row.text, w)))
		case rowLine:
			if row.line != nil {
				out = append(out, renderUnifiedLine(row.line, lex, w, cur))
			} else {
				half := (w - 1) / 2
				l := renderSide(row.left, lex, half, cur, true)
				rr := renderSide(row.right, lex, w-half-1, cur, false)
				out = append(out, l+stPanelBord.Render("│")+rr)
			}
		}
	}
	return out
}

func lineStyle(k diff.Kind) (bg, sign string) {
	switch k {
	case diff.Add:
		return cAddBg, cAddSign
	case diff.Del:
		return cDelBg, cDelSign
	}
	return cEditorBg, cLineNo
}

func renderUnifiedLine(l *diff.Line, lex chroma.Lexer, w int, cur bool) string {
	bg, sc := lineStyle(l.Kind)
	no := func(n int) string {
		if n == 0 {
			return "    "
		}
		return fmt.Sprintf("%4d", n)
	}
	gbg := bg
	if cur {
		gbg = cListSel
	}
	gutter := bgfg(gbg, cLineNo).Render(no(l.OldNo)+" "+no(l.NewNo)+" ") + bgfg(gbg, sc).Render(string(l.Kind)+" ")
	gw := 12
	return gutter + highlightText(l.Text, lex, bg, w-gw)
}

func renderSide(l *diff.Line, lex chroma.Lexer, w int, cur bool, isLeft bool) string {
	if l == nil {
		return bgfg(cSideBg, cLineNo).Render(fit("", w))
	}
	bg, sc := lineStyle(l.Kind)
	n := l.NewNo
	if isLeft {
		n = l.OldNo
	}
	gbg := bg
	if cur {
		gbg = cListSel
	}
	sign := " "
	if l.Kind != diff.Context {
		sign = string(l.Kind)
	}
	gutter := bgfg(gbg, cLineNo).Render(fmt.Sprintf("%4d ", n)) + bgfg(gbg, sc).Render(sign+" ")
	return gutter + highlightText(l.Text, lex, bg, w-7)
}

// highlightText renders one line of code with syntax colors on bg, fitted to w.
func highlightText(text string, lex chroma.Lexer, bg string, w int) string {
	if w <= 0 {
		return ""
	}
	text = strings.ReplaceAll(text, "\t", "    ")
	var sb strings.Builder
	used := 0
	emit := func(s, fg string) bool {
		sw := ansi.StringWidth(s)
		if used+sw > w {
			s = ansi.Truncate(s, w-used, "")
			sw = ansi.StringWidth(s)
		}
		sb.WriteString(styleFor(cellStyle{fg: fg, bg: bg}).Render(s))
		used += sw
		return used < w
	}
	if lex != nil {
		if it, err := lex.Tokenise(nil, text); err == nil {
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
		} else {
			emit(text, cEditorFg)
		}
	} else {
		emit(text, cEditorFg)
	}
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
