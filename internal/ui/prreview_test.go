package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// prApp makes a repository whose branch "feature" is the head of pull
// request #7 against "main": a.txt (lines 1..20) has line 10 changed and
// b.txt is added. The working tree equals the head commit, and diff mode
// compares with the pull request's base.
func prApp(t *testing.T) (*App, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, "line"+string(rune('a'+i-1)))
	}
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	lines[9] = "changed"
	os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b1\nb2\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "change")
	a := New(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	pr := &prInfo{ID: "PR_1", Number: 7, Title: "t", State: "OPEN", BaseRefName: "main", HeadRefName: "feature", HeadRefOid: gitOut(t, dir, "rev-parse", "HEAD")}
	a.showDiff(basePR, pr)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, pr).(changesLoadedMsg))
	a.focus = focusEditor
	return a, file
}

func TestStaleFilesFollowTheWorkingTree(t *testing.T) {
	a, file := prApp(t)
	if a.gd.staleAll || len(a.gd.stale) != 0 {
		t.Fatalf("clean tree: staleAll=%v stale=%v", a.gd.staleAll, a.gd.stale)
	}
	// an edit saved to a.txt makes it stale; undoing it makes it clean again
	orig, _ := os.ReadFile(file)
	os.WriteFile(file, append([]byte("top\n"), orig...), 0o644)
	os.WriteFile(filepath.Join(a.root, "new.txt"), []byte("n\n"), 0o644)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	if !a.isStale("a.txt") || !a.isStale("new.txt") || a.isStale("b.txt") {
		t.Errorf("after edit: %v", a.gd.stale)
	}
	os.WriteFile(file, orig, 0o644)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	if a.isStale("a.txt") {
		t.Error("a.txt still stale after undoing the edit")
	}
	// a head commit that is not here: every file is stale
	pr := *a.gd.pr
	pr.HeadRefOid = strings.Repeat("1", 40)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, &pr).(changesLoadedMsg))
	if !a.gd.staleAll || !a.isStale("b.txt") {
		t.Error("missing head commit did not make files stale")
	}
}

func TestStaleFileShowsMarks(t *testing.T) {
	a, file := prApp(t)
	a.Update(tea.WindowSizeMsg{Width: 220, Height: 40})
	a.sideWPref = 40
	a.gd.rv.threads = []*prThread{
		{ID: "T1", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "rev", Body: "hmm"}}},
		{ID: "T2", Path: "a.txt", Side: "RIGHT", File: true, Comments: []prComment{{Author: "rev", Body: "file"}}},
	}
	a.gd.rv.viewed = map[string]bool{"b.txt": true}
	view := ansi.Strip(a.render())
	for _, s := range []string{"● ファイルへのコメント 1", "viewed 1/2", "●2"} {
		if !strings.Contains(view, s) {
			t.Errorf("clean: view lacks %q", s)
		}
	}
	if mark := a.commentMark(a.curText()); mark == nil || mark(9) == "" || mark(8) != "" {
		t.Error("no marker on line 10")
	}
	orig, _ := os.ReadFile(file)
	os.WriteFile(file, append(orig, []byte("more\n")...), 0o644)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	view = ansi.Strip(a.render())
	for _, s := range []string{"a.txt ✎", "✎ 手元で変更あり（PR の最新コミットと違う）", "● コメント 2（行には表示しません。Space v t で一覧）"} {
		if !strings.Contains(view, s) {
			t.Errorf("stale: view lacks %q", s)
		}
	}
	if a.commentMark(a.curText()) != nil {
		t.Error("markers drawn on a stale file")
	}
}

