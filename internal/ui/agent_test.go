package ui

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

func TestMain(m *testing.M) {
	acptest.Main()
	// The assertions read the Japanese strings, so the tests run with
	// ui-language = "ja" in an otherwise empty config directory (which also
	// keeps the developer's own config out of the tests).
	cfgDir, err := os.MkdirTemp("", "shijimi-ui-test")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "shijimi-editor"), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "shijimi-editor", "config.toml"), []byte("ui-language = \"ja\"\n"), 0o644); err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", cfgDir)
	os.Setenv("HOME", cfgDir)
	i18n.SetLang(i18n.Japanese)
	code := m.Run()
	os.RemoveAll(cfgDir)
	os.Exit(code)
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

func logText(a *App) string {
	var sb strings.Builder
	for _, e := range a.ag.log {
		sb.WriteString(e.text)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// fakeAgent points the app at the scripted agent in acptest.
func fakeAgent(t *testing.T, a *App) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a.cfg.Agent = config.Agent{Command: exe, Env: map[string]string{acptest.Env: "1"}}
	t.Cleanup(a.Close)
}

func TestAgentReadsTheBufferAndWritesAfterPermission(t *testing.T) {
	a := newTestApp(t, "saved\n")
	fakeAgent(t, a)
	open := a.curText()
	open.buf.Insert("unsaved ")
	other := filepath.Join(a.root, "b.txt")
	p := newPump(a)
	p.run(a.runAgent(sgLSPErrors, "k", "テスト", func() string {
		return "READ:" + open.buf.Path + "\nWRITE:" + other
	}))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	if !a.showAgent {
		t.Error("the panel did not open for the permission request")
	}
	a.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	if !strings.Contains(ansi.Strip(a.render()), "許可を求めています") {
		t.Error("no permission card")
	}
	a.setFocus(focusAgent)
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
	open := a.curText()
	open.buf.Insert("unsaved ")
	p := newPump(a)
	p.run(a.runAgent(sgLSPErrors, "k", "テスト", func() string { return "WRITE:" + open.buf.Path }))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	a.answerPermission(true)
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if got := logText(a); !strings.Contains(got, "書けません") {
		t.Errorf("log = %q", got)
	}
	if data, _ := os.ReadFile(open.buf.Path); string(data) != "saved\n" {
		t.Errorf("file = %q", data)
	}
}

func TestRejectingPermissionKeepsTheFile(t *testing.T) {
	a := newTestApp(t, "saved\n")
	fakeAgent(t, a)
	other := filepath.Join(a.root, "b.txt")
	p := newPump(a)
	p.run(a.runAgent(sgLSPErrors, "k", "テスト", func() string { return "WRITE:" + other }))
	p.until(t, "the permission request", func() bool { return len(a.ag.perms) == 1 })
	a.setFocus(focusAgent)
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
	p.run(a.runAgent(sgFailure, "k", "テスト", func() string { return "WAIT" }))
	p.until(t, "the agent to start", func() bool { return a.ag.client != nil })
	time.Sleep(50 * time.Millisecond) // the prompt reaches the agent
	a.setFocus(focusAgent)
	a.handleKey(textKey("c"))
	p.until(t, "the end of the turn", func() bool { return a.ag.run == nil })
	if got := logText(a); !strings.Contains(got, "中止しました") {
		t.Errorf("log = %q", got)
	}
	if a.sg.fixOffer {
		t.Error("a cancelled analysis offered a fix")
	}
	// the session stays for the next run
	p.run(a.runAgent(sgFailure, "k", "もう一度", func() string { return "SAY:原因はこれです" }))
	p.until(t, "the second turn", func() bool { return a.ag.run == nil && strings.Contains(logText(a), "原因はこれです") })
	if !a.sg.fixOffer {
		t.Error("the analysis did not offer the fix")
	}
	if s := a.suggestions(); len(s) == 0 || s[0].kind != sgApplyFix {
		t.Errorf("suggestions = %+v", s)
	}
}

func TestAgentNeedsACommand(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.cfg.Agent = config.Agent{}
	if cmd := a.runAgent(sgConflict, "k", "t", func() string { return "" }); cmd != nil || !a.msgErr || !strings.Contains(a.msg, "[agent]") {
		t.Errorf("msg = %q", a.msg)
	}
	if a.ag.run != nil {
		t.Error("a run started")
	}
}

func TestLSPErrorSuggestionCanBeDismissed(t *testing.T) {
	a := newTestApp(t, "x\n")
	p := a.curText().buf.Path
	a.lsp.diags[p] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}, {Severity: 1, Message: "worse"}, {Severity: 2, Message: "meh"}}}
	list := a.suggestions()
	if len(list) != 1 || list[0].title != "エラー 2 件を直しますか" {
		t.Fatalf("suggestions = %+v", list)
	}
	if got := a.errorsText(); !strings.Contains(got, "a.txt:1:1: bad\n") || strings.Contains(got, "meh") {
		t.Errorf("errors = %q", got)
	}
	a.setFocus(focusAgent)
	a.handleKey(textKey("x"))
	if len(a.suggestions()) != 0 {
		t.Fatal("x did not dismiss")
	}
	// one more error is the same condition: it stays hidden
	a.lsp.diags[p]["s"] = append(a.lsp.diags[p]["s"], lsp.Diagnostic{Severity: 1, Message: "third"})
	a.pruneDismissed()
	if len(a.suggestions()) != 0 {
		t.Fatal("the dismissed suggestion came back while errors remained")
	}
	// once the errors are gone, new ones are suggested again
	a.lsp.diags[p]["s"] = nil
	a.pruneDismissed()
	a.lsp.diags[p]["s"] = []lsp.Diagnostic{{Severity: 1, Message: "new"}}
	if len(a.suggestions()) != 1 {
		t.Fatal("new errors were not suggested")
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
	a.clearFailure(1)
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
	a.lsp.diags[a.curText().buf.Path] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}}}
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
	if a.focus != focusAgent || l.agentW == 0 || l.agentX+l.agentW != a.w || l.rightX+l.rightW+1 != l.agentX {
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
	if a.focus != focusEditor || a.layout().agentW != 0 {
		t.Error("Alt+I did not close the panel")
	}
}

