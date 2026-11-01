package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
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
			case nil, core.WatchTick:
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
	other := filepath.Join(a.Root, "b.txt")
	p := newPump(a)
	p.run(a.effectCmd(a.RunAgent(core.SgLSPErrors, "k", "テスト", func() string {
		return "READ:" + open.Buf.Path + "\nWRITE:" + other
	})...))
	p.until(t, "the permission request", func() bool { return len(a.Ag.Perms) == 1 })
	if !a.showAgent {
		t.Error("the panel did not open for the permission request")
	}
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	if !strings.Contains(ansi.Strip(a.render()), "許可を求めています") {
		t.Error("no permission card")
	}
	a.SetFocus(core.FocusAgent)
	_, cmd := a.Update(textKey("y"))
	p.run(cmd)
	p.until(t, "the end of the turn", func() bool { return a.Ag.Run == nil })
	if got := logText(a); !strings.Contains(got, "内容: unsaved saved") || !strings.Contains(got, "許可しました: Edit "+other) {
		t.Errorf("log = %q", got)
	}
	if data, _ := os.ReadFile(other); string(data) != "written by agent\n" {
		t.Errorf("b.txt = %q", data)
	}
	if !a.Gd.On {
		t.Error("diff mode did not open after the turn")
	}
}

func TestAgentDoesNotOverwriteUnsavedEdits(t *testing.T) {
	a := newTestApp(t, "saved\n")
	fakeAgent(t, a)
	open := a.CurText()
	open.Buf.Insert("unsaved ")
	p := newPump(a)
	p.run(a.effectCmd(a.RunAgent(core.SgLSPErrors, "k", "テスト", func() string { return "WRITE:" + open.Buf.Path })...))
	p.until(t, "the permission request", func() bool { return len(a.Ag.Perms) == 1 })
	a.AnswerPermission(true)
	p.until(t, "the end of the turn", func() bool { return a.Ag.Run == nil })
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
	other := filepath.Join(a.Root, "b.txt")
	p := newPump(a)
	p.run(a.effectCmd(a.RunAgent(core.SgLSPErrors, "k", "テスト", func() string { return "WRITE:" + other })...))
	p.until(t, "the permission request", func() bool { return len(a.Ag.Perms) == 1 })
	a.SetFocus(core.FocusAgent)
	a.HandleKey(textKey("n"))
	p.until(t, "the end of the turn", func() bool { return a.Ag.Run == nil })
	if _, err := os.Stat(other); err == nil {
		t.Error("the file was written")
	}
}

func TestCancelEndsTheAgentTurn(t *testing.T) {
	a := newTestApp(t, "x\n")
	fakeAgent(t, a)
	p := newPump(a)
	p.run(a.effectCmd(a.RunAgent(core.SgFailure, "k", "テスト", func() string { return "WAIT" })...))
	p.until(t, "the agent to start", func() bool { return a.Ag.Client != nil })
	time.Sleep(50 * time.Millisecond) // the prompt reaches the agent
	a.SetFocus(core.FocusAgent)
	a.HandleKey(textKey("c"))
	p.until(t, "the end of the turn", func() bool { return a.Ag.Run == nil })
	if got := logText(a); !strings.Contains(got, "中止しました") {
		t.Errorf("log = %q", got)
	}
	if a.Sg.FixOffer {
		t.Error("a cancelled analysis offered a fix")
	}
	// the session stays for the next run
	p.run(a.effectCmd(a.RunAgent(core.SgFailure, "k", "もう一度", func() string { return "SAY:原因はこれです" })...))
	p.until(t, "the second turn", func() bool { return a.Ag.Run == nil && strings.Contains(logText(a), "原因はこれです") })
	if !a.Sg.FixOffer {
		t.Error("the analysis did not offer the fix")
	}
	if s := a.Suggestions(); len(s) == 0 || s[0].Kind != core.SgApplyFix {
		t.Errorf("suggestions = %+v", s)
	}
}

func TestTerminalFailuresAreSuggested(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := &termTab{id: 1, vt: vt.New(vt.Options{Cols: 40, Rows: 5, Scrollback: 100})}
	a.term.sh = tab
	tab.vt.Write([]byte("$ make\r\nboom\r\n\x1b]133;D;2\x07$ "))
	a.ScanTerminals()
	list := a.Suggestions()
	if len(list) != 1 || list[0].Kind != core.SgFailure || list[0].Detail != "ターミナル 1: 終了コード 2" {
		t.Fatalf("suggestions = %+v", list)
	}
	if got := core.FailurePrompt(a.Sg.Fails[0]); !strings.Contains(got, "boom") || !strings.Contains(got, "終了コード 2") {
		t.Errorf("prompt = %q", got)
	}
	// a command that succeeds afterwards clears it
	tab.vt.Write([]byte("ls\r\n\x1b]133;D;0\x07$ "))
	a.ScanTerminals()
	if len(a.Suggestions()) != 0 {
		t.Fatal("success did not clear the failure")
	}
	// Ctrl+C is not a failure
	tab.vt.Write([]byte("\x1b]133;D;130\x07"))
	a.ScanTerminals()
	if len(a.Suggestions()) != 0 {
		t.Fatal("an interrupt was suggested")
	}
	// without shell integration, a test runner's FAIL line is enough
	tab.vt.Write([]byte("\r\n--- FAIL: TestX (0.00s)\r\nok\r\n"))
	a.ScanTerminals()
	if list := a.Suggestions(); len(list) != 1 || !strings.Contains(list[0].Detail, "--- FAIL: TestX") {
		t.Fatalf("suggestions = %+v", list)
	}
	// the same line is not read twice
	a.ClearFailure(1)
	a.ScanTerminals()
	if len(a.Suggestions()) != 0 {
		t.Fatal("an old FAIL line was read again")
	}
	// a closed tab takes its failure with it
	tab.vt.Write([]byte("FAIL\tpkg 0.1s\r\n"))
	a.ScanTerminals()
	a.term.sh = nil
	a.ScanTerminals()
	if len(a.Suggestions()) != 0 {
		t.Fatal("the failure of a closed tab stayed")
	}
}