func TestCommentOnLines(t *testing.T) {
	a, _ := prApp(t)
	b := a.curText().buf
	// far from the change: GitHub would refuse it
	b.Cur = buffer.Pos{Row: 0}
	a.reviewKey("c")
	if a.composer != nil || !a.msgErr {
		t.Fatal("comment outside the hunk was accepted")
	}
	// lines 9..11 selected (Helix x on 9, then extended)
	anchor := buffer.Pos{Row: 8}
	b.Anchor, b.Cur = &anchor, buffer.Pos{Row: 11}
	a.reviewKey("c")
	if a.composer == nil {
		t.Fatalf("no editor: %s", a.msg)
	}
	a.composer.ta.InsertString("前置き")
	a.handleComposerKey(Key{Code: 's', Mod: ModCtrl})
	if v := a.composer.ta.Value(); !strings.HasPrefix(v, "前置き\n```suggestion\nlinei\nchanged\nlinek\n```") {
		t.Errorf("suggestion block: %q", v)
	}
	a.composer.ta.SetValue("見てください")
	a.handleComposerKey(Key{Code: KeyEnter})
	if a.composer != nil || len(a.gd.rv.pending) != 1 {
		t.Fatal("comment not added")
	}
	th := a.gd.rv.pending[0]
	if th.Path != "a.txt" || th.StartLine != 9 || th.Line != 11 || th.Side != "RIGHT" || th.Comments[0].Body != "見てください" {
		t.Errorf("thread = %+v", th)
	}
	if st := ansi.Strip(a.renderStatus()); !strings.Contains(st, "未送信コメント 1 件") {
		t.Errorf("status: %q", st)
	}
	// the marker shows on the last line, and Space v t opens the card
	b.Anchor, b.Cur = nil, buffer.Pos{Row: 10}
	a.reviewKey("t")
	if a.card == nil || a.card.thread != th {
		t.Fatal("card not opened")
	}
	if !strings.Contains(ansi.Strip(a.render()), "見てください") {
		t.Error("card not drawn")
	}
	// x deletes a comment that was not sent
	a.handleKey(textKey("x"))
	if a.card != nil || len(a.gd.rv.pending) != 0 {
		t.Error("x did not delete the pending comment")
	}
}

func TestCommentRefusedOnStaleOrUnsavedFile(t *testing.T) {
	a, _ := prApp(t)
	a.curText().buf.Cur = buffer.Pos{Row: 9}
	a.gd.stale = map[string]bool{"a.txt": true}
	a.reviewKey("c")
	if a.composer != nil || a.msg != "このファイルは手元で変更があるため、行にコメントできません" {
		t.Errorf("stale: composer=%v msg=%q", a.composer != nil, a.msg)
	}
	a.reviewKey("C")
	if a.composer != nil {
		t.Error("file comment on a stale file")
	}
	a.gd.stale = nil
	a.curText().buf.Insert("x")
	a.reviewKey("c")
	if a.composer != nil || !a.msgErr {
		t.Error("comment on an unsaved buffer")
	}
}

func TestSpaceVMenu(t *testing.T) {
	a, _ := prApp(t)
	a.curText().buf.Cur = buffer.Pos{Row: 9}
	press(a, " ")
	if !strings.Contains(ansi.Strip(a.render()), "レビュー（PR）") {
		t.Error("Space menu lacks v")
	}
	press(a, "v")
	if a.pending != "space v" || !strings.Contains(ansi.Strip(a.render()), "承認して送信") {
		t.Fatalf("pending=%q", a.pending)
	}
	press(a, "C")
	if a.composer == nil || !strings.Contains(a.composer.title, "ファイルへのコメント: a.txt") {
		t.Fatal("Space v C did not open the editor")
	}
	a.composer.ta.SetValue("全体")
	a.handleKey(Key{Code: KeyEnter})
	if len(a.gd.rv.pending) != 1 || !a.gd.rv.pending[0].File {
		t.Fatal("file comment not added")
	}
	// outside a pull request, Space v explains itself
	a.openDiff(baseHead)
	press(a, " ")
	press(a, "v")
	if a.pending != "" || !strings.Contains(a.msg, "PR の base と比べているとき") {
		t.Errorf("pending=%q msg=%q", a.pending, a.msg)
	}
}

func TestInHunk(t *testing.T) {
	a, _ := prApp(t)
	dv := a.diffFor(a.curText())
	for row, want := range map[int]bool{5: false, 6: true, 9: true, 12: true, 13: false} {
		if got := dv.inHunk(row); got != want {
			t.Errorf("row %d: %v", row, got)
		}
	}
}

func TestSuggestion(t *testing.T) {
	th := &prThread{Line: 3, StartLine: 2, Side: "RIGHT", Comments: []prComment{
		{Body: "前\n```suggestion\nold\n```"},
		{Body: "直しました\n```suggestion\nnew1\nnew2\n```\n後"},
	}}
	if got := suggestion(th); strings.Join(got, "|") != "new1|new2" {
		t.Errorf("got %q", got)
	}
	if got := suggestion(&prThread{Line: 1, Side: "RIGHT", Comments: []prComment{{Body: "```suggestion\n```"}}}); got == nil || len(got) != 0 {
		t.Errorf("empty suggestion: %q", got)
	}
	if suggestion(&prThread{Line: 1, Side: "RIGHT", Comments: []prComment{{Body: "```go\nx\n```"}}}) != nil {
		t.Error("a go block is not a suggestion")
	}
}