func TestPanelMouse(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	a.showAgent = true
	p := a.curText().buf.Path
	a.lsp.diags[p] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}}}
	a.sg.fixOffer = true
	l := a.layout()
	// row 0 is the title, rows 1-2 the first suggestion, rows 3-4 the second
	a.handleMouse(a.locate(Mouse{X: l.agentX + 2, Y: l.top + 4, Action: MousePress, Button: MouseLeft}))
	if a.focus != focusAgent || a.sg.sel != 1 {
		t.Errorf("focus = %v, sel = %d", a.focus, a.sg.sel)
	}
	a.handleMouse(a.locate(Mouse{X: l.agentX - 1, Y: l.top + 5, Action: MousePress, Button: MouseLeft}))
	a.handleMouse(a.locate(Mouse{X: l.agentX - 21, Y: l.top + 5, Action: MouseMotion, Button: MouseLeft}))
	a.handleMouse(a.locate(Mouse{X: l.agentX - 21, Y: l.top + 5, Action: MouseRelease, Button: MouseLeft}))
	if got := a.layout().agentW; got != l.agentW+20 {
		t.Errorf("width = %d, want %d", got, l.agentW+20)
	}
}

func TestParseReview(t *testing.T) {
	cs, err := parseReview("見ました。\n```json\n[{\"path\": \"a.go\", \"line\": 3, \"body\": \"x\"}]\n```\n")
	if err != nil || len(cs) != 1 || cs[0].Path != "a.go" || cs[0].Line != 3 {
		t.Errorf("cs = %+v, %v", cs, err)
	}
	if cs, err := parseReview(" [] "); err != nil || len(cs) != 0 {
		t.Errorf("bare = %+v, %v", cs, err)
	}
	if _, err := parseReview("指摘はありません"); err == nil {
		t.Error("no error for a reply without JSON")
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
	p.run(a.runAgent(sgAIReview, "k", "レビュー", func() string { return reply }))
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

func TestReviewPromptCarriesTheDiff(t *testing.T) {
	got := reviewPrompt(&prInfo{Number: 7, Title: "t"}, "+added")
	if !strings.Contains(got, "#7") || !strings.Contains(got, "```diff\n+added\n```") || !strings.Contains(got, "GitHub には投稿しないでください") {
		t.Errorf("prompt = %q", got)
	}
}

func TestRepoProbeFindsConflictsAndMissingPR(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	gitIn(t, dir, "checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("<<<<<<< HEAD\nx\n=======\ny\n>>>>>>> other\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "half.txt"), []byte("<<<<<<< only the start\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "change")
	p := probeRepoState(dir, repoProbe{}, false)
	if p.branch != "feature" || p.base != "main" || p.ahead != 1 {
		t.Errorf("probe = %+v", p)
	}
	if len(p.conflicts) != 1 || p.conflicts[0] != "a.txt" {
		t.Errorf("conflicts = %q", p.conflicts)
	}
	a := New(dir, "")
	a.sg.repo = p
	a.sg.repo.prKnown = true
	var titles []string
	for _, s := range a.suggestions() {
		titles = append(titles, s.title)
	}
	if strings.Join(titles, "|") != "衝突を解消しますか|PR を作りますか" {
		t.Errorf("titles = %q", titles)
	}
	a.sg.repo.prs = []prSummary{{Number: 3, Title: "x"}}
	if s := a.suggestions(); len(s) != 2 || s[1].kind != sgReviewMode || s[1].detail != "#3 x" {
		t.Errorf("suggestions = %+v", s)
	}
}

func TestZshReportsExitCodes(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh がありません")
	}
	t.Setenv("ZDOTDIR", "")
	os.Unsetenv("ZDOTDIR") // the user's files are then read from HOME
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".zshenv"), []byte("export FROM_USER_ZSHENV=1\n"), 0o644)
	args, env := shellIntegration(zsh)
	cmd := exec.Command(zsh, append(args, "-i")...)
	cmd.Env = append([]string{"HOME=" + home, "TERM=dumb", "PATH=" + os.Getenv("PATH")}, env...)
	f, err := pty.Start(cmd)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	f.Write([]byte("false\nprint ok=$FROM_USER_ZSHENV\nexit\n"))
	out := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		out <- sb.String()
	}()
	cmd.Wait()
	f.Close()
	got := <-out
	if !strings.Contains(got, "\x1b]133;D;1\a") || !strings.Contains(got, "\x1b]133;D;0\a") || !strings.Contains(got, "ok=1") {
		t.Errorf("output = %q", got)
	}
}

