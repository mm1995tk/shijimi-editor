package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"

	"github.com/mm1995tk/shijimi-editor/internal/acp/acptest"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
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

func TestAgentNeedsACommand(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.cfg.Agent = config.Agent{}
	if effs := a.runAgent(sgConflict, "k", "t", func() string { return "" }); effs != nil || !a.msgErr || !strings.Contains(a.msg, "[agent]") {
		t.Errorf("msg = %q", a.msg)
	}
	if a.ag.run != nil {
		t.Error("a run started")
	}
}

func TestLSPErrorSuggestionCanBeDismissed(t *testing.T) {
	a := newTestApp(t, "x\n")
	p := a.CurText().Buf.Path
	a.lsp.diags[p] = map[string][]lsp.Diagnostic{"s": {{Severity: 1, Message: "bad"}, {Severity: 1, Message: "worse"}, {Severity: 2, Message: "meh"}}}
	list := a.suggestions()
	if len(list) != 1 || list[0].title != "エラー 2 件を直しますか" {
		t.Fatalf("suggestions = %+v", list)
	}
	if got := a.errorsText(); !strings.Contains(got, "a.txt:1:1: bad\n") || strings.Contains(got, "meh") {
		t.Errorf("errors = %q", got)
	}
	a.SetFocus(FocusAgent)
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

func TestReviewPromptCarriesTheDiff(t *testing.T) {
	got := reviewPrompt(&PrInfo{Number: 7, Title: "t"}, "+added")
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
	a := newApp(dir, "")
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
	args, env := ShellIntegration(zsh)
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
	args, env := ShellIntegration(bash)
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

func TestStoppingTheAgentDropsTheFixOffer(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.sg.fixOffer = true
	a.stopAgent()
	if a.sg.fixOffer {
		t.Error("the fix of a gone session is still offered")
	}
}