func TestApplySuggestion(t *testing.T) {
	a, file := prApp(t)
	th := &prThread{ID: "T", Path: "a.txt", Line: 11, StartLine: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "```suggestion\nfixed\n```"}}}
	a.gd.rv.threads = []*prThread{th}
	a.card = &prCard{thread: th, path: "a.txt"}
	if !strings.Contains(ansi.Strip(a.render()), "s suggestion を適用") {
		t.Error("hint lacks s")
	}
	a.handleKey(textKey("s"))
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "linei\nfixed\nlinel\n") {
		t.Errorf("file = %q", data)
	}
	if a.card != nil {
		t.Error("card still open")
	}
	// the file now differs from the pull request: no more suggestions
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	a.card = &prCard{thread: th, path: "a.txt"}
	a.handleKey(textKey("s"))
	if !a.msgErr {
		t.Error("suggestion applied to a stale file")
	}
}

func TestCardKeys(t *testing.T) {
	a, _ := prApp(t)
	th := &prThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "x"}}}
	a.gd.rv.threads = []*prThread{th}
	a.card = &prCard{thread: th, path: "a.txt"}
	a.curText().buf.Cur = buffer.Pos{}
	a.handleKey(Key{Code: KeyEnter})
	if a.card != nil || a.curText().buf.Cur.Row != 9 {
		t.Errorf("Enter: card=%v row=%d", a.card != nil, a.curText().buf.Cur.Row)
	}
	a.card = &prCard{thread: th, path: "a.txt"}
	a.handleKey(textKey("r"))
	if a.composer == nil || !strings.HasPrefix(a.composer.title, "返信") {
		t.Fatal("r did not open a reply")
	}
	a.handleKey(Key{Code: KeyEsc})
	if a.composer != nil {
		t.Error("Esc did not close the editor")
	}
	a.handleKey(Key{Code: KeyEsc})
	if a.card != nil {
		t.Error("Esc did not close the card")
	}
	// another key closes the card and goes on to the editor
	a.card = &prCard{thread: th, path: "a.txt"}
	a.handleKey(textKey("l"))
	if a.card != nil || a.curText().buf.Cur.Col != 1 {
		t.Errorf("l: card=%v col=%d", a.card != nil, a.curText().buf.Cur.Col)
	}
}

func TestMarkerClickOpensCard(t *testing.T) {
	a, _ := prApp(t)
	th := &prThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "x"}}}
	a.gd.rv.threads = []*prThread{th}
	a.render()
	l := a.layout()
	g := a.dvGeom(l)
	tb := a.curText()
	dv := a.diffFor(tb)
	y := g.top + dv.newIdx[9] - dv.start(tb.buf.ScrollRow)
	a.handleMouse(Mouse{X: g.rightX, Y: y, Button: MouseLeft, Action: MousePress})
	if a.card == nil || a.card.thread != th {
		t.Fatal("click on the marker did not open the card")
	}
	a.handleMouse(Mouse{X: g.rightX + 30, Y: g.top, Button: MouseLeft, Action: MousePress})
	if a.card != nil {
		t.Error("click elsewhere did not close the card")
	}
}

func TestSwitchToPR(t *testing.T) {
	a, _ := gitDiffApp(t) // a.txt is modified and not committed
	pr := &prInfo{Number: 9, HeadRefName: "other", BaseRefName: "main"}
	if cmd := a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 9, pr: pr}); cmd != nil || a.confirm == nil {
		t.Fatal("no question about stashing")
	}
	if !strings.Contains(ansi.Strip(a.renderStatus()), "未コミットの変更を stash して切り替えますか？ (y/n)") {
		t.Error("question not shown")
	}
	press(a, "q") // other keys leave the question open
	if a.confirm == nil {
		t.Fatal("question closed by another key")
	}
	press(a, "n")
	if a.confirm != nil || a.gd.base == basePR {
		t.Fatal("n did not cancel")
	}
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 9, pr: pr})
	if cmd := a.handleKey(textKey("y")); cmd == nil || a.confirm != nil {
		t.Fatal("y did not start the checkout")
	}
	// unsaved buffers stop the switch before anything runs
	a.curText().buf.Insert("x")
	if cmd := a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 9, pr: pr}); cmd != nil || a.confirm != nil || !a.msgErr {
		t.Error("switched with unsaved buffers")
	}
}

