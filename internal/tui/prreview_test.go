package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestStaleFileShowsMarks(t *testing.T) {
	a, file := prApp(t)
	a.Update(tea.WindowSizeMsg{Width: 220, Height: 40})
	a.sideWPref = 40
	a.Gd.Rv.Threads = []*core.PrThread{
		{ID: "T1", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []core.PrComment{{Author: "rev", Body: "hmm"}}},
		{ID: "T2", Path: "a.txt", Side: "RIGHT", File: true, Comments: []core.PrComment{{Author: "rev", Body: "file"}}},
	}
	a.Gd.Rv.Viewed = map[string]bool{"b.txt": true}
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
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BasePR, a.Gd.Pr).(core.ChangesLoadedMsg))
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
	a.ReviewKey("c")
	if a.Composer != nil || !a.MsgErr {
		t.Fatal("comment outside the hunk was accepted")
	}
	// lines 9..11 selected (Helix x on 9, then extended)
	anchor := buffer.Pos{Row: 8}
	b.Anchor, b.Cur = &anchor, buffer.Pos{Row: 11}
	a.ReviewKey("c")
	if a.Composer == nil {
		t.Fatalf("no editor: %s", a.Msg)
	}
	a.Composer.Ta.InsertString("前置き")
	a.HandleComposerKey(core.Key{Code: 's', Mod: core.ModCtrl})
	if v := a.Composer.Ta.Value(); !strings.HasPrefix(v, "前置き\n```suggestion\nlinei\nchanged\nlinek\n```") {
		t.Errorf("suggestion block: %q", v)
	}
	a.Composer.Ta.SetValue("見てください")
	a.HandleComposerKey(core.Key{Code: KeyEnter})
	if a.Composer != nil || len(a.Gd.Rv.Pending) != 1 {
		t.Fatal("comment not added")
	}
	th := a.Gd.Rv.Pending[0]
	if th.Path != "a.txt" || th.StartLine != 9 || th.Line != 11 || th.Side != "RIGHT" || th.Comments[0].Body != "見てください" {
		t.Errorf("thread = %+v", th)
	}
	if st := ansi.Strip(a.renderStatus()); !strings.Contains(st, "未送信コメント 1 件") {
		t.Errorf("status: %q", st)
	}
	// the marker shows on the last line, and Space v t opens the card
	b.Anchor, b.Cur = nil, buffer.Pos{Row: 10}
	a.ReviewKey("t")
	if a.Card == nil || a.Card.Thread != th {
		t.Fatal("card not opened")
	}
	if !strings.Contains(ansi.Strip(a.render()), "見てください") {
		t.Error("card not drawn")
	}
	// x deletes a comment that was not sent
	a.HandleKey(textKey("x"))
	if a.Card != nil || len(a.Gd.Rv.Pending) != 0 {
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
	if a.Pending != "space v" || !strings.Contains(ansi.Strip(a.render()), "承認して送信") {
		t.Fatalf("pending=%q", a.Pending)
	}
	press(a, "C")
	if a.Composer == nil || !strings.Contains(a.Composer.Title, "ファイルへのコメント: a.txt") {
		t.Fatal("Space v C did not open the editor")
	}
	a.Composer.Ta.SetValue("全体")
	a.HandleKey(core.Key{Code: KeyEnter})
	if len(a.Gd.Rv.Pending) != 1 || !a.Gd.Rv.Pending[0].File {
		t.Fatal("file comment not added")
	}
	// outside a pull request, Space v explains itself
	a.OpenDiff(core.BaseHead)
	press(a, " ")
	press(a, "v")
	if a.Pending != "" || !strings.Contains(a.Msg, "PR の base と比べているとき") {
		t.Errorf("pending=%q msg=%q", a.Pending, a.Msg)
	}
}

