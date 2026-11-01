package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"

	"github.com/mm1995tk/shijimi-editor/internal/acp/acptest"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

// until handles messages until cond holds.
func (p *pump) until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for !cond() {
		select {
		case msg := <-p.ch:
			switch m := msg.(type) {
			case nil, WatchTick:
			case tea.BatchMsg:
				for _, c := range m {
					p.run(c)
				}
			default:
				_, cmd := p.a.Update(m)
				p.run(cmd)
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s; log: %q", what, logText(p.a))
		}
	}
}

func TestAgentReadsTheBufferAndWritesAfterPermission(t *testing.T) {
	a := newTestApp(t, "saved\n")
	fakeAgent(t, a)
	open := a.CurText()
	open.Buf.Insert("unsaved ")
	other := filepath.Join(a.root, "b.txt")
	p := newPump(a)
	p.run(a.effectCmd(a.runAgent(sgLSPErrors, "k", "テスト", func() string {
		return "READ:" + open.Buf.Path + "\nWRITE:" + other
	})...))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	if !a.showAgent {
		t.Error("the panel did not open for the permission request")
	}
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	if !strings.Contains(ansi.Strip(a.render()), "許可を求めています") {
		t.Error("no permission card")
	}
	a.SetFocus(FocusAgent)
	_, cmd := a.Update(textKey("y"))
	p.run(cmd)
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if got := logText(a); !strings.Contains(got, "内容: unsaved saved") || !strings.Contains(got, "許可しました: Edit "+other) {
		t.Errorf("log = %q", got)
	}
	if data, _ := os.ReadFile(other); string(data) != "written by agent\n" {
		t.Errorf("b.txt = %q", data)
	}
	if !a.gd.on {
		t.Error("diff mode did not open after the turn")
	}
}

func TestAgentDoesNotOverwriteUnsavedEdits(t *testing.T) {
	a := newTestApp(t, "saved\n")
	fakeAgent(t, a)
	open := a.CurText()
	open.Buf.Insert("unsaved ")
	p := newPump(a)
	p.run(a.effectCmd(a.runAgent(sgLSPErrors, "k", "テスト", func() string { return "WRITE:" + open.Buf.Path })...))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	a.answerPermission(true)
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if got := logText(a); !strings.Contains(got, "書けません") {
		t.Errorf("log = %q", got)
	}
	if data, _ := os.ReadFile(open.Buf.Path); string(data) != "saved\n" {
		t.Errorf("file = %q", data)
	}
}

func TestRejectingPermissionKeepsTheFile(t *testing.T) {
	a := newTestApp(t, "saved\n")
	fakeAgent(t, a)
	other := filepath.Join(a.root, "b.txt")
	p := newPump(a)
	p.run(a.effectCmd(a.runAgent(sgLSPErrors, "k", "テスト", func() string { return "WRITE:" + other })...))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	a.SetFocus(FocusAgent)
	a.handleKey(textKey("n"))
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if _, err := os.Stat(other); err == nil {
		t.Error("the file was written")
	}
}

func TestCancelEndsTheAgentTurn(t *testing.T) {
	a := newTestApp(t, "x\n")
	fakeAgent(t, a)
	p := newPump(a)
	p.run(a.effectCmd(a.runAgent(sgFailure, "k", "テスト", func() string { return "WAIT" })...))
	p.until(t, "the agent to start", func() bool { return a.ag.client != nil })
	time.Sleep(50 * time.Millisecond) // the prompt reaches the agent
	a.SetFocus(FocusAgent)
	a.handleKey(textKey("c"))
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if got := logText(a); !strings.Contains(got, "中止しました") {
		t.Errorf("log = %q", got)
	}
	if a.sg.fixOffer {
		t.Error("a cancelled analysis offered a fix")
	}
	// the session stays for the next run
	p.run(a.effectCmd(a.runAgent(sgFailure, "k", "もう一度", func() string { return "SAY:原因はこれです" })...))
	p.until(t, "the second turn", func() bool { return a.ag.run == nil && strings.Contains(logText(a), "原因はこれです") })
	if !a.sg.fixOffer {
		t.Error("the analysis did not offer the fix")
	}
	if s := a.suggestions(); len(s) == 0 || s[0].kind != sgApplyFix {
		t.Errorf("suggestions = %+v", s)
	}
}