// runShell feeds input to cmd on a terminal and returns everything it printed.
func runShell(t *testing.T, cmd *exec.Cmd, input string) string {
	t.Helper()
	f, err := pty.Start(cmd)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	f.Write([]byte(input))
	out := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		out <- sb.String()
	}()
	cmd.Wait()
	f.Close()
	return <-out
}

func TestBashReportsExitCodes(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash がありません")
	}
	home := t.TempDir()
	os.WriteFile(filepath.Join(home, ".bash_profile"), []byte("export FROM_PROFILE=1\nHISTCONTROL=ignoreboth\nPROMPT_COMMAND='echo user-hook'\n"), 0o644)
	args, env := shellIntegration(bash)
	cmd := exec.Command(bash, args...)
	cmd.Env = append([]string{"HOME=" + home, "TERM=dumb", "PATH=" + os.Getenv("PATH")}, env...)
	// a repeated command and one starting with a space stay out of the history
	got := runShell(t, cmd, "false\nfalse\n\n true\necho ok=$FROM_PROFILE\nexit\n")
	if strings.Count(got, "\x1b]133;D;1\a") != 2 || strings.Count(got, "\x1b]133;D;") != 4 || !strings.Contains(got, "\x1b]133;D;1\a") || !strings.Contains(got, "\x1b]133;D;0\a") {
		t.Errorf("exit codes: %q", got)
	}
	if !strings.Contains(got, "ok=1") || !strings.Contains(got, "user-hook") {
		t.Errorf("the user's profile was not read: %q", got)
	}
}

func TestOnlyWritingRunsMayWrite(t *testing.T) {
	a := newTestApp(t, "x\n")
	fakeAgent(t, a)
	other := filepath.Join(a.root, "b.txt")
	p := newPump(a)
	p.run(a.runAgent(sgFailure, "k", "テスト", func() string { return "WRITE:" + other }))
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

func TestStoppingTheAgentDropsTheFixOffer(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.sg.fixOffer = true
	a.stopAgent()
	if a.sg.fixOffer {
		t.Error("the fix of a gone session is still offered")
	}
}

func TestADismissedFailureStaysHiddenWhileTheTabFails(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := &termTab{id: 1, vt: vt.New(vt.Options{Cols: 40, Rows: 5, Scrollback: 100})}
	a.term.sh = tab
	tab.vt.Write([]byte("--- FAIL: TestX\r\n"))
	a.scanTerminals()
	a.setFocus(focusAgent)
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