func TestApplySuggestion(t *testing.T) {
	a, file := prApp(t)
	th := &core.PrThread{ID: "T", Path: "a.txt", Line: 11, StartLine: 10, Side: "RIGHT", Comments: []core.PrComment{{Author: "r", Body: "```suggestion\nfixed\n```"}}}
	a.Gd.Rv.Threads = []*core.PrThread{th}
	a.Card = &core.PrCard{Thread: th, Path: "a.txt"}
	if !strings.Contains(ansi.Strip(a.render()), "s suggestion を適用") {
		t.Error("hint lacks s")
	}
	a.HandleKey(textKey("s"))
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "linei\nfixed\nlinel\n") {
		t.Errorf("file = %q", data)
	}
	if a.Card != nil {
		t.Error("card still open")
	}
	// the file now differs from the pull request: no more suggestions
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BasePR, a.Gd.Pr).(core.ChangesLoadedMsg))
	a.Card = &core.PrCard{Thread: th, Path: "a.txt"}
	a.HandleKey(textKey("s"))
	if !a.MsgErr {
		t.Error("suggestion applied to a stale file")
	}
}

func TestMarkerClickOpensCard(t *testing.T) {
	a, _ := prApp(t)
	th := &core.PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []core.PrComment{{Author: "r", Body: "x"}}}
	a.Gd.Rv.Threads = []*core.PrThread{th}
	a.render()
	l := a.layout()
	g := a.dvGeom(l)
	tb := a.CurText()
	dv := a.DiffFor(tb)
	y := g.Top + dv.NewIdx[9] - dv.Start(tb.Buf.ScrollRow)
	a.HandleMouse(a.locate(g.RightX, y, core.Mouse{Button: core.MouseLeft, Action: core.MousePress}))
	if a.Card == nil || a.Card.Thread != th {
		t.Fatal("click on the marker did not open the card")
	}
	a.HandleMouse(a.locate(g.RightX+30, g.Top, core.Mouse{Button: core.MouseLeft, Action: core.MousePress}))
	if a.Card != nil {
		t.Error("click elsewhere did not close the card")
	}
}

func TestSwitchToPR(t *testing.T) {
	a, _ := gitDiffApp(t) // a.txt is modified and not committed
	pr := &core.PrInfo{Number: 9, HeadRefName: "other", BaseRefName: "main"}
	if cmd := a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 9, Pr: pr}); cmd != nil || a.Confirm == nil {
		t.Fatal("no question about stashing")
	}
	if !strings.Contains(ansi.Strip(a.renderStatus()), "未コミットの変更を stash して切り替えますか？ (y/n)") {
		t.Error("question not shown")
	}
	press(a, "q") // other keys leave the question open
	if a.Confirm == nil {
		t.Fatal("question closed by another key")
	}
	press(a, "n")
	if a.Confirm != nil || a.Gd.Base == core.BasePR {
		t.Fatal("n did not cancel")
	}
	a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 9, Pr: pr})
	if cmd := a.HandleKey(textKey("y")); cmd == nil || a.Confirm != nil {
		t.Fatal("y did not start the checkout")
	}
	// unsaved buffers stop the switch before anything runs
	a.CurText().Buf.Insert("x")
	if cmd := a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 9, Pr: pr}); cmd != nil || a.Confirm != nil || !a.MsgErr {
		t.Error("switched with unsaved buffers")
	}
}

func TestSyncWith(t *testing.T) {
	a, _ := prApp(t)
	head := gitOut(t, a.Root, "rev-parse", "HEAD")
	base := gitOut(t, a.Root, "rev-parse", "main")
	if s := core.SyncWith(a.Root, head); !s.Known || s.Ahead != 0 || s.Behind != 0 {
		t.Errorf("same: %+v", s)
	}
	if s := core.SyncWith(a.Root, base); s.Ahead != 1 || s.Behind != 0 {
		t.Errorf("local ahead: %+v", s)
	}
	a.Gd.Rv.Sync = core.SyncWith(a.Root, base)
	if !strings.Contains(ansi.Strip(a.renderStatus()), "push していないコミットが 1 個あります") {
		t.Error("status lacks the note")
	}
	// r with only the pull request ahead pulls; otherwise it reloads
	a.Gd.Rv.Sync = core.PrSync{Known: true, Behind: 1}
	a.ReloadPR()
	if !strings.Contains(a.Msg, "git pull") {
		t.Errorf("r: %q", a.Msg)
	}
	a.Gd.Rv.Sync = core.PrSync{Known: true, Ahead: 1, Behind: 1}
	a.ReloadPR()
	if strings.Contains(a.Msg, "git pull") {
		t.Error("r pulled a diverged branch")
	}
}

