package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// Reviewing a pull request inside diff mode (base PR): comments on lines,
// ranges and files, replies, resolving, suggestions, viewed marks and
// sending the review. The keys sit under Space v, since the right side is
// an editor.
//
// GitHub numbers comment lines in the pull request's head commit. Instead of
// mapping them onto edited files, a file takes comments only while its
// working tree equals that commit: the numbers then agree.

// prReview is the review state of the pull request shown.
type prReview struct {
	num     int         // the pull request this state belongs to
	threads []*prThread // from GitHub
	pending []*prThread // written here, sent with the review
	viewed  map[string]bool
	sync    prSync
	seq     int // bumped on every load; older answers are dropped
}

// yesNo is a question answered with y or n on the status line.
type yesNo struct {
	prompt  string
	yes, no func() tea.Cmd
}

func (a *App) handleConfirmKey(key string) tea.Cmd {
	c := a.confirm
	switch key {
	case "y", "Y":
		a.confirm = nil
		return c.yes()
	case "n", "N", "esc", "ctrl+c":
		a.confirm = nil
		return c.no()
	}
	return nil // any other key leaves the question open
}

// ---- loading ----

// prReviewMsg carries the review of a pull request and how HEAD stands
// against its head commit.
type prReviewMsg struct {
	seq  int
	num  int
	sync prSync
	data prReviewData
	err  error
}

// loadReview fetches the pull request's head commit, compares it with HEAD
// and reads its comments and viewed files.
func (a *App) loadReview() tea.Cmd {
	pr := a.gd.pr
	if pr == nil {
		return nil
	}
	a.gd.rv.seq++
	root, seq := a.root, a.gd.rv.seq
	return func() tea.Msg {
		fetchPR(root, pr)
		m := prReviewMsg{seq: seq, num: pr.Number, sync: syncWith(root, pr.HeadRefOid)}
		if pr.ID != "" {
			m.data, m.err = loadReview(root, pr)
		}
		return m
	}
}

func (a *App) handlePRReview(m prReviewMsg) tea.Cmd {
	rv := &a.gd.rv
	if m.seq != rv.seq || !a.prShown() || a.gd.pr.Number != m.num {
		return nil
	}
	rv.sync = m.sync
	if m.err != nil {
		a.fail(m.err)
	} else {
		rv.threads, rv.viewed = m.data.threads, m.data.viewed
		if c := a.card; c != nil && !c.thread.Pending {
			a.card = nil
			for _, t := range rv.threads {
				if t.ID == c.thread.ID {
					a.card = &prCard{thread: t, path: c.path}
				}
			}
		}
	}
	return a.reloadChanges() // the head commit may have just arrived
}

// prShown reports whether diff mode compares with a pull request's base.
func (a *App) prShown() bool {
	return a.gd.on && a.gd.base == basePR && a.gd.pr != nil
}

// ---- files ----

// repoPath is abs relative to the repository top, as GitHub names files.
func (a *App) repoPath(abs string) string {
	if r, err := filepath.Rel(a.gitTopDir(), abs); err == nil {
		return filepath.ToSlash(r)
	}
	return abs
}

// isStale reports whether file p (from the repository top) differs from the
// pull request's head commit, so its line numbers may not match GitHub's.
func (a *App) isStale(p string) bool {
	return a.gd.staleAll || a.gd.stale[p]
}

// threadsIn returns the threads of file p, sent ones first.
func (rv *prReview) threadsIn(p string) []*prThread {
	var out []*prThread
	for _, ts := range [][]*prThread{rv.threads, rv.pending} {
		for _, t := range ts {
			if t.Path == p {
				out = append(out, t)
			}
		}
	}
	return out
}

// lineThreads returns the threads shown at line (1-based) of file p.
func (rv *prReview) lineThreads(p string, line int) []*prThread {
	var out []*prThread
	for _, t := range rv.threadsIn(p) {
		if t.onLine(line) {
			out = append(out, t)
		}
	}
	return out
}

// fileThreads returns the comments on file p as a whole.
func (rv *prReview) fileThreads(p string) []*prThread {
	var out []*prThread
	for _, t := range rv.threadsIn(p) {
		if t.File {
			out = append(out, t)
		}
	}
	return out
}

func (rv *prReview) viewedCount(files []*diff.File) int {
	n := 0
	for _, f := range files {
		if rv.viewed[f.Path()] {
			n++
		}
	}
	return n
}

