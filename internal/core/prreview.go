package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	threads []*PrThread // from GitHub
	pending []*PrThread // written here, sent with the review
	viewed  map[string]bool
	sync    prSync
	seq     int // bumped on every load; older answers are dropped
}

// yesNo is a question answered with y or n on the status line.
type yesNo struct {
	prompt  string
	yes, no func() []Effect
}

func (a *Core) handleConfirmKey(key string) []Effect {
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
func (a *Core) loadReview() []Effect {
	pr := a.gd.pr
	if pr == nil {
		return nil
	}
	a.gd.rv.seq++
	root, seq := a.root, a.gd.rv.seq
	return async(func() Event {
		fetchPR(root, pr)
		m := prReviewMsg{seq: seq, num: pr.Number, sync: syncWith(root, pr.HeadRefOid)}
		if pr.ID != "" {
			m.data, m.err = loadReview(root, pr)
		}
		return m
	})
}

func (a *Core) handlePRReview(m prReviewMsg) []Effect {
	rv := &a.gd.rv
	if m.seq != rv.seq || !a.PrShown() || a.gd.pr.Number != m.num {
		return nil
	}
	rv.sync = m.sync
	if m.err != nil {
		a.Fail(m.err)
	} else {
		rv.threads, rv.viewed = m.data.threads, m.data.viewed
		if c := a.card; c != nil && !c.Thread.Pending {
			a.card = nil
			for _, t := range rv.threads {
				if t.ID == c.Thread.ID {
					a.card = &prCard{Thread: t, Path: c.Path}
				}
			}
		}
	}
	return a.reloadChanges() // the head commit may have just arrived
}

// prShown reports whether diff mode compares with a pull request's base.
func (a *Core) PrShown() bool {
	return a.gd.on && a.gd.base == basePR && a.gd.pr != nil
}

// ---- files ----

// repoPath is abs relative to the repository top, as GitHub names files.
func (a *Core) RepoPath(abs string) string {
	if r, err := filepath.Rel(a.gitTopDir(), abs); err == nil {
		return filepath.ToSlash(r)
	}
	return abs
}

// isStale reports whether file p (from the repository top) differs from the
// pull request's head commit, so its line numbers may not match GitHub's.
func (a *Core) IsStale(p string) bool {
	return a.gd.staleAll || a.gd.stale[p]
}

// threadsIn returns the threads of file p, sent ones first.
func (rv *prReview) threadsIn(p string) []*PrThread {
	var out []*PrThread
	for _, ts := range [][]*PrThread{rv.threads, rv.pending} {
		for _, t := range ts {
			if t.Path == p {
				out = append(out, t)
			}
		}
	}
	return out
}

// lineThreads returns the threads shown at line (1-based) of file p.
func (rv *prReview) lineThreads(p string, line int) []*PrThread {
	var out []*PrThread
	for _, t := range rv.threadsIn(p) {
		if t.onLine(line) {
			out = append(out, t)
		}
	}
	return out
}

// fileThreads returns the comments on file p as a whole.
func (rv *prReview) fileThreads(p string) []*PrThread {
	var out []*PrThread
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
func (a *Core) CommentMark(t *Tab) func(row int) string {
	if !a.PrShown() || t.Buf.Path == "" {
		return nil
	}
	p := a.RepoPath(t.Buf.Path)
	if a.IsStale(p) {
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
			return CurTheme.Renamed
		}
		return ""
	}
}

// ---- the Space v menu ----

var ReviewItems = []SpaceItem{
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
func (a *Core) openReviewMenu() []Effect {
	if !a.PrShown() {
		a.InfoText(i18n.T("rv.needpr"))
		return nil
	}
	a.pending = "space v"
	return nil
}

// reviewCommand runs Space v <key> from the command palette.
func (a *Core) reviewCommand(key string) []Effect {
	if !a.PrShown() {
		return a.openReviewMenu() // says where reviewing works
	}
	return a.reviewKey(key)
}

// reviewKey runs the command of Space v <key>.
func (a *Core) reviewKey(key string) []Effect {
	if !a.PrShown() {
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
func (a *Core) reviewTarget() (string, *Tab) {
	if a.focus == FocusChanges {
		if f := a.gd.file(); f != nil {
			abs := filepath.Join(a.gitTopDir(), f.Path())
			for _, t := range a.tabs {
				if t.Path() == abs {
					return f.Path(), t
				}
			}
			return f.Path(), nil
		}
		return "", nil
	}
	if t := a.CurText(); t != nil && t.Buf.Path != "" {
		return a.RepoPath(t.Buf.Path), t
	}
	return "", nil
}

// checkCommentable reports why file p of tab t cannot take comments, or nil.
func (a *Core) checkCommentable(p string, t *Tab) error {
	if p == "" || a.gd.byAbs[filepath.Join(a.gitTopDir(), p)] == nil {
		return errors.New(i18n.T("rv.needchanged"))
	}
	if a.IsStale(p) {
		return errors.New(i18n.T("rv.localchanged"))
	}
	if t != nil && t.dirty() {
		return errors.New(i18n.T("rv.unsaved"))
	}
	return nil
}

// commentLines starts a comment on the cursor line or the selected lines.
func (a *Core) commentLines() []Effect {
	p, t := a.reviewTarget()
	if a.focus == FocusChanges || t == nil {
		a.Fail(errors.New(i18n.T("rv.selectline")))
		return nil
	}
	if err := a.checkCommentable(p, t); err != nil {
		a.Fail(err)
		return nil
	}
	b := t.Buf
	first, last := b.Cur.Row, b.Cur.Row
	if s, e, ok := b.Selection(); ok {
		first, last = s.Row, e.Row
		if e.Col == 0 && e.Row > s.Row {
			last-- // a whole-line selection ends at the start of the next line
		}
	}
	if dv := a.DiffFor(t); dv == nil || !dv.inHunk(first) || !dv.inHunk(last) {
		a.Fail(errors.New(i18n.T("rv.outofdiff")))
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
	return a.compose(i18n.T("rv.commentlabel")+where, "", orig, func(body string) []Effect {
		th := &PrThread{Path: p, Line: last + 1, Side: "RIGHT", Pending: true, Comments: []prComment{{Author: i18n.T("rv.you"), Body: body, Created: time.Now()}}}
		if last > first {
			th.StartLine = first + 1
		}
		a.gd.rv.pending = append(a.gd.rv.pending, th)
		a.InfoText(i18n.T("rv.commentadded"))
		return nil
	})
}

// commentFile starts a comment on the whole current file.
func (a *Core) commentFile() []Effect {
	p, t := a.reviewTarget()
	if err := a.checkCommentable(p, t); err != nil {
		a.Fail(err)
		return nil
	}
	return a.compose(i18n.T("rv.filecommentlabel")+p, "", nil, func(body string) []Effect {
		a.gd.rv.pending = append(a.gd.rv.pending, &PrThread{Path: p, Side: "RIGHT", File: true, Pending: true,
			Comments: []prComment{{Author: i18n.T("rv.you"), Body: body, Created: time.Now()}}})
		a.InfoText(i18n.T("rv.filecommentadded"))
		return nil
	})
}

// openThread opens the thread on the cursor line as a card, or lets the user
// pick one of the file's threads.
func (a *Core) openThread() []Effect {
	p, t := a.reviewTarget()
	if p == "" {
		a.Fail(errors.New(i18n.T("rv.openfile")))
		return nil
	}
	if t != nil && a.focus != FocusChanges && !a.IsStale(p) {
		if ts := a.gd.rv.lineThreads(p, t.Buf.Cur.Row+1); len(ts) == 1 {
			a.card = &prCard{Thread: ts[0], Path: p}
			return nil
		}
	}
	all := a.gd.rv.threadsIn(p)
	if len(all) == 0 {
		a.InfoText(i18n.T("rv.nocomments"))
		return nil
	}
	if len(all) == 1 {
		a.card = &prCard{Thread: all[0], Path: p}
		return nil
	}
	items := make([]paletteItem, len(all))
	for i, th := range all {
		items[i] = paletteItem{label: threadWhere(th) + "  " + FirstLine(th.Comments[0].Body), detail: "@" + th.Comments[0].Author + threadState(th)}
	}
	return a.pal.pick(i18n.T("rv.commentlabel")+p, items, func(i int) []Effect {
		a.card = &prCard{Thread: all[i], Path: p}
		return nil
	})
}

// threadWhere names what a thread is about: "右 99 行目", "ファイル", ….
func threadWhere(t *PrThread) string {
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

func threadState(t *PrThread) string {
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

func (a *Core) toggleViewed() []Effect {
	p, _ := a.reviewTarget()
	if p == "" {
		a.Fail(errors.New(i18n.T("rv.openfile")))
		return nil
	}
	if a.gd.pr.ID == "" {
		return nil
	}
	v := !a.gd.rv.viewed[p]
	root, pr := a.root, a.gd.pr
	return async(func() Event { return viewedMsg{num: pr.Number, path: p, viewed: v, err: setViewed(root, pr, p, v)} })
}

func (a *Core) handleViewed(m viewedMsg) {
	if !a.PrShown() || a.gd.pr.Number != m.num {
		return // another pull request is shown now
	}
	if m.err != nil {
		a.Fail(m.err)
		return
	}
	if a.gd.rv.viewed == nil {
		a.gd.rv.viewed = map[string]bool{}
	}
	a.gd.rv.viewed[m.path] = m.viewed
	if m.viewed {
		a.Info(i18n.T("rv.viewedon"), m.path)
	} else {
		a.Info(i18n.T("rv.viewedoff"), m.path)
	}
}

// ---- sending ----

// reviewSentMsg reports the end of sending something to GitHub; the review
// is reloaded after it.
type reviewSentMsg struct {
	num  int    // the pull request it was sent to
	done string // message on success
	sent []*PrThread
	err  error
}

func eventNames() map[string]string {
	return map[string]string{"APPROVE": i18n.T("rv.event.approve"), "REQUEST_CHANGES": i18n.T("rv.event.request"), "COMMENT": i18n.T("rv.event.comment")}
}

func (a *Core) startSubmit(event string) []Effect {
	if a.gd.pr.ID == "" {
		return nil
	}
	n := len(a.gd.rv.pending)
	title := fmt.Sprintf(i18n.T("rv.submitprompt"), eventNames()[event], n)
	cmd := a.compose(title, "", nil, func(body string) []Effect {
		if event != "APPROVE" && body == "" && n == 0 {
			a.Fail(errors.New(i18n.T("rv.needbody")))
			return nil
		}
		a.InfoText(i18n.T("rv.submitting"))
		root, pr, threads := a.root, a.gd.pr, append([]*PrThread(nil), a.gd.rv.pending...)
		return async(func() Event {
			err := submitReview(root, pr, event, body, threads)
			return reviewSentMsg{num: pr.Number, done: fmt.Sprintf(i18n.T("rv.submitted"), eventNames()[event]), sent: threads, err: err}
		})
	})
	a.composer.empty = true
	return cmd
}

func (a *Core) handleReviewSent(m reviewSentMsg) []Effect {
	if m.err != nil {
		a.Fail(m.err)
		return nil
	}
	if m.num == a.gd.rv.num && len(m.sent) > 0 {
		var keep []*PrThread
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
	a.Info("%s", m.done)
	if a.card != nil && a.card.Thread.Pending {
		a.card = nil
	}
	if !a.PrShown() || a.gd.pr.Number != m.num {
		return nil
	}
	return a.loadReview()
}

// ---- which lines take comments ----

// inHunk reports whether buffer line row lies in a hunk of the diff GitHub
// shows: a changed line, or one of the 3 lines of context around a change.
func (dv *DiffView) inHunk(row int) bool {
	if dv.Single {
		return row >= 0 && row < len(dv.NewIdx) // the whole file is one added hunk
	}
	next := 0 // buffer line the rows so far end before
	for _, r := range dv.Rows {
		if r.New >= 0 {
			next = r.New + 1
		}
		if !r.changed {
			continue
		}
		if r.New >= 0 {
			if row >= r.New-3 && row <= r.New+3 {
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
	Thread    *PrThread
	Path      string
	scroll    int
	byPointer bool      // opened by resting the pointer: it takes no keys, like a hover
	At        *ScreenPt // the marker it hangs from when the mouse opened it
}

// cardDoc lays the thread out as markdown at width w (content only): for
// each comment a header line, then its body, with a rule between comments.
func (a *Core) cardDoc(c *prCard, w int) MdDoc {
	t := c.Thread
	var doc MdDoc
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
		body := RenderMarkdown(cm.Body, w, MdOpts{mermaid: a.mermaidLines})
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
func (a *Core) CardOverlay(c *prCard, w, maxH int) screen.Overlay {
	doc := a.cardDoc(c, w)
	h := min(len(doc.Lines), maxH)
	c.scroll = Clamp(c.scroll, 0, max(0, len(doc.Lines)-h))
	end := min(len(doc.Lines), c.scroll+h)
	ov := screen.Overlay{
		Kind: screen.OverlayReviewCard,
		View: &screen.Viewport{Top: c.scroll, Rows: h, Cols: w, Total: len(doc.Lines)},
		Hint: a.cardHint(c),
	}
	if t := a.CurText(); t != nil && t.Buf.Path != "" && a.RepoPath(t.Buf.Path) == c.Path {
		ov.Pane = t.Path()
		if c.Thread.Line > 0 {
			ov.At = &screen.Pos{Line: c.Thread.Line - 1}
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
func (a *Core) cardHint(c *prCard) string {
	t := c.Thread
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

func (a *Core) handleCardKey(msg Key) ([]Effect, bool) {
	c := a.card
	t := c.Thread
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
			return a.compose(i18n.T("rv.editlabel")+threadWhere(t), t.Comments[0].Body, a.threadLines(c), func(body string) []Effect {
				t.Comments[0].Body = body
				return nil
			}), true
		}
		return a.compose(i18n.T("rv.replylabel")+threadWhere(t), "", nil, func(body string) []Effect {
			a.InfoText(i18n.T("rv.replying"))
			root, id := a.root, t.ID
			num := a.gd.pr.Number
			return async(func() Event {
				return reviewSentMsg{num: num, done: i18n.T("rv.replied"), err: replyThread(root, id, body)}
			})
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
			a.InfoText(i18n.T("rv.deletedunsent"))
			return nil, true
		}
		root, id, resolve := a.root, t.ID, !t.Resolved
		done := i18n.T("rv.resolved")
		if !resolve {
			done = i18n.T("rv.reopened")
		}
		num := a.gd.pr.Number
		return async(func() Event { return reviewSentMsg{num: num, done: done, err: resolveThread(root, id, resolve)} }), true
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
func (a *Core) gotoThread(c *prCard) []Effect {
	abs := filepath.Join(a.gitTopDir(), c.Path)
	if !a.openFile(abs) {
		return nil
	}
	// base side and outdated lines count in another version of the file
	if t := a.CurText(); t != nil && c.Thread.onLine(c.Thread.Line) {
		t.Buf.Anchor = nil
		t.Buf.Cur = t.Buf.ClampPos(buffer.Pos{Row: c.Thread.Line - 1})
		a.follow = true
	}
	return a.SetFocus(FocusEditor)
}

// threadLines returns the lines a thread is on, from the open buffer.
func (a *Core) threadLines(c *prCard) []string {
	t := c.Thread
	if t.File || t.Line <= 0 {
		return nil
	}
	abs := filepath.Join(a.gitTopDir(), c.Path)
	for _, tb := range a.tabs {
		if tb.Path() == abs && tb.Kind == TabText {
			var out []string
			for r := t.first() - 1; r < t.Line && r < len(tb.Buf.Lines); r++ {
				out = append(out, string(tb.Buf.Lines[r]))
			}
			return out
		}
	}
	return nil
}

// suggestion returns the lines of the last ```suggestion block in the
// thread, or nil.
func suggestion(t *PrThread) []string {
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
func (a *Core) applySuggestion(c *prCard) []Effect {
	if err := a.checkCommentable(c.Path, nil); err != nil {
		a.Fail(errors.New(i18n.T("rv.sugglocal")))
		return nil
	}
	lines := suggestion(c.Thread)
	if !a.openFile(filepath.Join(a.gitTopDir(), c.Path)) {
		return nil
	}
	t := a.CurText()
	b := t.Buf
	if t.dirty() {
		a.Fail(errors.New(i18n.T("rv.suggunsaved")))
		return nil
	}
	first, last := c.Thread.first()-1, c.Thread.Line-1
	if first < 0 || last >= len(b.Lines) {
		a.Fail(errors.New(i18n.T("rv.nolinefile")))
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
		a.Info(i18n.T("rv.suggapplied"), c.Path)
	}
	return Batch(cmd, a.SetFocus(FocusEditor))
}

// ---- the comment editor ----

// composer is the input for a comment, a reply or a review body, with a
// rendered preview under it.
type composer struct {
	title    string
	Ta       TextArea
	orig     []string // lines a suggestion starts from (Ctrl+S); nil = none
	empty    bool     // an empty text may be sent
	onSubmit func(string) []Effect
}

// compose opens the comment editor. orig is what Ctrl+S puts in a
// suggestion block.
func (a *Core) compose(title, initial string, orig []string, onSubmit func(string) []Effect) []Effect {
	ta := a.NewTextArea(initial)
	a.comp, a.hoverText = nil, ""
	a.composer = &composer{title: title, Ta: ta, orig: orig, onSubmit: onSubmit}
	return nil
}

func (a *Core) handleComposerKey(msg Key) []Effect {
	c := a.composer
	switch msg.String() {
	case "esc":
		a.composer = nil
		a.InfoText(i18n.T("rv.inputcancelled"))
		return nil
	case "enter":
		body := strings.TrimSpace(c.Ta.Value())
		if body == "" && !c.empty {
			return nil
		}
		a.composer = nil
		return c.onSubmit(body)
	case "shift+enter", "alt+enter", "ctrl+j":
		c.Ta.InsertString("\n")
		return nil
	case "ctrl+s":
		if c.orig == nil {
			a.InfoText(i18n.T("rv.suggestonline"))
			return nil
		}
		block := "```suggestion\n" + strings.Join(c.orig, "\n") + "\n```\n"
		if c.Ta.Column() > 0 {
			block = "\n" + block // a fence must start a line
		}
		c.Ta.InsertString(block)
		return nil
	}
	c.Ta.Update(msg)
	return nil
}

// composerOverlay is the open comment editor as a screen.Overlay, iw cells
// wide inside its border: Title, Hint, the text and cursor in Compose, and
// in Lines the rendered preview of the text (the first previewRows-1 rows; the
// label takes the first row). Compose.PreviewLabel is empty when previewRows
// is 0, as there is no room. It changes nothing. Drawing it is drawComposer's
// work.
func (a *Core) ComposerOverlay(iw, previewRows int) screen.Overlay {
	c := a.composer
	ov := screen.Overlay{
		Kind:    screen.OverlayReviewCard,
		Title:   c.title,
		Hint:    i18n.T("rv.hint.input"),
		View:    &screen.Viewport{Rows: max(0, previewRows), Cols: iw},
		Compose: &screen.Compose{Text: c.Ta.Value(), Cursor: screen.Pos{Line: c.Ta.Line(), Col: c.Ta.Column()}},
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
	doc := RenderMarkdown(ov.Compose.Text, iw, MdOpts{mermaid: a.mermaidLines})
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

// reviewMermaid returns the mermaid blocks of the open card and editor.
func (a *Core) reviewMermaid() []string {
	var out []string
	if c := a.card; c != nil {
		for _, cm := range c.Thread.Comments {
			out = append(out, mermaidBlocks(cm.Body)...)
		}
	}
	if c := a.composer; c != nil {
		out = append(out, mermaidBlocks(c.Ta.Value())...)
	}
	return out
}

// ---- the mouse ----

// reviewNote is what the file name line of a diff says about the review of
// t's file: whether it differs from the pull request, and its comments
// that have no line to sit on.
func (a *Core) reviewNote(t *Tab) string {
	if !a.PrShown() {
		return ""
	}
	p := a.RepoPath(t.Buf.Path)
	if a.IsStale(p) {
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