func TestCheckoutPRStashes(t *testing.T) {
	a, _ := gitDiffApp(t)
	pr := &prInfo{Number: 9, HeadRefName: "other", BaseRefName: "main"}
	msg := a.checkoutPR(pr, true)().(prCheckedOutMsg)
	if msg.err == nil {
		t.Skip("gh pr checkout worked here; nothing to check")
	}
	// the checkout failed on the same branch: the stash is popped again
	if got := gitOut(t, a.root, "stash", "list"); got != "" {
		t.Errorf("stash list = %q", got)
	}
	if got := gitOut(t, a.root, "status", "--porcelain"); !strings.Contains(got, "a.txt") || !strings.Contains(got, "new.txt") {
		t.Errorf("changes not restored: %q", got)
	}
	a.handlePRCheckedOut(msg)
	if !a.msgErr || a.gd.base == basePR {
		t.Error("failed checkout opened the pull request")
	}
}

func TestSyncText(t *testing.T) {
	for _, c := range []struct {
		s    prSync
		want string
	}{
		{prSync{}, ""},
		{prSync{known: true}, ""},
		{prSync{known: true, behind: 2}, "PR が更新されました。r で pull します"},
		{prSync{known: true, ahead: 3}, "push していないコミットが 3 個あります"},
		{prSync{known: true, ahead: 1, behind: 1}, "ローカルと PR が分岐しています"},
	} {
		if got := c.s.text(); got != c.want {
			t.Errorf("%+v: %q", c.s, got)
		}
	}
}

func TestSyncWith(t *testing.T) {
	a, _ := prApp(t)
	head := gitOut(t, a.root, "rev-parse", "HEAD")
	base := gitOut(t, a.root, "rev-parse", "main")
	if s := syncWith(a.root, head); !s.known || s.ahead != 0 || s.behind != 0 {
		t.Errorf("same: %+v", s)
	}
	if s := syncWith(a.root, base); s.ahead != 1 || s.behind != 0 {
		t.Errorf("local ahead: %+v", s)
	}
	a.gd.rv.sync = syncWith(a.root, base)
	if !strings.Contains(ansi.Strip(a.renderStatus()), "push していないコミットが 1 個あります") {
		t.Error("status lacks the note")
	}
	// r with only the pull request ahead pulls; otherwise it reloads
	a.gd.rv.sync = prSync{known: true, behind: 1}
	a.reloadPR()
	if !strings.Contains(a.msg, "git pull") {
		t.Errorf("r: %q", a.msg)
	}
	a.gd.rv.sync = prSync{known: true, ahead: 1, behind: 1}
	a.reloadPR()
	if strings.Contains(a.msg, "git pull") {
		t.Error("r pulled a diverged branch")
	}
}

func TestReviewSentDropsSentComments(t *testing.T) {
	a, _ := prApp(t)
	t1, t2 := &prThread{Pending: true}, &prThread{Pending: true}
	a.gd.rv.pending = []*prThread{t1, t2}
	a.handleReviewSent(reviewSentMsg{num: 7, done: "ok", sent: []*prThread{t1}})
	if len(a.gd.rv.pending) != 1 || a.gd.rv.pending[0] != t2 {
		t.Errorf("pending = %v", a.gd.rv.pending)
	}
	a.handleReviewSent(reviewSentMsg{num: 7, err: errString("boom"), sent: []*prThread{t2}})
	if len(a.gd.rv.pending) != 1 || !a.msgErr {
		t.Error("a failed send dropped comments")
	}
}

func TestSubmitNeedsSomething(t *testing.T) {
	a, _ := prApp(t)
	a.reviewKey("S")
	if a.composer == nil {
		t.Fatal("no editor for the review body")
	}
	if cmd := a.handleComposerKey(Key{Code: KeyEnter}); cmd != nil || !a.msgErr {
		t.Error("empty comment-only review was sent")
	}
	a.reviewKey("A")
	if cmd := a.handleComposerKey(Key{Code: KeyEnter}); cmd == nil {
		t.Error("approval without a body was not sent")
	}
}

func TestComposerNewlineAndPreview(t *testing.T) {
	a, _ := prApp(t)
	a.compose("t", "", nil, func(string) tea.Cmd { return nil })
	a.composer.ta.InsertString("# 見出し")
	a.handleComposerKey(Key{Code: KeyEnter, Mod: ModShift})
	a.composer.ta.InsertString("本文")
	if v := a.composer.ta.Value(); v != "# 見出し\n本文" {
		t.Errorf("value %q", v)
	}
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "プレビュー") || !strings.Contains(view, "本文") {
		t.Error("preview not drawn")
	}
	if _, _, ok := a.caretPos(a.layout()); !ok {
		t.Error("no caret for the IME")
	}
}

