package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/acp/acptest"
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// TestMain hands the process to the scripted agent first: fakeAgent runs this
// test binary as the agent, and without this the child would run every test
// again (and start more children).
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

func logText(a *App) string {
	var sb strings.Builder
	for _, e := range a.Ag.Log {
		sb.WriteString(e.Text)
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
	a.Cfg.Agent = config.Agent{Command: exe, Env: map[string]string{acptest.Env: "1"}}
	t.Cleanup(a.Close)
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitDiffApp makes a repository with a committed file a.txt (a..e) whose
// working copy has "b" removed and "x" added after "d", plus an untracked file.
func gitDiffApp(t *testing.T) (*App, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte("a\nb\nc\nd\ne\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	os.WriteFile(file, []byte("a\nc\nd\nx\ne\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n1\nn2\n"), 0o644)
	a := New(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Gd.On = true
	a.Gd.Base = core.BaseHead
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BaseHead, nil).(core.ChangesLoadedMsg))
	return a, file
}

func openNewFile(t *testing.T, a *App) *core.DiffView {
	t.Helper()
	for i, f := range a.Gd.Files {
		if f.Path() == "new.txt" {
			a.Gd.Sel = i
		}
	}
	a.OpenChange(a.Gd.File())
	dv := a.DiffFor(a.CurText())
	if dv == nil {
		t.Fatal("no diff view")
	}
	return dv
}

var (
	ctrlD = core.Key{Code: 'd', Mod: core.ModCtrl}
	ctrlU = core.Key{Code: 'u', Mod: core.ModCtrl}
)

var (
	ctrlAltD = core.Key{Code: 'd', Mod: core.ModCtrl | core.ModAlt}
	ctrlAltU = core.Key{Code: 'u', Mod: core.ModCtrl | core.ModAlt}
)

func keys(a *App, ks ...string) {
	for _, k := range ks {
		var m core.Key
		switch k {
		case "esc":
			m = core.Key{Code: KeyEscape}
		case "enter":
			m = core.Key{Code: KeyEnter}
		case "space":
			m = core.Key{Code: KeySpace, Text: " "}
		default:
			m = textKey(k)
		}
		a.Update(m)
	}
}

// textKey is a key press that types s (several runes arrive as one message).
func textKey(s string) core.Key {
	r, _ := utf8.DecodeRuneInString(s)
	if utf8.RuneCountInString(s) > 1 {
		r = KeyExtended
	}
	return core.Key{Code: r, Text: s}
}

// altKey is Alt held with the key that types s.
func altKey(s string) core.Key {
	r, _ := utf8.DecodeRuneInString(s)
	return core.Key{Code: r, Mod: core.ModAlt}
}

func newTestApp(t *testing.T, content string) *App {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte(content), 0o644)
	a := New(dir, p)
	a.Helix = true
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a
}

func newGoApp(t *testing.T, src string) (*App, *core.Tab) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	os.WriteFile(p, []byte(src), 0o644)
	a := New(dir, p)
	a.Helix = true
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a, a.CurText()
}

type fakeHost struct{ clip, url []string }

const goplsHover = "```go\nfunc Println(a ...any) (n int, err error)\n```\n\n---\n\nPrintln formats using the default formats for its operands and writes to standard output. See `fmt` for [details](https://pkg.go.dev/fmt).\n"

var backslash = core.Key{Code: '\\', Text: "\\"}

const mdSample = "# 見出し\n\n段落に **太字** と `code` と [リンク](http://x)。\n\n- 項目1\n- 項目2\n  - 入れ子\n\n1. 一\n2. 二\n\n```go\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```mermaid\ngraph TD; A-->B\n```\n"

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h)))
	f.Close()
}

func paneTab(src string) *core.Tab {
	b := buffer.New()
	b.Lines = nil
	for l := range strings.SplitSeq(src, "\n") {
		b.Lines = append(b.Lines, []rune(l))
	}
	return core.NewTab(b)
}

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
	pr := &core.PrInfo{ID: "PR_1", Number: 7, Title: "t", State: "OPEN", BaseRefName: "main", HeadRefName: "feature", HeadRefOid: gitOut(t, dir, "rev-parse", "HEAD")}
	a.ShowDiff(core.BasePR, pr)
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BasePR, pr).(core.ChangesLoadedMsg))
	a.Focus = core.FocusEditor
	return a, file
}

func writeFiles(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	ps := make([]string, len(names))
	for i, n := range names {
		ps[i] = filepath.Join(dir, n)
		if err := os.WriteFile(ps[i], []byte(n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ps
}

// splitApp is an app with a.txt open on the left and b.txt on the right.
func splitApp(t *testing.T) (*App, []string) {
	t.Helper()
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt", "c.txt")
	a.Update(core.Key{Code: KeyEscape})
	a.OpenFile(ps[0])
	a.TrackLast()
	a.OpenFile(a.Tabs[0].Path())
	a.TrackLast()
	keys(a, "space", "w", "v")
	return a, ps
}

func (h *fakeHost) WriteClipboard(s string) { h.clip = append(h.clip, s) }

func (h *fakeHost) OpenURL(u string) { h.url = append(h.url, u) }