func TestComposerNewlineAndPreview(t *testing.T) {
	a, _ := prApp(t)
	a.Compose("t", "", nil, func(string) []core.Effect { return nil })
	a.Composer.Ta.InsertString("# 見出し")
	a.HandleComposerKey(core.Key{Code: KeyEnter, Mod: core.ModShift})
	a.Composer.Ta.InsertString("本文")
	if v := a.Composer.Ta.Value(); v != "# 見出し\n本文" {
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
	a.HandleMouse(a.locate(l.sideW, 5, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(40, 5, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}))
	a.HandleMouse(a.locate(40, 5, core.Mouse{Action: core.MouseRelease, Button: core.MouseNone}))
	if a.layout().sideW != 40 || a.resizing != core.ResizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}
}

func TestResizeDragInDiffMode(t *testing.T) {
	a, _ := prApp(t)
	l := a.layout()
	a.HandleMouse(a.locate(l.sideW, 5, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(40, 5, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(40, 5, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
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
	a.Cfg.Editor.MouseHover = true
	th := &core.PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []core.PrComment{{Author: "r", Body: "x"}}}
	a.Gd.Rv.Threads = []*core.PrThread{th}
	x, y := markerCell(a, 9)
	a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}))
	if a.Card == nil || !a.Card.ByPointer {
		t.Fatal("resting on the marker did not show the card")
	}
	// a key closes it and goes to the editor: x selects the line, nothing is sent
	cmd := a.HandleKey(textKey("x"))
	if a.Card != nil || cmd != nil {
		t.Errorf("x went to the card: card=%v cmd=%v", a.Card != nil, cmd != nil)
	}
	if _, _, ok := a.CurText().Buf.Selection(); !ok {
		t.Error("x did not reach the editor")
	}
	// leaving the marker closes it
	a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}))
	a.HandleMouse(a.locate(x+60, y-5, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}))
	if a.Card != nil {
		t.Error("card stayed after the pointer left")
	}
	// without mouse hover, resting opens nothing
	a.Cfg.Editor.MouseHover = false
	a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}))
	if a.Card != nil {
		t.Error("card opened with mouse hover off")
	}
}

func TestPendingCommentsSurviveHeadBase(t *testing.T) {
	a, _ := prApp(t)
	pr := a.Gd.Pr
	a.Gd.Rv.Pending = []*core.PrThread{{Path: "a.txt", Line: 10, Side: "RIGHT", Pending: true, Comments: []core.PrComment{{Body: "x"}}}}
	a.OpenDiff(core.BaseHead)
	a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 7, Pr: pr})
	if len(a.Gd.Rv.Pending) != 1 {
		t.Fatal("Space G then Space g lost the comment")
	}
	// another pull request asks first
	other := &core.PrInfo{Number: 8, HeadRefName: "feature", BaseRefName: "main"}
	a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 8, Pr: other})
	if a.Confirm == nil || !strings.Contains(a.Confirm.Prompt, "PR #7 の未送信コメントが 1 件あります") {
		t.Fatal("no question before dropping comments")
	}
	press(a, "n")
	if len(a.Gd.Rv.Pending) != 1 || a.Gd.Pr.Number != 7 {
		t.Error("n dropped the comments")
	}
	a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 8, Pr: other})
	press(a, "y")
	if len(a.Gd.Rv.Pending) != 0 || a.Gd.Pr.Number != 8 {
		t.Errorf("y: pending=%d pr=%d", len(a.Gd.Rv.Pending), a.Gd.Pr.Number)
	}
}

func TestCommentOnNewFileInOnePane(t *testing.T) {
	a, _ := prApp(t)
	b := filepath.Join(a.GitTopDir(), "b.txt")
	if !a.OpenFile(b) {
		t.Fatal("b.txt not opened")
	}
	tb := a.CurText()
	dv := a.DiffFor(tb)
	if dv == nil || !dv.Single {
		t.Fatalf("new file against a PR is not one pane: %+v", dv)
	}
	if !dv.InHunk(0) || !dv.InHunk(1) || dv.InHunk(len(dv.NewIdx)) {
		t.Error("inHunk of a one-pane file")
	}
	tb.Buf.Anchor, tb.Buf.Cur = nil, buffer.Pos{Row: 1}
	a.ReviewKey("c")
	if a.Composer == nil {
		t.Fatalf("no editor: %s", a.Msg)
	}
	a.Composer.Ta.SetValue("新規ファイルへ")
	a.HandleComposerKey(core.Key{Code: KeyEnter})
	if len(a.Gd.Rv.Pending) != 1 {
		t.Fatal("comment not added")
	}
	if th := a.Gd.Rv.Pending[0]; th.Path != "b.txt" || th.Line != 2 || th.Side != "RIGHT" {
		t.Errorf("thread = %+v", th)
	}
	if got := a.Gd.Rv.LineThreads("b.txt", 2); len(got) != 1 {
		t.Errorf("thread not found on line 2: %v", got)
	}
}