func TestResizeDragWithoutButton(t *testing.T) {
	a := resizeApp(t)
	l := a.layout()
	a.handleMouse(Mouse{X: l.sideW, Y: 5, Action: MousePress, Button: MouseLeft})
	a.handleMouse(Mouse{X: 40, Y: 5, Action: MouseMotion, Button: MouseNone})
	a.handleMouse(Mouse{X: 40, Y: 5, Action: MouseRelease, Button: MouseNone})
	if a.layout().sideW != 40 || a.resizing != resizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}
}

func TestResizeDragInDiffMode(t *testing.T) {
	a, _ := prApp(t)
	l := a.layout()
	a.handleMouse(Mouse{X: l.sideW, Y: 5, Action: MousePress, Button: MouseLeft})
	a.handleMouse(Mouse{X: 40, Y: 5, Action: MouseMotion, Button: MouseLeft})
	a.handleMouse(Mouse{X: 40, Y: 5, Action: MouseRelease, Button: MouseLeft})
	if a.layout().sideW != 40 {
		t.Fatalf("side drag in diff mode: %d", a.layout().sideW)
	}
}

// markerCell returns the screen cell of the marker of buffer line row.
func markerCell(a *App, row int) (x, y int) {
	a.render()
	g := a.dvGeom(a.layout())
	tb := a.curText()
	dv := a.diffFor(tb)
	return g.rightX, g.top + dv.newIdx[row] - dv.start(tb.buf.ScrollRow)
}

func TestPointerCardTakesNoKeys(t *testing.T) {
	a, _ := prApp(t)
	a.cfg.Editor.MouseHover = true
	th := &prThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "x"}}}
	a.gd.rv.threads = []*prThread{th}
	x, y := markerCell(a, 9)
	a.handleMouse(Mouse{X: x, Y: y, Action: MouseMotion, Button: MouseNone})
	if a.card == nil || !a.card.byPointer {
		t.Fatal("resting on the marker did not show the card")
	}
	// a key closes it and goes to the editor: x selects the line, nothing is sent
	cmd := a.handleKey(textKey("x"))
	if a.card != nil || cmd != nil {
		t.Errorf("x went to the card: card=%v cmd=%v", a.card != nil, cmd != nil)
	}
	if _, _, ok := a.curText().buf.Selection(); !ok {
		t.Error("x did not reach the editor")
	}
	// leaving the marker closes it
	a.handleMouse(Mouse{X: x, Y: y, Action: MouseMotion, Button: MouseNone})
	a.handleMouse(Mouse{X: x + 60, Y: y - 5, Action: MouseMotion, Button: MouseNone})
	if a.card != nil {
		t.Error("card stayed after the pointer left")
	}
	// without mouse hover, resting opens nothing
	a.cfg.Editor.MouseHover = false
	a.handleMouse(Mouse{X: x, Y: y, Action: MouseMotion, Button: MouseNone})
	if a.card != nil {
		t.Error("card opened with mouse hover off")
	}
}

func TestPendingCommentsSurviveHeadBase(t *testing.T) {
	a, _ := prApp(t)
	pr := a.gd.pr
	a.gd.rv.pending = []*prThread{{Path: "a.txt", Line: 10, Side: "RIGHT", Pending: true, Comments: []prComment{{Body: "x"}}}}
	a.openDiff(baseHead)
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 7, pr: pr})
	if len(a.gd.rv.pending) != 1 {
		t.Fatal("Space G then Space g lost the comment")
	}
	// another pull request asks first
	other := &prInfo{Number: 8, HeadRefName: "feature", BaseRefName: "main"}
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 8, pr: other})
	if a.confirm == nil || !strings.Contains(a.confirm.prompt, "PR #7 の未送信コメントが 1 件あります") {
		t.Fatal("no question before dropping comments")
	}
	press(a, "n")
	if len(a.gd.rv.pending) != 1 || a.gd.pr.Number != 7 {
		t.Error("n dropped the comments")
	}
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 8, pr: other})
	press(a, "y")
	if len(a.gd.rv.pending) != 0 || a.gd.pr.Number != 8 {
		t.Errorf("y: pending=%d pr=%d", len(a.gd.rv.pending), a.gd.pr.Number)
	}
}

