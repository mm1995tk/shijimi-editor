package tui

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
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestStaleFileShowsMarks(t *testing.T) {
	a, file := prApp(t)
	a.Update(tea.WindowSizeMsg{Width: 220, Height: 40})
	a.sideWPref = 40
	a.gd.rv.threads = []*PrThread{
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
	if mark := a.CommentMark(a.CurText()); mark == nil || mark(9) == "" || mark(8) != "" {
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
	if a.CommentMark(a.CurText()) != nil {
		t.Error("markers drawn on a stale file")
	}
}

func TestCommentOnLines(t *testing.T) {
	a, _ := prApp(t)
	b := a.CurText().Buf
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
	a.composer.Ta.InsertString("前置き")
	a.handleComposerKey(Key{Code: 's', Mod: ModCtrl})
	if v := a.composer.Ta.Value(); !strings.HasPrefix(v, "前置き\n```suggestion\nlinei\nchanged\nlinek\n```") {
		t.Errorf("suggestion block: %q", v)
	}
	a.composer.Ta.SetValue("見てください")
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
	if a.card == nil || a.card.Thread != th {
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

func TestSpaceVMenu(t *testing.T) {
	a, _ := prApp(t)
	a.CurText().Buf.Cur = buffer.Pos{Row: 9}
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
	a.composer.Ta.SetValue("全体")
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

func TestApplySuggestion(t *testing.T) {
	a, file := prApp(t)
	th := &PrThread{ID: "T", Path: "a.txt", Line: 11, StartLine: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "```suggestion\nfixed\n```"}}}
	a.gd.rv.threads = []*PrThread{th}
	a.card = &prCard{Thread: th, Path: "a.txt"}
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
	a.card = &prCard{Thread: th, Path: "a.txt"}
	a.handleKey(textKey("s"))
	if !a.msgErr {
		t.Error("suggestion applied to a stale file")
	}
}

func TestMarkerClickOpensCard(t *testing.T) {
	a, _ := prApp(t)
	th := &PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "x"}}}
	a.gd.rv.threads = []*PrThread{th}
	a.render()
	l := a.layout()
	g := a.dvGeom(l)
	tb := a.CurText()
	dv := a.DiffFor(tb)
	y := g.Top + dv.NewIdx[9] - dv.Start(tb.Buf.ScrollRow)
	a.HandleMouse(a.locate(g.RightX, y, Mouse{Button: MouseLeft, Action: MousePress}))
	if a.card == nil || a.card.Thread != th {
		t.Fatal("click on the marker did not open the card")
	}
	a.HandleMouse(a.locate(g.RightX+30, g.Top, Mouse{Button: MouseLeft, Action: MousePress}))
	if a.card != nil {
		t.Error("click elsewhere did not close the card")
	}
}

func TestSwitchToPR(t *testing.T) {
	a, _ := gitDiffApp(t) // a.txt is modified and not committed
	pr := &PrInfo{Number: 9, HeadRefName: "other", BaseRefName: "main"}
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
	a.CurText().Buf.Insert("x")
	if cmd := a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 9, pr: pr}); cmd != nil || a.confirm != nil || !a.msgErr {
		t.Error("switched with unsaved buffers")
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

func TestComposerNewlineAndPreview(t *testing.T) {
	a, _ := prApp(t)
	a.compose("t", "", nil, func(string) []Effect { return nil })
	a.composer.Ta.InsertString("# 見出し")
	a.handleComposerKey(Key{Code: KeyEnter, Mod: ModShift})
	a.composer.Ta.InsertString("本文")
	if v := a.composer.Ta.Value(); v != "# 見出し\n本文" {
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
	a.HandleMouse(a.locate(l.sideW, 5, Mouse{Action: MousePress, Button: MouseLeft}))
	a.HandleMouse(a.locate(40, 5, Mouse{Action: MouseMotion, Button: MouseNone}))
	a.HandleMouse(a.locate(40, 5, Mouse{Action: MouseRelease, Button: MouseNone}))
	if a.layout().sideW != 40 || a.resizing != ResizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}
}

func TestResizeDragInDiffMode(t *testing.T) {
	a, _ := prApp(t)
	l := a.layout()
	a.HandleMouse(a.locate(l.sideW, 5, Mouse{Action: MousePress, Button: MouseLeft}))
	a.HandleMouse(a.locate(40, 5, Mouse{Action: MouseMotion, Button: MouseLeft}))
	a.HandleMouse(a.locate(40, 5, Mouse{Action: MouseRelease, Button: MouseLeft}))
	if a.layout().sideW != 40 {
		t.Fatalf("side drag in diff mode: %d", a.layout().sideW)
	}
}

// markerCell returns the screen cell of the marker of buffer line row.
func markerCell(a *App, row int) (x, y int) {
	a.render()
	g := a.dvGeom(a.layout())
	tb := a.CurText()
	dv := a.DiffFor(tb)
	return g.RightX, g.Top + dv.NewIdx[row] - dv.Start(tb.Buf.ScrollRow)
}

func TestPointerCardTakesNoKeys(t *testing.T) {
	a, _ := prApp(t)
	a.cfg.Editor.MouseHover = true
	th := &PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "x"}}}
	a.gd.rv.threads = []*PrThread{th}
	x, y := markerCell(a, 9)
	a.HandleMouse(a.locate(x, y, Mouse{Action: MouseMotion, Button: MouseNone}))
	if a.card == nil || !a.card.byPointer {
		t.Fatal("resting on the marker did not show the card")
	}
	// a key closes it and goes to the editor: x selects the line, nothing is sent
	cmd := a.handleKey(textKey("x"))
	if a.card != nil || cmd != nil {
		t.Errorf("x went to the card: card=%v cmd=%v", a.card != nil, cmd != nil)
	}
	if _, _, ok := a.CurText().Buf.Selection(); !ok {
		t.Error("x did not reach the editor")
	}
	// leaving the marker closes it
	a.HandleMouse(a.locate(x, y, Mouse{Action: MouseMotion, Button: MouseNone}))
	a.HandleMouse(a.locate(x+60, y-5, Mouse{Action: MouseMotion, Button: MouseNone}))
	if a.card != nil {
		t.Error("card stayed after the pointer left")
	}
	// without mouse hover, resting opens nothing
	a.cfg.Editor.MouseHover = false
	a.HandleMouse(a.locate(x, y, Mouse{Action: MouseMotion, Button: MouseNone}))
	if a.card != nil {
		t.Error("card opened with mouse hover off")
	}
}