func TestTerminalFailuresAreSuggested(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := &termTab{id: 1, vt: vt.New(vt.Options{Cols: 40, Rows: 5, Scrollback: 100})}
	a.term.sh = tab
	tab.vt.Write([]byte("$ make\r\nboom\r\n\x1b]133;D;2\x07$ "))
	a.scanTerminals()
	list := a.suggestions()
	if len(list) != 1 || list[0].kind != sgFailure || list[0].detail != "ターミナル 1: 終了コード 2" {
		t.Fatalf("suggestions = %+v", list)
	}
	if got := failurePrompt(a.sg.fails[0]); !strings.Contains(got, "boom") || !strings.Contains(got, "終了コード 2") {
		t.Errorf("prompt = %q", got)
	}
	// a command that succeeds afterwards clears it
	tab.vt.Write([]byte("ls\r\n\x1b]133;D;0\x07$ "))
	a.scanTerminals()
	if len(a.suggestions()) != 0 {
		t.Fatal("success did not clear the failure")
	}
	// Ctrl+C is not a failure
	tab.vt.Write([]byte("\x1b]133;D;130\x07"))
	a.scanTerminals()
	if len(a.suggestions()) != 0 {
		t.Fatal("an interrupt was suggested")
	}
	// without shell integration, a test runner's FAIL line is enough
	tab.vt.Write([]byte("\r\n--- FAIL: TestX (0.00s)\r\nok\r\n"))
	a.scanTerminals()
	if list := a.suggestions(); len(list) != 1 || !strings.Contains(list[0].detail, "--- FAIL: TestX") {
		t.Fatalf("suggestions = %+v", list)
	}
	// the same line is not read twice
	a.ClearFailure(1)
	a.scanTerminals()
	if len(a.suggestions()) != 0 {
		t.Fatal("an old FAIL line was read again")
	}
	// a closed tab takes its failure with it
	tab.vt.Write([]byte("FAIL\tpkg 0.1s\r\n"))
	a.scanTerminals()
	a.term.sh = nil
	a.scanTerminals()
	if len(a.suggestions()) != 0 {
		t.Fatal("the failure of a closed tab stayed")
	}
}

func TestPanelOpensBesideTheEditor(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.lsp.diags[a.CurText().Buf.Path] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}}}
	// closed at startup, even on a wide screen
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	if l := a.layout(); l.agentW != 0 {
		t.Fatalf("panel shown at startup: %+v", l)
	}
	// while it is closed, the status line counts the suggestions
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !strings.Contains(ansi.Strip(a.renderStatus()), "提案 1（Alt+I）") {
		t.Errorf("status = %q", ansi.Strip(a.renderStatus()))
	}
	a.handleKey(Key{Code: 'i', Mod: ModAlt})
	l := a.layout()
	if a.focus != FocusAgent || l.agentW == 0 || l.agentX+l.agentW != a.w || l.rightX+l.rightW+1 != l.agentX {
		t.Fatalf("focus = %v, layout = %+v", a.focus, l)
	}
	screen := strings.Split(ansi.Strip(a.render()), "\n")
	if len(screen) != a.h {
		t.Fatalf("%d rows", len(screen))
	}
	for i, row := range screen {
		if w := ansi.StringWidth(row); w != a.w {
			t.Errorf("row %d is %d wide: %q", i, w, row)
		}
	}
	if !strings.Contains(strings.Join(screen, "\n"), "エラー 1 件を直しますか") {
		t.Errorf("screen lacks the suggestion:\n%s", strings.Join(screen, "\n"))
	}
	a.handleKey(Key{Code: 'i', Mod: ModAlt})
	if a.focus != FocusEditor || a.layout().agentW != 0 {
		t.Error("Alt+I did not close the panel")
	}
}

func TestPanelMouse(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	a.showAgent = true
	p := a.CurText().Buf.Path
	a.lsp.diags[p] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}}}
	a.sg.fixOffer = true
	l := a.layout()
	// row 0 is the title, rows 1-2 the first suggestion, rows 3-4 the second
	a.HandleMouse(a.locate(l.agentX+2, l.top+4, Mouse{Action: MousePress, Button: MouseLeft}))
	if a.focus != FocusAgent || a.sg.sel != 1 {
		t.Errorf("focus = %v, sel = %d", a.focus, a.sg.sel)
	}
	a.HandleMouse(a.locate(l.agentX-1, l.top+5, Mouse{Action: MousePress, Button: MouseLeft}))
	a.HandleMouse(a.locate(l.agentX-21, l.top+5, Mouse{Action: MouseMotion, Button: MouseLeft}))
	a.HandleMouse(a.locate(l.agentX-21, l.top+5, Mouse{Action: MouseRelease, Button: MouseLeft}))
	if got := a.layout().agentW; got != l.agentW+20 {
		t.Errorf("width = %d, want %d", got, l.agentW+20)
	}
}