func TestLateAnswersForAnotherPRAreDropped(t *testing.T) {
	a, _ := prApp(t)
	a.handleViewed(viewedMsg{num: 99, path: "a.txt", viewed: true})
	if a.gd.rv.viewed["a.txt"] {
		t.Error("viewed answer of another PR applied")
	}
	a.handleViewed(viewedMsg{num: 7, path: "a.txt", viewed: true})
	if !a.gd.rv.viewed["a.txt"] {
		t.Error("viewed answer not applied")
	}
	p := &prThread{Pending: true}
	a.gd.rv.pending = []*prThread{p}
	if cmd := a.handleReviewSent(reviewSentMsg{num: 99, sent: []*prThread{p}}); cmd != nil || len(a.gd.rv.pending) != 1 {
		t.Error("send answer of another PR applied")
	}
	// review loads: an older one or one for another PR is dropped; the open
	// card follows its thread into the new data
	old := &prThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Body: "old"}}}
	a.card = &prCard{thread: old, path: "a.txt"}
	a.gd.rv.seq = 5
	fresh := &prThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Body: "old"}, {Body: "reply"}}}
	a.handlePRReview(prReviewMsg{seq: 4, num: 7, data: prReviewData{threads: []*prThread{fresh}}})
	a.handlePRReview(prReviewMsg{seq: 5, num: 8, data: prReviewData{threads: []*prThread{fresh}}})
	if len(a.gd.rv.threads) != 0 {
		t.Fatal("stale review load applied")
	}
	a.handlePRReview(prReviewMsg{seq: 5, num: 7, data: prReviewData{threads: []*prThread{fresh}}})
	if len(a.gd.rv.threads) != 1 || a.card == nil || a.card.thread != fresh {
		t.Error("card not moved to the reloaded thread")
	}
}

func TestInHunkAroundDeletion(t *testing.T) {
	// base 1..10, buffer has line 5 deleted: the deletion sits between buffer
	// lines 3 and 4 (0-based); 3 lines of context on each side make rows 1..6
	// the hunk
	dv := &diffView{}
	for i := 1; i <= 10; i++ {
		dv.old = append(dv.old, fmt.Sprint(i))
	}
	b := buffer.New()
	b.Insert("1\n2\n3\n4\n6\n7\n8\n9\n10")
	dv.build(newTab(b), false)
	for row, want := range map[int]bool{0: false, 1: true, 3: true, 6: true, 7: false} {
		if got := dv.inHunk(row); got != want {
			t.Errorf("row %d: %v", row, got)
		}
	}
}

func TestApplyEmptySuggestionDeletesLines(t *testing.T) {
	a, file := prApp(t)
	th := &prThread{ID: "T", Path: "a.txt", Line: 11, StartLine: 10, Side: "RIGHT", Comments: []prComment{{Body: "```suggestion\n```"}}}
	a.gd.rv.threads = []*prThread{th}
	a.card = &prCard{thread: th, path: "a.txt"}
	a.handleKey(textKey("s"))
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "linei\nlinel\n") || strings.Contains(string(data), "changed") {
		t.Errorf("file = %q", data)
	}
	// the last line: the line break before it goes
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	gitIn(t, a.root, "commit", "-qam", "x")
	a.gd.pr.HeadRefOid = gitOut(t, a.root, "rev-parse", "HEAD")
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	n := len(a.curText().buf.Lines) // the empty line after the last newline counts
	th2 := &prThread{ID: "U", Path: "a.txt", Line: n - 1, Side: "RIGHT", Comments: []prComment{{Body: "```suggestion\n```"}}}
	a.card = &prCard{thread: th2, path: "a.txt"}
	a.handleKey(textKey("s"))
	data, _ = os.ReadFile(file)
	if !strings.HasSuffix(string(data), "lines\n") {
		t.Errorf("last line not removed: %q", data[len(data)-20:])
	}
}

func TestEnterOnBaseSideThreadDoesNotMove(t *testing.T) {
	a, _ := prApp(t)
	th := &prThread{ID: "T", Path: "a.txt", Line: 15, Side: "LEFT", Comments: []prComment{{Body: "x"}}}
	a.card = &prCard{thread: th, path: "a.txt"}
	a.curText().buf.Cur = buffer.Pos{Row: 2}
	a.handleKey(Key{Code: KeyEnter})
	if a.curText().buf.Cur.Row != 2 {
		t.Errorf("moved to %d", a.curText().buf.Cur.Row)
	}
}