// commentMark returns the marker color for line row (0-based) of t, or "".
func (a *App) commentMark(t *tab) func(row int) string {
	if !a.prShown() || t.buf.Path == "" {
		return nil
	}
	p := a.repoPath(t.buf.Path)
	if a.isStale(p) {
		return nil // the marks would land on the wrong lines
	}
	rows := map[int]bool{}
	for _, th := range a.gd.rv.threadsIn(p) {
		if th.onLine(th.Line) {
			rows[th.Line-1] = true
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return func(row int) string {
		if rows[row] {
			return curTheme.Renamed
		}
		return ""
	}
}

// ---- the Space v menu ----

var reviewItems = []spaceItem{
	{"c", "rv.item.comment", true},
	{"C", "rv.item.filecomment", true},
	{"t", "rv.item.thread", false},
	{"v", "rv.item.viewed", false},
	{"A", "rv.item.approve", false},
	{"R", "rv.item.request", false},
	{"S", "rv.item.commentonly", false},
	{"y", "rv.item.copyurl", false},
	{"o", "rv.item.openurl", false},
}

// openReviewMenu handles Space v.
func (a *App) openReviewMenu() tea.Cmd {
	if !a.prShown() {
		a.infoText(i18n.T("rv.needpr"))
		return nil
	}
	a.pending = "space v"
	return nil
}

// reviewCommand runs Space v <key> from the command palette.
func (a *App) reviewCommand(key string) tea.Cmd {
	if !a.prShown() {
		return a.openReviewMenu() // says where reviewing works
	}
	return a.reviewKey(key)
}

// reviewKey runs the command of Space v <key>.
func (a *App) reviewKey(key string) tea.Cmd {
	if !a.prShown() {
		return nil
	}
	switch key {
	case "c":
		return a.commentLines()
	case "C":
		return a.commentFile()
	case "t":
		return a.openThread()
	case "v":
		return a.toggleViewed()
	case "A":
		return a.startSubmit("APPROVE")
	case "R":
		return a.startSubmit("REQUEST_CHANGES")
	case "S":
		return a.startSubmit("COMMENT")
	case "y":
		return a.copyPRURL()
	case "o":
		return a.openPRURL()
	}
	return nil
}

// reviewTarget is the file a review command acts on: the selected one in
// CHANGES, otherwise the current tab's.
func (a *App) reviewTarget() (string, *tab) {
	if a.focus == focusChanges {
		if f := a.gd.file(); f != nil {
			abs := filepath.Join(a.gitTopDir(), f.Path())
			for _, t := range a.tabs {
				if t.path() == abs {
					return f.Path(), t
				}
			}
			return f.Path(), nil
		}
		return "", nil
	}
	if t := a.curText(); t != nil && t.buf.Path != "" {
		return a.repoPath(t.buf.Path), t
	}
	return "", nil
}

// checkCommentable reports why file p of tab t cannot take comments, or nil.
func (a *App) checkCommentable(p string, t *tab) error {
	if p == "" || a.gd.byAbs[filepath.Join(a.gitTopDir(), p)] == nil {
		return errors.New(i18n.T("rv.needchanged"))
	}
	if a.isStale(p) {
		return errors.New(i18n.T("rv.localchanged"))
	}
	if t != nil && t.dirty() {
		return errors.New(i18n.T("rv.unsaved"))
	}
	return nil
}

// commentLines starts a comment on the cursor line or the selected lines.
func (a *App) commentLines() tea.Cmd {
	p, t := a.reviewTarget()
	if a.focus == focusChanges || t == nil {
		a.fail(errors.New(i18n.T("rv.selectline")))
		return nil
	}
	if err := a.checkCommentable(p, t); err != nil {
		a.fail(err)
		return nil
	}
	b := t.buf
	first, last := b.Cur.Row, b.Cur.Row
	if s, e, ok := b.Selection(); ok {
		first, last = s.Row, e.Row
		if e.Col == 0 && e.Row > s.Row {
			last-- // a whole-line selection ends at the start of the next line
		}
	}
	if dv := a.diffFor(t); dv == nil || !dv.inHunk(first) || !dv.inHunk(last) {
		a.fail(errors.New(i18n.T("rv.outofdiff")))
		return nil
	}
	var orig []string
	for r := first; r <= last; r++ {
		orig = append(orig, string(b.Lines[r]))
	}
	where := fmt.Sprintf(i18n.T("rv.lineof"), p, last+1)
	if last > first {
		where = fmt.Sprintf(i18n.T("rv.rangeof"), p, first+1, last+1)
	}
	return a.compose(i18n.T("rv.commentlabel")+where, "", orig, func(body string) tea.Cmd {
		th := &prThread{Path: p, Line: last + 1, Side: "RIGHT", Pending: true, Comments: []prComment{{Author: i18n.T("rv.you"), Body: body, Created: time.Now()}}}
		if last > first {
			th.StartLine = first + 1
		}
		a.gd.rv.pending = append(a.gd.rv.pending, th)
		a.infoText(i18n.T("rv.commentadded"))
		return nil
	})
}

// commentFile starts a comment on the whole current file.
func (a *App) commentFile() tea.Cmd {
	p, t := a.reviewTarget()
	if err := a.checkCommentable(p, t); err != nil {
		a.fail(err)
		return nil
	}
	return a.compose(i18n.T("rv.filecommentlabel")+p, "", nil, func(body string) tea.Cmd {
		a.gd.rv.pending = append(a.gd.rv.pending, &prThread{Path: p, Side: "RIGHT", File: true, Pending: true,
			Comments: []prComment{{Author: i18n.T("rv.you"), Body: body, Created: time.Now()}}})
		a.infoText(i18n.T("rv.filecommentadded"))
		return nil
	})
}

// openThread opens the thread on the cursor line as a card, or lets the user
// pick one of the file's threads.
func (a *App) openThread() tea.Cmd {
	p, t := a.reviewTarget()
	if p == "" {
		a.fail(errors.New(i18n.T("rv.openfile")))
		return nil
	}
	if t != nil && a.focus != focusChanges && !a.isStale(p) {
		if ts := a.gd.rv.lineThreads(p, t.buf.Cur.Row+1); len(ts) == 1 {
			a.card = &prCard{thread: ts[0], path: p}
			return nil
		}
	}
	all := a.gd.rv.threadsIn(p)
	if len(all) == 0 {
		a.infoText(i18n.T("rv.nocomments"))
		return nil
	}
	if len(all) == 1 {
		a.card = &prCard{thread: all[0], path: p}
		return nil
	}
	items := make([]paletteItem, len(all))
	for i, th := range all {
		items[i] = paletteItem{label: threadWhere(th) + "  " + firstLine(th.Comments[0].Body), detail: "@" + th.Comments[0].Author + threadState(th)}
	}
	return a.pal.pick(i18n.T("rv.commentlabel")+p, items, func(i int) tea.Cmd {
		a.card = &prCard{thread: all[i], path: p}
		return nil
	})
}

// threadWhere names what a thread is about: "右 99 行目", "ファイル", ….
func threadWhere(t *prThread) string {
	side := i18n.T("rv.right")
	if t.Side == "LEFT" {
		side = i18n.T("rv.left")
	}
	switch {
	case t.File:
		return i18n.T("sym.file")
	case t.Outdated:
		return i18n.T("rv.outdated")
	case t.StartLine > 0:
		return fmt.Sprintf(i18n.T("rv.range"), side, t.StartLine, t.Line)
	}
	return fmt.Sprintf(i18n.T("rv.line"), side, t.Line)
}

func threadState(t *prThread) string {
	switch {
	case t.Pending:
		return i18n.T("rv.unsent")
	case t.Resolved:
		return i18n.T("rv.resolvedtag")
	}
	return ""
}

// viewedMsg reports the end of marking a file viewed or not.
type viewedMsg struct {
	num    int
	path   string
	viewed bool
	err    error
}

func (a *App) toggleViewed() tea.Cmd {
	p, _ := a.reviewTarget()
	if p == "" {
		a.fail(errors.New(i18n.T("rv.openfile")))
		return nil
	}
	if a.gd.pr.ID == "" {
		return nil
	}
	v := !a.gd.rv.viewed[p]
	root, pr := a.root, a.gd.pr
	return func() tea.Msg { return viewedMsg{num: pr.Number, path: p, viewed: v, err: setViewed(root, pr, p, v)} }
}

func (a *App) handleViewed(m viewedMsg) {
	if !a.prShown() || a.gd.pr.Number != m.num {
		return // another pull request is shown now
	}
	if m.err != nil {
		a.fail(m.err)
		return
	}
	if a.gd.rv.viewed == nil {
		a.gd.rv.viewed = map[string]bool{}
	}
	a.gd.rv.viewed[m.path] = m.viewed
	if m.viewed {
		a.info(i18n.T("rv.viewedon"), m.path)
	} else {
		a.info(i18n.T("rv.viewedoff"), m.path)
	}
}

// ---- sending ----

// reviewSentMsg reports the end of sending something to GitHub; the review
// is reloaded after it.
type reviewSentMsg struct {
	num  int    // the pull request it was sent to
	done string // message on success
	sent []*prThread
	err  error
}

func eventNames() map[string]string {
	return map[string]string{"APPROVE": i18n.T("rv.event.approve"), "REQUEST_CHANGES": i18n.T("rv.event.request"), "COMMENT": i18n.T("rv.event.comment")}
}

func (a *App) startSubmit(event string) tea.Cmd {
	if a.gd.pr.ID == "" {
		return nil
	}
	n := len(a.gd.rv.pending)
	title := fmt.Sprintf(i18n.T("rv.submitprompt"), eventNames()[event], n)
	cmd := a.compose(title, "", nil, func(body string) tea.Cmd {
		if event != "APPROVE" && body == "" && n == 0 {
			a.fail(errors.New(i18n.T("rv.needbody")))
			return nil
		}
		a.infoText(i18n.T("rv.submitting"))
		root, pr, threads := a.root, a.gd.pr, append([]*prThread(nil), a.gd.rv.pending...)
		return func() tea.Msg {
			err := submitReview(root, pr, event, body, threads)
			return reviewSentMsg{num: pr.Number, done: fmt.Sprintf(i18n.T("rv.submitted"), eventNames()[event]), sent: threads, err: err}
		}
	})
	a.composer.empty = true
	return cmd
}

func (a *App) handleReviewSent(m reviewSentMsg) tea.Cmd {
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	if m.num == a.gd.rv.num && len(m.sent) > 0 {
		var keep []*prThread
		for _, t := range a.gd.rv.pending {
			sent := false
			for _, s := range m.sent {
				sent = sent || s == t
			}
			if !sent {
				keep = append(keep, t)
			}
		}
		a.gd.rv.pending = keep
	}
	a.info("%s", m.done)
	if a.card != nil && a.card.thread.Pending {
		a.card = nil
	}
	if !a.prShown() || a.gd.pr.Number != m.num {
		return nil
	}
	return a.loadReview()
}

// ---- which lines take comments ----

// inHunk reports whether buffer line row lies in a hunk of the diff GitHub
// shows: a changed line, or one of the 3 lines of context around a change.
func (dv *diffView) inHunk(row int) bool {
	if dv.single {
		return row >= 0 && row < len(dv.newIdx) // the whole file is one added hunk
	}
	next := 0 // buffer line the rows so far end before
	for _, r := range dv.rows {
		if r.new >= 0 {
			next = r.new + 1
		}
		if !r.changed {
			continue
		}
		if r.new >= 0 {
			if row >= r.new-3 && row <= r.new+3 {
				return true
			}
		} else if row >= next-3 && row <= next+2 {
			// a deleted line sits between buffer lines next-1 and next
			return true
		}
	}
	return false
}

// ---- the comment card ----

// prCard is a thread shown like a hover popup. It takes the keys while open.
type prCard struct {
	thread    *prThread
	path      string
	scroll    int
	byPointer bool      // opened by resting the pointer: it takes no keys, like a hover
	at        *screenPt // the marker it hangs from when the mouse opened it
}

// cardDoc lays the thread out as markdown at width w (content only): for
// each comment a header line, then its body, with a rule between comments.
func (a *App) cardDoc(c *prCard, w int) mdDoc {
	t := c.thread
	var doc mdDoc
	for i, cm := range t.Comments {
		if i > 0 {
			doc.Lines = append(doc.Lines, mdRow(screen.Span{Text: strings.Repeat("─", w), Style: screen.StyleDim}))
		}
		head := []screen.Span{
			{Text: "@" + cm.Author, Style: screen.StyleStrong, Bold: true},
			{Text: "  " + relTime(cm.Created), Style: screen.StyleDim},
		}
		if i == 0 {
			head = append(head, screen.Span{Text: " · " + threadWhere(t) + threadState(t), Style: screen.StyleDim})
		}
		doc.Lines = append(doc.Lines, mdRow(cutSpans(head, 0, w)...))
		body := renderMarkdown(cm.Body, w, mdOpts{mermaid: a.mermaidLines})
		for _, im := range body.Images {
			im.Row += len(doc.Lines)
			doc.Images = append(doc.Images, im)
		}
		doc.Lines = append(doc.Lines, body.Lines...)
	}
	return doc
}

// cardOverlay is the open card c as a screen.Overlay: the rows in view when
// scrolled by c.scroll (the rest stays in core), at most maxH of them, w cells
// wide, with View telling where they are among all of them. It hangs on the
// line of the thread in the pane of its file (if that is the current one).
// Lines hold the card's content, header lines included. It clamps c.scroll
// and changes nothing else. Drawing it is drawCard's work.
func (a *App) cardOverlay(c *prCard, w, maxH int) screen.Overlay {
	doc := a.cardDoc(c, w)
	h := min(len(doc.Lines), maxH)
	c.scroll = clamp(c.scroll, 0, max(0, len(doc.Lines)-h))
	end := min(len(doc.Lines), c.scroll+h)
	ov := screen.Overlay{
		Kind: screen.OverlayReviewCard,
		View: &screen.Viewport{Top: c.scroll, Rows: h, Cols: w, Total: len(doc.Lines)},
		Hint: a.cardHint(c),
	}
	if t := a.curText(); t != nil && t.buf.Path != "" && a.repoPath(t.buf.Path) == c.path {
		ov.Pane = t.path()
		if c.thread.Line > 0 {
			ov.At = &screen.Pos{Line: c.thread.Line - 1}
		}
	}
	for i, l := range doc.Lines[c.scroll:end] {
		l.No = c.scroll + i
		ov.Lines = append(ov.Lines, l)
	}
	for _, im := range doc.Images {
		if im.Row < end && im.Row+im.Rows > c.scroll {
			ov.Images = append(ov.Images, im)
		}
	}
	return ov
}

func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case t.IsZero():
		return ""
	case d < time.Minute:
		return i18n.T("rv.justnow")
	case d < time.Hour:
		return fmt.Sprintf(i18n.T("rv.minago"), int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf(i18n.T("rv.hourago"), int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf(i18n.T("rv.dayago"), int(d.Hours()/24))
	}
	return t.Local().Format("2006-01-02")
}

// cardHint lists the card's keys.
func (a *App) cardHint(c *prCard) string {
	t := c.thread
	if t.Pending {
		return i18n.T("rv.hint.own")
	}
	resolve := i18n.T("rv.resolve")
	if t.Resolved {
		resolve = i18n.T("rv.unresolve")
	}
	s := ""
	if suggestion(t) != nil {
		s = i18n.T("rv.applysuggestion")
	}
	return i18n.T("rv.hint.reply1") + resolve + s + i18n.T("rv.hint.reply2")
}

// drawCard draws a review card Overlay as a bordered box ov.View.Cols+4
// cells wide: the content rows, the hint on the bottom border and, when the
// card scrolls, where the rows are among all of them.
func (a *App) drawCard(ov screen.Overlay) []string {
	vw := screen.Viewport{Cols: 1}
	if ov.View != nil {
		vw = *ov.View
	}
	w := vw.Cols + 4
	end := vw.Top + len(ov.Lines)
	bd := bgfg(cEditorBg, cHoverBorder)
	iw := w - 2
	out := []string{bd.Render("╭" + strings.Repeat("─", iw) + "╮")}
	pad := stEmpty.Render(" ")
	for _, l := range a.drawMdRows(ov.Lines, vw.Top, ov.Images, vw.Cols) {
		out = append(out, bd.Render("│")+pad+l+pad+bd.Render("│"))
	}
	ind := ""
	if vw.Total > vw.Rows {
		ind = fmt.Sprintf(" %d/%d ", end, vw.Total)
	}
	bottom := strings.Repeat("─", iw)
	if hw, in := ansi.StringWidth(ov.Hint), ansi.StringWidth(ind); hw+in+2 <= iw {
		bottom = "─" + ov.Hint + strings.Repeat("─", iw-hw-in-1) + ind
	}
	return append(out, bd.Render("╰"+bottom+"╯"))
}

// cardBox lays out the open card for layout l.
func (a *App) cardBox(l layout) (rows []string, x, y int) {
	c := a.card
	w := min(84, l.rightW-2)
	if w < 20 {
		return nil, 0, 0
	}
	rows = a.drawCard(a.cardOverlay(c, w-4, max(3, l.editH*2/3-2)))
	cx, cy := l.rightX+2, l.editY+1
	if c.at != nil {
		cx, cy = c.at.x, c.at.y
	} else if t := a.curText(); t != nil && t.buf.Path != "" && a.repoPath(t.buf.Path) == c.path {
		cx, cy = a.cursorScreen(t, l)
		if t.buf.Cur.Row+1 != c.thread.Line {
			cx, cy = l.rightX+2, l.editY+1
		}
	}
	x, y = popupPos(l, a.w, max(cx-4, l.rightX), cy, rows)
	return rows, x, y
}

func (a *App) handleCardKey(msg Key) (tea.Cmd, bool) {
	c := a.card
	t := c.thread
	switch key := msg.String(); key {
	case "esc", "q":
		a.card = nil
	case "j", "down", "ctrl+d", "pgdown":
		d := 1
		if key == "ctrl+d" || key == "pgdown" {
			d = 5
		}
		c.scroll += d
	case "k", "up", "ctrl+u", "pgup":
		d := 1
		if key == "ctrl+u" || key == "pgup" {
			d = 5
		}
		c.scroll = max(0, c.scroll-d)
	case "enter":
		a.card = nil
		return a.gotoThread(c), true
	case "r":
		if t.Pending {
			return a.compose(i18n.T("rv.editlabel")+threadWhere(t), t.Comments[0].Body, a.threadLines(c), func(body string) tea.Cmd {
				t.Comments[0].Body = body
				return nil
			}), true
		}
		return a.compose(i18n.T("rv.replylabel")+threadWhere(t), "", nil, func(body string) tea.Cmd {
			a.infoText(i18n.T("rv.replying"))
			root, id := a.root, t.ID
			num := a.gd.pr.Number
			return func() tea.Msg {
				return reviewSentMsg{num: num, done: i18n.T("rv.replied"), err: replyThread(root, id, body)}
			}
		}), true
	case "x":
		if t.Pending {
			for i, p := range a.gd.rv.pending {
				if p == t {
					a.gd.rv.pending = append(a.gd.rv.pending[:i], a.gd.rv.pending[i+1:]...)
					break
				}
			}
			a.card = nil
			a.infoText(i18n.T("rv.deletedunsent"))
			return nil, true
		}
		root, id, resolve := a.root, t.ID, !t.Resolved
		done := i18n.T("rv.resolved")
		if !resolve {
			done = i18n.T("rv.reopened")
		}
		num := a.gd.pr.Number
		return func() tea.Msg { return reviewSentMsg{num: num, done: done, err: resolveThread(root, id, resolve)} }, true
	case "s":
		if t.Pending || suggestion(t) == nil {
			return nil, true
		}
		return a.applySuggestion(c), true
	default:
		a.card = nil
		return nil, false // the key goes on to whatever has the focus
	}
	return nil, true
}

// gotoThread opens the thread's file with the cursor on its line.
func (a *App) gotoThread(c *prCard) tea.Cmd {
	abs := filepath.Join(a.gitTopDir(), c.path)
	if !a.openFile(abs) {
		return nil
	}
	// base side and outdated lines count in another version of the file
	if t := a.curText(); t != nil && c.thread.onLine(c.thread.Line) {
		t.buf.Anchor = nil
		t.buf.Cur = t.buf.ClampPos(buffer.Pos{Row: c.thread.Line - 1})
		a.follow = true
	}
	return a.setFocus(focusEditor)
}

// threadLines returns the lines a thread is on, from the open buffer.
func (a *App) threadLines(c *prCard) []string {
	t := c.thread
	if t.File || t.Line <= 0 {
		return nil
	}
	abs := filepath.Join(a.gitTopDir(), c.path)
	for _, tb := range a.tabs {
		if tb.path() == abs && tb.kind == tabText {
			var out []string
			for r := t.first() - 1; r < t.Line && r < len(tb.buf.Lines); r++ {
				out = append(out, string(tb.buf.Lines[r]))
			}
			return out
		}
	}
	return nil
}

// suggestion returns the lines of the last ```suggestion block in the
// thread, or nil.
func suggestion(t *prThread) []string {
	if t.File || t.Outdated || t.Side != "RIGHT" || t.Line <= 0 {
		return nil
	}
	for _, v := range slices.Backward(t.Comments) {
		var block []string
		in, found := false, false
		for l := range strings.SplitSeq(strings.ReplaceAll(v.Body, "\r\n", "\n"), "\n") {
			trim := strings.TrimSpace(l)
			switch {
			case !in && strings.HasPrefix(trim, "```suggestion"):
				in, block = true, []string{}
			case in && strings.HasPrefix(trim, "```"):
				in, found = false, true
				if block == nil {
					block = []string{}
				}
			case in:
				block = append(block, l)
			}
		}
		if found {
			if len(block) == 0 {
				return []string{} // deletes the lines
			}
			return block
		}
	}
	return nil
}

// applySuggestion replaces the thread's lines with its suggestion and saves.
func (a *App) applySuggestion(c *prCard) tea.Cmd {
	if err := a.checkCommentable(c.path, nil); err != nil {
		a.fail(errors.New(i18n.T("rv.sugglocal")))
		return nil
	}
	lines := suggestion(c.thread)
	if !a.openFile(filepath.Join(a.gitTopDir(), c.path)) {
		return nil
	}
	t := a.curText()
	b := t.buf
	if t.dirty() {
		a.fail(errors.New(i18n.T("rv.suggunsaved")))
		return nil
	}
	first, last := c.thread.first()-1, c.thread.Line-1
	if first < 0 || last >= len(b.Lines) {
		a.fail(errors.New(i18n.T("rv.nolinefile")))
		return nil
	}
	start, end := buffer.Pos{Row: first}, buffer.Pos{Row: last, Col: len(b.Lines[last])}
	text := strings.Join(lines, "\n")
	if len(lines) == 0 {
		// remove the lines with their line break
		if last+1 < len(b.Lines) {
			end = buffer.Pos{Row: last + 1}
		} else if first > 0 {
			start = buffer.Pos{Row: first - 1, Col: len(b.Lines[first-1])}
		}
	}
	b.ApplyEdits([]buffer.Edit{{Start: start, End: end, Text: text}})
	b.Anchor = nil
	b.Cur = b.ClampPos(buffer.Pos{Row: first})
	a.card = nil
	a.follow = true
	cmd := a.saveTab(t)
	if !a.msgErr {
		a.info(i18n.T("rv.suggapplied"), c.path)
	}
	return tea.Batch(cmd, a.setFocus(focusEditor))
}

// ---- the comment editor ----

// composer is the input for a comment, a reply or a review body, with a
// rendered preview under it.
type composer struct {
	title    string
	ta       textarea.Model
	orig     []string // lines a suggestion starts from (Ctrl+S); nil = none
	empty    bool     // an empty text may be sent
	onSubmit func(string) tea.Cmd
}

// compose opens the comment editor. orig is what Ctrl+S puts in a
// suggestion block.
func (a *App) compose(title, initial string, orig []string, onSubmit func(string) tea.Cmd) tea.Cmd {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.SetHeight(6)
	st := ta.Styles()
	base := bgfg(cPaletteBg, cEditorFg)
	st.Focused = textarea.StyleState{Base: base, Text: base, CursorLine: base, Placeholder: bgfg(cPaletteBg, curTheme.Dim), EndOfBuffer: base, Prompt: base}
	st.Blurred = st.Focused
	ta.SetStyles(st)
	ta.Placeholder = i18n.T("rv.markdownok")
	ta.SetValue(initial)
	a.comp, a.hoverText = nil, ""
	a.composer = &composer{title: title, ta: ta, orig: orig, onSubmit: onSubmit}
	return a.composer.ta.Focus()
}

func (a *App) handleComposerKey(msg Key) tea.Cmd {
	c := a.composer
	switch msg.String() {
	case "esc":
		a.composer = nil
		a.infoText(i18n.T("rv.inputcancelled"))
		return nil
	case "enter":
		body := strings.TrimSpace(c.ta.Value())
		if body == "" && !c.empty {
			return nil
		}
		a.composer = nil
		return c.onSubmit(body)
	case "shift+enter", "alt+enter", "ctrl+j":
		c.ta.InsertString("\n")
		return nil
	case "ctrl+s":
		if c.orig == nil {
			a.infoText(i18n.T("rv.suggestonline"))
			return nil
		}
		block := "```suggestion\n" + strings.Join(c.orig, "\n") + "\n```\n"
		if c.ta.Column() > 0 {
			block = "\n" + block // a fence must start a line
		}
		c.ta.InsertString(block)
		return nil
	}
	var cmd tea.Cmd
	c.ta, cmd = c.ta.Update(widgetMsg(msg))
	return cmd
}

// composerOverlay is the open comment editor as a screen.Overlay, iw cells
// wide inside its border: Title, Hint, the text and cursor in Compose, and
// in Lines the rendered preview of the text (the first previewRows-1 rows; the
// label takes the first row). Compose.PreviewLabel is empty when previewRows
// is 0, as there is no room. It changes nothing. Drawing it is drawComposer's
// work.
func (a *App) composerOverlay(iw, previewRows int) screen.Overlay {
	c := a.composer
	ov := screen.Overlay{
		Kind:    screen.OverlayReviewCard,
		Title:   c.title,
		Hint:    i18n.T("rv.hint.input"),
		View:    &screen.Viewport{Rows: max(0, previewRows), Cols: iw},
		Compose: &screen.Compose{Text: c.ta.Value(), Cursor: screen.Pos{Line: c.ta.Line(), Col: c.ta.Column()}},
	}
	if c.orig != nil {
		ov.Hint = i18n.T("rv.hint.inputsugg")
	}
	if previewRows <= 0 {
		return ov
	}
	ov.Compose.PreviewLabel = i18n.T("rv.preview")
	if strings.TrimSpace(ov.Compose.Text) == "" {
		return ov
	}
	doc := renderMarkdown(ov.Compose.Text, iw, mdOpts{mermaid: a.mermaidLines})
	n := min(len(doc.Lines), previewRows-1)
	for i, l := range doc.Lines[:n] {
		l.No = i
		ov.Lines = append(ov.Lines, l)
	}
	for _, im := range doc.Images {
		if im.Row < n {
			ov.Images = append(ov.Images, im)
		}
	}
	return ov
}

// composerBox draws the editor over the bottom of the editor area.
func (a *App) composerBox(l layout) (rows []string, x, y int) {
	c := a.composer
	w := min(96, l.rightW-2)
	if w < 24 {
		w = max(10, l.rightW)
	}
	iw := w - 4
	c.ta.SetWidth(iw)
	input := strings.Split(c.ta.View(), "\n")
	prevH := max(0, l.editH-(len(input)+2)-1)
	rows = a.drawComposer(a.composerOverlay(iw, min(prevH, 12)), input)
	x = l.rightX + max(0, (l.rightW-w)/2)
	y = max(l.editY, l.editY+l.editH-len(rows))
	return rows, x, y
}

// drawComposer draws a comment editor Overlay as a bordered box ov.View.Cols+4
// cells wide. input is the text field as the widget draws it (its scrolling
// and cursor), which stays until the widget is replaced by our own type.
func (a *App) drawComposer(ov screen.Overlay, input []string) []string {
	iw := ov.View.Cols
	w := iw + 4
	bd := bgfg(cPaletteBg, cHoverBorder)
	in := bgfg(cPaletteBg, cEditorFg)
	line := func(s string) string {
		return bd.Render("│") + in.Render(" ") + fitStyled(s, iw, in) + in.Render(" ") + bd.Render("│")
	}
	title := " " + ov.Title + " "
	top := "╭─" + ansi.Truncate(title, w-4, "…")
	top += strings.Repeat("─", max(0, w-1-ansi.StringWidth(top))) + "╮"
	rows := []string{bd.Render(top)}
	for _, l := range input {
		rows = append(rows, line(l))
	}
	rows = append(rows, bd.Render("├"+fit(ov.Hint, w-2)+"┤"))
	if label := ov.Compose.PreviewLabel; label != "" {
		prev := append([]string{stPanelDim.Render(fit(label, iw))}, a.drawMdRows(ov.Lines, 0, ov.Images, iw)...)
		for _, p := range prev {
			rows = append(rows, bd.Render("│")+stEmpty.Render(" ")+fitStyled(p, iw, stEmpty)+stEmpty.Render(" ")+bd.Render("│"))
		}
	}
	return append(rows, bd.Render("╰"+strings.Repeat("─", w-2)+"╯"))
}

// reviewMermaid returns the mermaid blocks of the open card and editor.
func (a *App) reviewMermaid() []string {
	var out []string
	if c := a.card; c != nil {
		for _, cm := range c.thread.Comments {
			out = append(out, mermaidBlocks(cm.Body)...)
		}
	}
	if c := a.composer; c != nil {
		out = append(out, mermaidBlocks(c.ta.Value())...)
	}
	return out
}

// ---- the mouse ----

// markerAt returns the thread whose marker is at screen cell (x, y), if any.
func (a *App) markerAt(l layout, x, y int) *prThread {
	t := a.curText()
	if t == nil || !a.prShown() {
		return nil
	}
	dv := a.diffFor(t)
	if dv == nil {
		return nil
	}
	g := a.dvGeom(l)
	if x != g.rightX || y < g.top || y >= g.top+g.h {
		return nil
	}
	idx := dv.start(t.buf.ScrollRow) + y - g.top
	if idx >= len(dv.rows) || dv.rows[idx].new < 0 {
		return nil
	}
	p := a.repoPath(t.buf.Path)
	if a.isStale(p) {
		return nil
	}
	if ts := a.gd.rv.lineThreads(p, dv.rows[idx].new+1); len(ts) > 0 {
		return ts[0]
	}
	return nil
}

// reviewNote is what the file name line of a diff says about the review of
// t's file: whether it differs from the pull request, and its comments
// that have no line to sit on.
func (a *App) reviewNote(t *tab) string {
	if !a.prShown() {
		return ""
	}
	p := a.repoPath(t.buf.Path)
	if a.isStale(p) {
		s := i18n.T("rv.localdiff")
		if n := len(a.gd.rv.threadsIn(p)); n > 0 {
			s += fmt.Sprintf(i18n.T("rv.threadcount"), n)
		}
		return s
	}
	if n := len(a.gd.rv.fileThreads(p)); n > 0 {
		return fmt.Sprintf(i18n.T("rv.filethreadcount"), n)
	}
	return ""
}

// composerCaret is the screen cell of the comment editor's cursor, where an
// IME shows its composition.
func (a *App) composerCaret(l layout) (x, y int, ok bool) {
	rows, bx, by := a.composerBox(l)
	ta := a.composer.ta
	ta.SetVirtualCursor(false)
	c := ta.Cursor()
	if c == nil {
		return 0, 0, false
	}
	x, y = bx+2+c.Position.X, by+1+c.Position.Y
	return x, y, y < by+len(rows) && x < a.w
}
