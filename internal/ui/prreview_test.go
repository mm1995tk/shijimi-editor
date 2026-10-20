package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
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
	a.handleComposerKey(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if v := a.composer.ta.Value(); !strings.HasPrefix(v, "前置き\n```suggestion\nlinei\nchanged\nlinek\n```") {
		t.Errorf("suggestion block: %q", v)
	}
	a.composer.ta.SetValue("見てください")
	a.handleComposerKey(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	a.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	a.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if a.card != nil || a.curText().buf.Cur.Row != 9 {
		t.Errorf("Enter: card=%v row=%d", a.card != nil, a.curText().buf.Cur.Row)
	}
	a.card = &prCard{thread: th, path: "a.txt"}
	a.handleKey(textKey("r"))
	if a.composer == nil || !strings.HasPrefix(a.composer.title, "返信") {
		t.Fatal("r did not open a reply")
	}
	a.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if a.composer != nil {
		t.Error("Esc did not close the editor")
	}
	a.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
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
	a.handleMouse(mouseEvent{X: g.rightX, Y: y, Button: tea.MouseLeft, Action: mousePress})
	if a.card == nil || a.card.thread != th {
		t.Fatal("click on the marker did not open the card")
	}
	a.handleMouse(mouseEvent{X: g.rightX + 30, Y: g.top, Button: tea.MouseLeft, Action: mousePress})
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
	if got := gitOut(t, a.root, "stash", "list"); !strings.Contains(got, "shijimi: PR #9 に切り替える前の変更") {
		t.Errorf("stash list = %q", got)
	}
	if got := gitOut(t, a.root, "status", "--porcelain"); got != "" {
		t.Errorf("tree not clean after stash: %q", got)
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
	a.handleReviewSent(reviewSentMsg{done: "ok", sent: []*prThread{t1}})
	if len(a.gd.rv.pending) != 1 || a.gd.rv.pending[0] != t2 {
		t.Errorf("pending = %v", a.gd.rv.pending)
	}
	a.handleReviewSent(reviewSentMsg{err: errString("boom"), sent: []*prThread{t2}})
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
	if cmd := a.handleComposerKey(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || !a.msgErr {
		t.Error("empty comment-only review was sent")
	}
	a.reviewKey("A")
	if cmd := a.handleComposerKey(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd == nil {
		t.Error("approval without a body was not sent")
	}
}

func TestComposerNewlineAndPreview(t *testing.T) {
	a, _ := prApp(t)
	a.compose("t", "", nil, func(string) tea.Cmd { return nil })
	a.composer.ta.InsertString("# 見出し")
	a.handleComposerKey(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
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
	a.handleMouse(mouseEvent{X: l.sideW, Y: 5, Action: mousePress, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: 40, Y: 5, Action: mouseMotion, Button: tea.MouseNone})
	a.handleMouse(mouseEvent{X: 40, Y: 5, Action: mouseRelease, Button: tea.MouseNone})
	if a.layout().sideW != 40 || a.resizing != resizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}
}

func TestResizeDragInDiffMode(t *testing.T) {
	a, _ := prApp(t)
	l := a.layout()
	a.handleMouse(mouseEvent{X: l.sideW, Y: 5, Action: mousePress, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: 40, Y: 5, Action: mouseMotion, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: 40, Y: 5, Action: mouseRelease, Button: tea.MouseLeft})
	if a.layout().sideW != 40 {
		t.Fatalf("side drag in diff mode: %d", a.layout().sideW)
	}
}