// stubGH replaces gh with f for the test.
func stubGH(t *testing.T, f func(req map[string]any) string) *[]string {
	t.Helper()
	var queries []string
	old := ghRun
	ghRun = func(dir string, stdin []byte, name string, args ...string) ([]byte, error) {
		var req map[string]any
		json.Unmarshal(stdin, &req)
		queries = append(queries, req["query"].(string))
		return []byte(f(req)), nil
	}
	t.Cleanup(func() { ghRun = old })
	return &queries
}

func TestSubmitReviewBuildsAndCleansUp(t *testing.T) {
	pr := &prInfo{ID: "PR", Number: 1, HeadRefOid: "abc", URL: "https://example/pr/1"}
	threads := []*prThread{
		{Path: "a.go", Line: 5, StartLine: 3, Side: "RIGHT", Comments: []prComment{{Body: "range"}}},
		{Path: "b.go", File: true, Comments: []prComment{{Body: "file"}}},
	}
	q := stubGH(t, func(req map[string]any) string {
		s := req["query"].(string)
		switch {
		case strings.Contains(s, "reviews(states: PENDING"):
			return `{"data":{"node":{"reviews":{"nodes":[]}}}}`
		case strings.Contains(s, "addPullRequestReview("):
			return `{"data":{"addPullRequestReview":{"pullRequestReview":{"id":"R1"}}}}`
		}
		return `{"data":{}}`
	})
	if err := submitReview("", pr, "COMMENT", "body", threads); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(*q, "\n")
	for _, s := range []string{"startLine: $start, startSide: RIGHT", "subjectType: FILE", "submitPullRequestReview", "$body: String"} {
		if !strings.Contains(all, s) {
			t.Errorf("queries lack %q", s)
		}
	}
	if strings.Contains(all, "deletePullRequestReview") {
		t.Error("deleted a review that was sent")
	}

	// a failing thread deletes the draft
	q = stubGH(t, func(req map[string]any) string {
		s := req["query"].(string)
		switch {
		case strings.Contains(s, "reviews(states: PENDING"):
			return `{"data":{"node":{"reviews":{"nodes":[]}}}}`
		case strings.Contains(s, "addPullRequestReview("):
			return `{"data":{"addPullRequestReview":{"pullRequestReview":{"id":"R1"}}}}`
		case strings.Contains(s, "addPullRequestReviewThread"):
			return `{"errors":[{"message":"line must be part of the diff"}]}`
		}
		return `{"data":{}}`
	})
	if err := submitReview("", pr, "COMMENT", "", threads); err == nil || !strings.Contains(err.Error(), "a.go") {
		t.Errorf("err = %v", err)
	}
	if !strings.Contains(strings.Join(*q, "\n"), "deletePullRequestReview") {
		t.Error("draft not deleted after a failure")
	}

	// a draft left on GitHub stops the send before anything is created
	q = stubGH(t, func(req map[string]any) string {
		return `{"data":{"node":{"reviews":{"nodes":[{"viewerDidAuthor":true}]}}}}`
	})
	if err := submitReview("", pr, "APPROVE", "", nil); err == nil || !strings.Contains(err.Error(), "下書きのレビュー") {
		t.Errorf("err = %v", err)
	}
	if len(*q) != 1 {
		t.Errorf("%d queries after finding a draft", len(*q))
	}
}