func TestComposerOverlay(t *testing.T) {
	a, _ := prApp(t)
	a.Compose("title", "", []string{"x"}, func(string) []core.Effect { return nil })
	a.Composer.Ta.SetValue("# 見出し\n本文")
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
	pr := &core.PrInfo{Number: 9, HeadRefName: "other", BaseRefName: "main"}
	msg := cmdOf(a, a.CheckoutPR(pr, true))().(core.PrCheckedOutMsg)
	if msg.Err == nil {
		t.Skip("gh pr checkout worked here; nothing to check")
	}
	// the checkout failed on the same branch: the stash is popped again
	if got := gitOut(t, a.Root, "stash", "list"); got != "" {
		t.Errorf("stash list = %q", got)
	}
	if got := gitOut(t, a.Root, "status", "--porcelain"); !strings.Contains(got, "a.txt") || !strings.Contains(got, "new.txt") {
		t.Errorf("changes not restored: %q", got)
	}
	a.HandlePRCheckedOut(msg)
	if !a.MsgErr || a.Gd.Base == core.BasePR {
		t.Error("failed checkout opened the pull request")
	}
}

func TestReviewPRURLCommands(t *testing.T) {
	a, _ := prApp(t)
	a.Gd.Pr.URL = "https://github.com/o/r/pull/7"
	oldOpen, oldSSH := core.OpenExternal, core.IsRemoteSession
	defer func() { core.OpenExternal, core.IsRemoteSession = oldOpen, oldSSH }()
	var opened string
	var openErr error
	core.OpenExternal = func(u string) error { opened = u; return openErr }
	remote := false
	core.IsRemoteSession = func() bool { return remote }
	reset := func() { opened, a.Clipboard, a.Msg, a.MsgErr = "", "", "", false }
	const url = "https://github.com/o/r/pull/7"

	reset()
	runCmd(a, cmdOf(a, a.ReviewKey("y")))
	if a.Clipboard != url || a.MsgErr || !strings.Contains(a.Msg, url) {
		t.Fatalf("copy: clip %q msg %q", a.Clipboard, a.Msg)
	}
	reset()
	runCmd(a, cmdOf(a, a.ReviewKey("o")))
	if opened != url || a.Clipboard != "" || a.MsgErr {
		t.Fatalf("open: opened %q clip %q msg %q", opened, a.Clipboard, a.Msg)
	}
	reset()
	remote = true
	runCmd(a, cmdOf(a, a.ReviewKey("o")))
	if opened != "" || a.Clipboard != url || a.MsgErr || !strings.Contains(a.Msg, "SSH") {
		t.Fatalf("ssh: opened %q clip %q msg %q", opened, a.Clipboard, a.Msg)
	}
	remote = false
	reset()
	openErr = fmt.Errorf("boom")
	runCmd(a, cmdOf(a, a.ReviewKey("o")))
	if a.Clipboard != url || a.MsgErr || strings.Contains(a.Msg, "boom") {
		t.Fatalf("fallback: clip %q msg %q err %v", a.Clipboard, a.Msg, a.MsgErr)
	}
	// Not comparing with a PR base: palette commands only say so.
	reset()
	a.Gd.Base = core.BaseHead
	runCmd(a, cmdOf(a, a.ReviewCommand("y")))
	runCmd(a, cmdOf(a, a.ReviewCommand("o")))
	if opened != "" || a.Clipboard != "" || a.Msg != i18n.T("rv.needpr") {
		t.Fatalf("needpr: opened %q clip %q msg %q", opened, a.Clipboard, a.Msg)
	}
}