func TestPendingCommentsSurviveHeadBase(t *testing.T) {
	a, _ := prApp(t)
	pr := a.gd.pr
	a.gd.rv.pending = []*PrThread{{Path: "a.txt", Line: 10, Side: "RIGHT", Pending: true, Comments: []prComment{{Body: "x"}}}}
	a.openDiff(baseHead)
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 7, pr: pr})
	if len(a.gd.rv.pending) != 1 {
		t.Fatal("Space G then Space g lost the comment")
	}
	// another pull request asks first
	other := &PrInfo{Number: 8, HeadRefName: "feature", BaseRefName: "main"}
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

func TestCommentOnNewFileInOnePane(t *testing.T) {
	a, _ := prApp(t)
	b := filepath.Join(a.gitTopDir(), "b.txt")
	if !a.openFile(b) {
		t.Fatal("b.txt not opened")
	}
	tb := a.CurText()
	dv := a.DiffFor(tb)
	if dv == nil || !dv.Single {
		t.Fatalf("new file against a PR is not one pane: %+v", dv)
	}
	if !dv.inHunk(0) || !dv.inHunk(1) || dv.inHunk(len(dv.NewIdx)) {
		t.Error("inHunk of a one-pane file")
	}
	tb.Buf.Anchor, tb.Buf.Cur = nil, buffer.Pos{Row: 1}
	a.reviewKey("c")
	if a.composer == nil {
		t.Fatalf("no editor: %s", a.msg)
	}
	a.composer.Ta.SetValue("新規ファイルへ")
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

func TestComposerOverlay(t *testing.T) {
	a, _ := prApp(t)
	a.compose("title", "", []string{"x"}, func(string) []Effect { return nil })
	a.composer.Ta.SetValue("# 見出し\n本文")
	ov := a.ComposerOverlay(30, 5)
	if ov.Kind != screen.OverlayReviewCard || ov.Title != "title" || ov.Compose == nil || ov.Compose.Text != "# 見出し\n本文" {
		t.Fatalf("overlay = %+v", ov)
	}
	if ov.Compose.Cursor != (screen.Pos{Line: 1, Col: 2}) || ov.Compose.PreviewLabel == "" || len(ov.Lines) == 0 || len(ov.Lines) > 4 {
		t.Errorf("compose = %+v, %d preview rows", *ov.Compose, len(ov.Lines))
	}
	if ov.Hint != i18n.T("rv.hint.inputsugg") {
		t.Errorf("hint = %q", ov.Hint)
	}
	if ov = a.ComposerOverlay(30, 0); ov.Compose.PreviewLabel != "" || len(ov.Lines) != 0 {
		t.Errorf("no room for a preview: %+v", ov)
	}
}


func TestCheckoutPRStashes(t *testing.T) {
	a, _ := gitDiffApp(t)
	pr := &PrInfo{Number: 9, HeadRefName: "other", BaseRefName: "main"}
	msg := cmdOf(a, a.checkoutPR(pr, true))().(prCheckedOutMsg)
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
	runCmd(a, cmdOf(a, a.reviewKey("y")))
	if a.clipboard != url || a.msgErr || !strings.Contains(a.msg, url) {
		t.Fatalf("copy: clip %q msg %q", a.clipboard, a.msg)
	}
	reset()
	runCmd(a, cmdOf(a, a.reviewKey("o")))
	if opened != url || a.clipboard != "" || a.msgErr {
		t.Fatalf("open: opened %q clip %q msg %q", opened, a.clipboard, a.msg)
	}
	reset()
	remote = true
	runCmd(a, cmdOf(a, a.reviewKey("o")))
	if opened != "" || a.clipboard != url || a.msgErr || !strings.Contains(a.msg, "SSH") {
		t.Fatalf("ssh: opened %q clip %q msg %q", opened, a.clipboard, a.msg)
	}
	remote = false
	reset()
	openErr = fmt.Errorf("boom")
	runCmd(a, cmdOf(a, a.reviewKey("o")))
	if a.clipboard != url || a.msgErr || strings.Contains(a.msg, "boom") {
		t.Fatalf("fallback: clip %q msg %q err %v", a.clipboard, a.msg, a.msgErr)
	}
	// Not comparing with a PR base: palette commands only say so.
	reset()
	a.gd.base = baseHead
	runCmd(a, cmdOf(a, a.reviewCommand("y")))
	runCmd(a, cmdOf(a, a.reviewCommand("o")))
	if opened != "" || a.clipboard != "" || a.msg != i18n.T("rv.needpr") {
		t.Fatalf("needpr: opened %q clip %q msg %q", opened, a.clipboard, a.msg)
	}
}
