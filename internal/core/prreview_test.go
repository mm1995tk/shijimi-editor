package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
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
	a := newApp(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	pr := &PrInfo{ID: "PR_1", Number: 7, Title: "t", State: "OPEN", BaseRefName: "main", HeadRefName: "feature", HeadRefOid: gitOut(t, dir, "rev-parse", "HEAD")}
	a.showDiff(basePR, pr)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, pr).(changesLoadedMsg))
	a.focus = FocusEditor
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
	if !a.IsStale("a.txt") || !a.IsStale("new.txt") || a.IsStale("b.txt") {
		t.Errorf("after edit: %v", a.gd.stale)
	}
	os.WriteFile(file, orig, 0o644)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, a.gd.pr).(changesLoadedMsg))
	if a.IsStale("a.txt") {
		t.Error("a.txt still stale after undoing the edit")
	}
	// a head commit that is not here: every file is stale
	pr := *a.gd.pr
	pr.HeadRefOid = strings.Repeat("1", 40)
	a.applyChanges(loadChanges(a.gd.gen, a.root, basePR, &pr).(changesLoadedMsg))
	if !a.gd.staleAll || !a.IsStale("b.txt") {
		t.Error("missing head commit did not make files stale")
	}
}

func TestCommentRefusedOnStaleOrUnsavedFile(t *testing.T) {
	a, _ := prApp(t)
	a.CurText().Buf.Cur = buffer.Pos{Row: 9}
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
	a.CurText().Buf.Insert("x")
	a.reviewKey("c")
	if a.composer != nil || !a.msgErr {
		t.Error("comment on an unsaved buffer")
	}
}

func TestInHunk(t *testing.T) {
	a, _ := prApp(t)
	dv := a.DiffFor(a.CurText())
	for row, want := range map[int]bool{5: false, 6: true, 9: true, 12: true, 13: false} {
		if got := dv.inHunk(row); got != want {
			t.Errorf("row %d: %v", row, got)
		}
	}
}

func TestSuggestion(t *testing.T) {
	th := &PrThread{Line: 3, StartLine: 2, Side: "RIGHT", Comments: []prComment{
		{Body: "前\n```suggestion\nold\n```"},
		{Body: "直しました\n```suggestion\nnew1\nnew2\n```\n後"},
	}}
	if got := suggestion(th); strings.Join(got, "|") != "new1|new2" {
		t.Errorf("got %q", got)
	}
	if got := suggestion(&PrThread{Line: 1, Side: "RIGHT", Comments: []prComment{{Body: "```suggestion\n```"}}}); got == nil || len(got) != 0 {
		t.Errorf("empty suggestion: %q", got)
	}
	if suggestion(&PrThread{Line: 1, Side: "RIGHT", Comments: []prComment{{Body: "```go\nx\n```"}}}) != nil {
		t.Error("a go block is not a suggestion")
	}
}

func TestCardKeys(t *testing.T) {
	a, _ := prApp(t)
	th := &PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Author: "r", Body: "x"}}}
	a.gd.rv.threads = []*PrThread{th}
	a.card = &prCard{Thread: th, Path: "a.txt"}
	a.CurText().Buf.Cur = buffer.Pos{}
	a.handleKey(Key{Code: KeyEnter})
	if a.card != nil || a.CurText().Buf.Cur.Row != 9 {
		t.Errorf("Enter: card=%v row=%d", a.card != nil, a.CurText().Buf.Cur.Row)
	}
	a.card = &prCard{Thread: th, Path: "a.txt"}
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
	a.card = &prCard{Thread: th, Path: "a.txt"}
	a.handleKey(textKey("l"))
	if a.card != nil || a.CurText().Buf.Cur.Col != 1 {
		t.Errorf("l: card=%v col=%d", a.card != nil, a.CurText().Buf.Cur.Col)
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

func TestReviewSentDropsSentComments(t *testing.T) {
	a, _ := prApp(t)
	t1, t2 := &PrThread{Pending: true}, &PrThread{Pending: true}
	a.gd.rv.pending = []*PrThread{t1, t2}
	a.handleReviewSent(reviewSentMsg{num: 7, done: "ok", sent: []*PrThread{t1}})
	if len(a.gd.rv.pending) != 1 || a.gd.rv.pending[0] != t2 {
		t.Errorf("pending = %v", a.gd.rv.pending)
	}
	a.handleReviewSent(reviewSentMsg{num: 7, err: errString("boom"), sent: []*PrThread{t2}})
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
	p := &PrThread{Pending: true}
	a.gd.rv.pending = []*PrThread{p}
	if cmd := a.handleReviewSent(reviewSentMsg{num: 99, sent: []*PrThread{p}}); cmd != nil || len(a.gd.rv.pending) != 1 {
		t.Error("send answer of another PR applied")
	}
	// review loads: an older one or one for another PR is dropped; the open
	// card follows its thread into the new data
	old := &PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Body: "old"}}}
	a.card = &prCard{Thread: old, Path: "a.txt"}
	a.gd.rv.seq = 5
	fresh := &PrThread{ID: "T", Path: "a.txt", Line: 10, Side: "RIGHT", Comments: []prComment{{Body: "old"}, {Body: "reply"}}}
	a.handlePRReview(prReviewMsg{seq: 4, num: 7, data: prReviewData{threads: []*PrThread{fresh}}})
	a.handlePRReview(prReviewMsg{seq: 5, num: 8, data: prReviewData{threads: []*PrThread{fresh}}})
	if len(a.gd.rv.threads) != 0 {
		t.Fatal("stale review load applied")
	}
	a.handlePRReview(prReviewMsg{seq: 5, num: 7, data: prReviewData{threads: []*PrThread{fresh}}})
	if len(a.gd.rv.threads) != 1 || a.card == nil || a.card.Thread != fresh {
		t.Error("card not moved to the reloaded thread")
	}
}