func TestLoadReviewReadsEveryPage(t *testing.T) {
	stubGH(t, func(req map[string]any) string {
		vars := req["variables"].(map[string]any)
		after, _ := vars["after"].(string)
		if strings.Contains(req["query"].(string), "reviewThreads") {
			if after == "" {
				return `{"data":{"node":{"reviewThreads":{"pageInfo":{"hasNextPage":true,"endCursor":"c1"},"nodes":[{"id":"T1","path":"a","line":3,"diffSide":"RIGHT","subjectType":"LINE","comments":{"nodes":[{"body":"x","author":{"login":"u"}}]}}]}}}}`
			}
			return `{"data":{"node":{"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[{"id":"T2","path":"b","line":null,"diffSide":"RIGHT","subjectType":"FILE","comments":{"nodes":[{"body":"y","author":null}]}}]}}}}`
		}
		if after == "" {
			return `{"data":{"node":{"files":{"pageInfo":{"hasNextPage":true,"endCursor":"f1"},"nodes":[{"path":"a","viewerViewedState":"VIEWED"}]}}}}`
		}
		return `{"data":{"node":{"files":{"pageInfo":{"hasNextPage":false},"nodes":[{"path":"b","viewerViewedState":"VIEWED"},{"path":"c","viewerViewedState":"UNVIEWED"}]}}}}`
	})
	d, err := loadReview("", &prInfo{ID: "PR"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.threads) != 2 || !d.threads[1].File || d.threads[1].Outdated || d.threads[1].Comments[0].Author != "ghost" {
		t.Errorf("threads = %+v", d.threads)
	}
	if !d.viewed["a"] || !d.viewed["b"] || d.viewed["c"] {
		t.Errorf("viewed = %v", d.viewed)
	}
}

func TestCommentOnNewFileInOnePane(t *testing.T) {
	a, _ := prApp(t)
	b := filepath.Join(a.gitTopDir(), "b.txt")
	if !a.openFile(b) {
		t.Fatal("b.txt not opened")
	}
	tb := a.curText()
	dv := a.diffFor(tb)
	if dv == nil || !dv.single {
		t.Fatalf("new file against a PR is not one pane: %+v", dv)
	}
	if !dv.inHunk(0) || !dv.inHunk(1) || dv.inHunk(len(dv.newIdx)) {
		t.Error("inHunk of a one-pane file")
	}
	tb.buf.Anchor, tb.buf.Cur = nil, buffer.Pos{Row: 1}
	a.reviewKey("c")
	if a.composer == nil {
		t.Fatalf("no editor: %s", a.msg)
	}
	a.composer.ta.SetValue("新規ファイルへ")
	a.handleComposerKey(Key{Code: KeyEnter})
	if len(a.gd.rv.pending) != 1 {
		t.Fatal("comment not added")
	}
	if th := a.gd.rv.pending[0]; th.Path != "b.txt" || th.Line != 2 || th.Side != "RIGHT" {
		t.Errorf("thread = %+v", th)
	}
	if got := a.gd.rv.lineThreads("b.txt", 2); len(got) != 1 {
		t.Errorf("thread not found on line 2: %v", got)
	}
}

func TestReviewPRURLCommands(t *testing.T) {
	a, _ := prApp(t)
	a.gd.pr.URL = "https://github.com/o/r/pull/7"
	oldOpen, oldSSH := openExternal, isRemoteSession
	defer func() { openExternal, isRemoteSession = oldOpen, oldSSH }()
	var opened string
	var openErr error
	openExternal = func(u string) error { opened = u; return openErr }
	remote := false
	isRemoteSession = func() bool { return remote }
	reset := func() { opened, a.clipboard, a.msg, a.msgErr = "", "", "", false }
	const url = "https://github.com/o/r/pull/7"

	reset()
	runCmd(a, a.reviewKey("y"))
	if a.clipboard != url || a.msgErr || !strings.Contains(a.msg, url) {
		t.Fatalf("copy: clip %q msg %q", a.clipboard, a.msg)
	}
	reset()
	runCmd(a, a.reviewKey("o"))
	if opened != url || a.clipboard != "" || a.msgErr {
		t.Fatalf("open: opened %q clip %q msg %q", opened, a.clipboard, a.msg)
	}
	reset()
	remote = true
	runCmd(a, a.reviewKey("o"))
	if opened != "" || a.clipboard != url || a.msgErr || !strings.Contains(a.msg, "SSH") {
		t.Fatalf("ssh: opened %q clip %q msg %q", opened, a.clipboard, a.msg)
	}
	remote = false
	reset()
	openErr = fmt.Errorf("boom")
	runCmd(a, a.reviewKey("o"))
	if a.clipboard != url || a.msgErr || strings.Contains(a.msg, "boom") {
		t.Fatalf("fallback: clip %q msg %q err %v", a.clipboard, a.msg, a.msgErr)
	}
	// Not comparing with a PR base: palette commands only say so.
	reset()
	a.gd.base = baseHead
	runCmd(a, a.reviewCommand("y"))
	runCmd(a, a.reviewCommand("o"))
	if opened != "" || a.clipboard != "" || a.msg != i18n.T("rv.needpr") {
		t.Fatalf("needpr: opened %q clip %q msg %q", opened, a.clipboard, a.msg)
	}
}

func TestSSHDetection(t *testing.T) {
	for _, k := range []string{"SSH_CONNECTION", "SSH_TTY"} {
		t.Setenv("SSH_CONNECTION", "")
		t.Setenv("SSH_TTY", "")
		if isRemoteSession() {
			t.Fatal("not remote")
		}
		t.Setenv(k, "x")
		if !isRemoteSession() {
			t.Fatalf("%s must mean remote", k)
		}
	}
}