func TestPanelOpensBesideTheEditor(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Lsp.Diags[a.CurText().Buf.Path] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}}}
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
	a.HandleKey(core.Key{Code: 'i', Mod: core.ModAlt})
	l := a.layout()
	if a.Focus != core.FocusAgent || l.agentW == 0 || l.agentX+l.agentW != a.w || l.rightX+l.rightW+1 != l.agentX {
		t.Fatalf("focus = %v, layout = %+v", a.Focus, l)
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
	a.HandleKey(core.Key{Code: 'i', Mod: core.ModAlt})
	if a.Focus != core.FocusEditor || a.layout().agentW != 0 {
		t.Error("Alt+I did not close the panel")
	}
}

func TestPanelMouse(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	a.showAgent = true
	p := a.CurText().Buf.Path
	a.Lsp.Diags[p] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}}}
	a.Sg.FixOffer = true
	l := a.layout()
	// row 0 is the title, rows 1-2 the first suggestion, rows 3-4 the second
	a.HandleMouse(a.locate(l.agentX+2, l.top+4, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Focus != core.FocusAgent || a.Sg.Sel != 1 {
		t.Errorf("focus = %v, sel = %d", a.Focus, a.Sg.Sel)
	}
	a.HandleMouse(a.locate(l.agentX-1, l.top+5, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.agentX-21, l.top+5, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.agentX-21, l.top+5, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if got := a.layout().agentW; got != l.agentW+20 {
		t.Errorf("width = %d, want %d", got, l.agentW+20)
	}
}

func TestAIReviewBecomesUnsentComments(t *testing.T) {
	a, _ := prApp(t)
	fakeAgent(t, a)
	list := a.Suggestions()
	if len(list) == 0 || list[len(list)-1].Kind != core.SgAIReview {
		t.Fatalf("suggestions = %+v", list)
	}
	reply := "SAY:```json\\n" + `[{"path":"a.txt","line":11,"start_line":10,"body":"範囲"},` +
		`{"path":"a.txt","line":1,"body":"遠い"},{"path":"nope.txt","line":1,"body":"無い"},{"path":"b.txt","body":"全体"}]` + "\\n```"
	p := newPump(a)
	p.run(a.effectCmd(a.RunAgent(core.SgAIReview, "k", "レビュー", func() string { return reply })...))
	p.until(t, "the review", func() bool { return a.Ag.Run == nil })
	got := a.Gd.Rv.Pending
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
	if !strings.Contains(a.Msg, "3 件") || !strings.Contains(a.Msg, "1 件は") {
		t.Errorf("msg = %q", a.Msg)
	}
}

func TestOnlyWritingRunsMayWrite(t *testing.T) {
	a := newTestApp(t, "x\n")
	fakeAgent(t, a)
	other := filepath.Join(a.Root, "b.txt")
	p := newPump(a)
	p.run(a.effectCmd(a.RunAgent(core.SgFailure, "k", "テスト", func() string { return "WRITE:" + other })...))
	p.until(t, "the permission request", func() bool { return len(a.Ag.Perms) == 1 })
	a.AnswerPermission(true)
	p.until(t, "the end of the turn", func() bool { return a.Ag.Run == nil })
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
	a.ScanTerminals()
	a.SetFocus(core.FocusAgent)
	a.HandleKey(textKey("x"))
	// the exit code of the same command arrives a tick later
	tab.vt.Write([]byte("\x1b]133;D;1\x07$ "))
	a.ScanTerminals()
	if len(a.Suggestions()) != 0 {
		t.Fatalf("suggestions = %+v", a.Suggestions())
	}
	// a redrawn line moves the cursor up: reading the FAIL line again keeps the same suggestion
	tab.vt.Write([]byte("\x1b[A\x1b[A"))
	a.ScanTerminals()
	tab.vt.Write([]byte("\r\n\r\n"))
	a.ScanTerminals()
	if len(a.Suggestions()) != 0 || len(a.Sg.Fails) != 1 {
		t.Fatalf("fails = %+v", a.Sg.Fails)
	}
}

func TestFailLinesAfterClearAreRead(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := &termTab{id: 1, vt: vt.New(vt.Options{Cols: 40, Rows: 5, Scrollback: 100})}
	a.term.sh = tab
	tab.vt.Write([]byte("a\r\nb\r\nc\r\n"))
	a.ScanTerminals()
	tab.vt.Write([]byte("\x1b[H\x1b[2J--- FAIL: TestY\r\n"))
	a.ScanTerminals()
	if list := a.Suggestions(); len(list) != 1 || !strings.Contains(list[0].Detail, "TestY") {
		t.Fatalf("suggestions = %+v", list)
	}
}

// pump runs commands the way Bubble Tea does and feeds their messages back
// into the app.
type pump struct {
	a  *App
	ch chan tea.Msg
}

func newPump(a *App) *pump { return &pump{a: a, ch: make(chan tea.Msg, 64)} }

func (p *pump) run(cmd tea.Cmd) {
	if cmd != nil {
		go func() { p.ch <- cmd() }()
	}
}