func TestInHunkAroundDeletion(t *testing.T) {
	// base 1..10, buffer has line 5 deleted: the deletion sits between buffer
	// lines 3 and 4 (0-based); 3 lines of context on each side make rows 1..6
	// the hunk
	dv := &DiffView{}
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
	th := &PrThread{ID: "T", Path: "a.txt", Line: 11, StartLine: 10, Side: "RIGHT", Comments: []prComment{{Body: "```suggestion\n```"}}}
	a.gd.rv.threads = []*PrThread{th}
	a.card = &prCard{Thread: th, Path: "a.txt"}
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
	n := len(a.CurText().Buf.Lines) // the empty line after the last newline counts
	th2 := &PrThread{ID: "U", Path: "a.txt", Line: n - 1, Side: "RIGHT", Comments: []prComment{{Body: "```suggestion\n```"}}}
	a.card = &prCard{Thread: th2, Path: "a.txt"}
	a.handleKey(textKey("s"))
	data, _ = os.ReadFile(file)
	if !strings.HasSuffix(string(data), "lines\n") {
		t.Errorf("last line not removed: %q", data[len(data)-20:])
	}
}

func TestEnterOnBaseSideThreadDoesNotMove(t *testing.T) {
	a, _ := prApp(t)
	th := &PrThread{ID: "T", Path: "a.txt", Line: 15, Side: "LEFT", Comments: []prComment{{Body: "x"}}}
	a.card = &prCard{Thread: th, Path: "a.txt"}
	a.CurText().Buf.Cur = buffer.Pos{Row: 2}
	a.handleKey(Key{Code: KeyEnter})
	if a.CurText().Buf.Cur.Row != 2 {
		t.Errorf("moved to %d", a.CurText().Buf.Cur.Row)
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
	pr := &PrInfo{ID: "PR", Number: 1, HeadRefOid: "abc", URL: "https://example/pr/1"}
	threads := []*PrThread{
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
	d, err := loadReview("", &PrInfo{ID: "PR"})
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

func TestCardOverlay(t *testing.T) {
	a, _ := prApp(t)
	th := &PrThread{ID: "T", Path: "a.txt", Line: 11, Side: "RIGHT", Comments: []prComment{
		{Author: "r", Body: "one\n\ntwo\n\nthree\n\nfour"},
		{Author: "me", Body: "reply"},
	}}
	a.card = &prCard{Thread: th, Path: "a.txt", scroll: 99}
	ov := a.CardOverlay(a.card, 40, 4)
	if ov.Kind != screen.OverlayReviewCard || ov.Compose != nil || ov.View == nil || ov.View.Rows != 4 || ov.View.Cols != 40 {
		t.Fatalf("overlay = %+v", ov)
	}
	if ov.View.Top != ov.View.Total-4 || len(ov.Lines) != 4 || ov.Lines[0].No != ov.View.Top {
		t.Errorf("view = %+v, %d lines (scroll is clamped to the last page)", *ov.View, len(ov.Lines))
	}
	if ov.Hint != a.cardHint(a.card) {
		t.Errorf("hint = %q", ov.Hint)
	}
	if ov.Pane != a.CurText().Path() || ov.At == nil || ov.At.Line != 10 {
		t.Errorf("a card in the pane of its file hangs on a line: %+v", ov)
	}
	a.card.scroll = 0
	ov = a.CardOverlay(a.card, 40, 4)
	if got := ov.Lines[0].Spans[0].Text; got != "@r" {
		t.Errorf("first row = %q, want the author", got)
	}
}