func TestAIReviewBecomesUnsentComments(t *testing.T) {
	a, _ := prApp(t)
	fakeAgent(t, a)
	list := a.suggestions()
	if len(list) == 0 || list[len(list)-1].kind != sgAIReview {
		t.Fatalf("suggestions = %+v", list)
	}
	reply := "SAY:```json\\n" + `[{"path":"a.txt","line":11,"start_line":10,"body":"範囲"},` +
		`{"path":"a.txt","line":1,"body":"遠い"},{"path":"nope.txt","line":1,"body":"無い"},{"path":"b.txt","body":"全体"}]` + "\\n```"
	p := newPump(a)
	p.run(a.effectCmd(a.runAgent(sgAIReview, "k", "レビュー", func() string { return reply })...))
	p.until(t, "the review", func() bool { return a.ag.run == nil })
	got := a.gd.rv.pending
	if len(got) != 3 {
		t.Fatalf("pending = %d, log = %q", len(got), logText(a))
	}
	if th := got[0]; th.Path != "a.txt" || th.Line != 11 || th.StartLine != 10 || th.File || th.Comments[0].Author != "AI" {
		t.Errorf("range = %+v", th)
	}
	if th := got[1]; !th.File || !strings.HasPrefix(th.Comments[0].Body, "**1 行目**") {
		t.Errorf("outside the diff = %+v", th)
	}
	if th := got[2]; th.Path != "b.txt" || !th.File {
		t.Errorf("file = %+v", th)
	}
	if !strings.Contains(a.msg, "3 件") || !strings.Contains(a.msg, "1 件は") {
		t.Errorf("msg = %q", a.msg)
	}
}

func TestOnlyWritingRunsMayWrite(t *testing.T) {
	a := newTestApp(t, "x\n")
	fakeAgent(t, a)
	other := filepath.Join(a.root, "b.txt")
	p := newPump(a)
	p.run(a.effectCmd(a.runAgent(sgFailure, "k", "テスト", func() string { return "WRITE:" + other })...))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	a.answerPermission(true)
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if _, err := os.Stat(other); err == nil {
		t.Error("the analysis wrote a file")
	}
	if got := logText(a); !strings.Contains(got, "書き換えられません") {
		t.Errorf("log = %q", got)
	}
}

func TestADismissedFailureStaysHiddenWhileTheTabFails(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := &termTab{id: 1, vt: vt.New(vt.Options{Cols: 40, Rows: 5, Scrollback: 100})}
	a.term.sh = tab
	tab.vt.Write([]byte("--- FAIL: TestX\r\n"))
	a.scanTerminals()
	a.SetFocus(FocusAgent)
	a.handleKey(textKey("x"))
	// the exit code of the same command arrives a tick later
	tab.vt.Write([]byte("\x1b]133;D;1\x07$ "))
	a.scanTerminals()
	if len(a.suggestions()) != 0 {
		t.Fatalf("suggestions = %+v", a.suggestions())
	}
	// a redrawn line moves the cursor up: reading the FAIL line again keeps the same suggestion
	tab.vt.Write([]byte("\x1b[A\x1b[A"))
	a.scanTerminals()
	tab.vt.Write([]byte("\r\n\r\n"))
	a.scanTerminals()
	if len(a.suggestions()) != 0 || len(a.sg.fails) != 1 {
		t.Fatalf("fails = %+v", a.sg.fails)
	}
}

func TestFailLinesAfterClearAreRead(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := &termTab{id: 1, vt: vt.New(vt.Options{Cols: 40, Rows: 5, Scrollback: 100})}
	a.term.sh = tab
	tab.vt.Write([]byte("a\r\nb\r\nc\r\n"))
	a.scanTerminals()
	tab.vt.Write([]byte("\x1b[H\x1b[2J--- FAIL: TestY\r\n"))
	a.scanTerminals()
	if list := a.suggestions(); len(list) != 1 || !strings.Contains(list[0].detail, "TestY") {
		t.Fatalf("suggestions = %+v", list)
	}
}
