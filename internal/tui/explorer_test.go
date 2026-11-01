package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// render fits the tree to h rows and draws it, the way View does.
func renderExplorer(e *core.Explorer, w, h int, focused bool, activePath string) []string {
	e.EnsureVisible(h - 1)
	return drawExplorer(e.Screen(h, activePath), w, h, focused)
}

func TestExplorerRenderIndentGuides(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "f.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e := core.NewExplorer(dir)
	e.Reveal(filepath.Join(dir, "a", "b", "f.txt"))
	lines := renderExplorer(e, 40, 4, false, "")
	want := []string{" ▾ a", " │ ▾ b", " │ │   f.txt"}
	for i, w := range want {
		got := strings.TrimRight(ansi.Strip(lines[i+1]), " ")
		if got != w {
			t.Fatalf("row %d: got %q want %q", i, got, w)
		}
	}
}

func TestExplorerRenderChevronWithIcons(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := core.NewExplorer(dir)
	e.Icons = true
	e.Toggle(e.Flat[0])
	lines := renderExplorer(e, 40, 3, false, "")
	if got := ansi.Strip(lines[1]); !strings.HasPrefix(got, "  ") {
		t.Fatalf("expanded dir: got %q", got)
	}
	if got := ansi.Strip(lines[2]); !strings.HasPrefix(got, "  ") {
		t.Fatalf("collapsed dir: got %q", got)
	}
}

func TestExplorerDimsIgnoredFiles(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir := t.TempDir()
	initCmd := exec.Command("git", "init", "-q")
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		".gitignore": "build/\n*.log\n", "a.log": "", "main.go": "", "build/out.txt": "",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := core.NewExplorer(dir)
	e.Reveal(filepath.Join(dir, "build", "out.txt"))
	byName := map[string]*core.Node{}
	for _, n := range e.Flat {
		byName[n.Name] = n
	}
	// An ignored directory stands for everything under it.
	for _, name := range []string{"build", "out.txt", "a.log"} {
		if !e.IsIgnored(byName[name]) {
			t.Errorf("%s: not ignored", name)
		}
		if l := e.Git[byName[name].Path]; l != "" {
			t.Errorf("%s: git letter %q", name, l)
		}
	}
	for _, name := range []string{".gitignore", "main.go"} {
		if e.IsIgnored(byName[name]) {
			t.Errorf("%s: ignored", name)
		}
		if l := e.Git[byName[name].Path]; l != "U" {
			t.Errorf("%s: git letter %q want U", name, l)
		}
	}
	// Ignored rows are dimmed and carry no status letter.
	var r, g, b int
	if _, err := fmt.Sscanf(core.CurTheme.Dim, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("theme Dim %q: %v", core.CurTheme.Dim, err)
	}
	seq := fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
	for i, n := range e.Flat {
		line := renderExplorer(e, 40, len(e.Flat)+1, false, "")[i+1]
		plain := strings.TrimRight(ansi.Strip(line), " ")
		if e.IsIgnored(n) {
			if !strings.Contains(line, seq) {
				t.Errorf("%s: not dimmed: %q", n.Name, line)
			}
			if strings.HasSuffix(plain, " U") {
				t.Errorf("%s: has status letter: %q", n.Name, plain)
			}
		} else if !strings.HasSuffix(plain, " U") {
			t.Errorf("%s: missing status letter: %q", n.Name, plain)
		}
	}
}
