package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// setup makes a user directory and a project directory and returns a function
// that writes a file into the user's directory.
func setup(t *testing.T) (root, userDir string, write func(name, content string) string) {
	t.Helper()
	root = t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	userDir = filepath.Join(xdg, "shijimi-editor")
	os.MkdirAll(userDir, 0o755)
	write = func(name, content string) string {
		p := filepath.Join(userDir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	return
}

func problems(t *testing.T, root string) []Problem {
	t.Helper()
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load returned no error")
	}
	var ce *Error
	if !errors.As(err, &ce) {
		t.Fatalf("error is %T, want *Error", err)
	}
	return ce.Problems
}

func TestSyntaxErrorHasFileAndLine(t *testing.T) {
	root, _, write := setup(t)
	path := write("config.toml", "theme = \"x\"\n\n[editor\nfile-icons = true\n")
	if !strings.Contains(path, "config.toml") {
		t.Fatal(path)
	}
	ps := problems(t, root)
	if len(ps) != 1 || ps[0].File != path || ps[0].Line != 3 {
		t.Fatalf("problems = %+v", ps)
	}
	if !strings.Contains(ps[0].Error(), path+":3: ") {
		t.Fatalf("message = %q", ps[0].Error())
	}
}

func TestUnknownKey(t *testing.T) {
	root, _, write := setup(t)
	path := write("config.toml", "theme = \"x\"\n\n[editor]\nfile-icons = true\nline-numbers = \"relative\"\n\n[editr]\na = 1\n")
	ps := problems(t, root)
	if len(ps) != 2 {
		t.Fatalf("problems = %+v", ps)
	}
	if ps[0].File != path || ps[0].Line != 5 || ps[0].Key != "editor.line-numbers" {
		t.Fatalf("first = %+v", ps[0])
	}
	if ps[1].Line != 7 || ps[1].Key != "editr" {
		t.Fatalf("second = %+v", ps[1])
	}
}

func TestWrongType(t *testing.T) {
	root, _, write := setup(t)
	write("config.toml", `
[editor]
file-icons = "yes"

[editor.auto-save.after-delay]
enable = true
timeout = "soon"

[search]
exclude = ["a", 2]
`)
	ps := problems(t, root)
	want := []struct {
		line int
		key  string
	}{
		{3, "editor.file-icons"},
		{7, "editor.auto-save.after-delay.timeout"},
		{10, "search.exclude[1]"},
	}
	// problems come in the order of the schema walk: sorted by key within a table
	got := map[string]int{}
	for _, p := range ps {
		got[p.Key] = p.Line
	}
	if len(ps) != len(want) {
		t.Fatalf("problems = %+v", ps)
	}
	for _, w := range want {
		if got[w.key] != w.line {
			t.Errorf("%s: line %d, want %d (all: %v)", w.key, got[w.key], w.line, got)
		}
	}
}

func TestWrongValues(t *testing.T) {
	root, _, write := setup(t)
	for name, c := range map[string]string{
		"ui-language":     "ui-language = \"fr\"\n",
		"multiplexer":     "[terminal]\nmultiplexer = \"screen\"\n",
		"negative days":   "[server]\nidle_stop_days = -1\n",
		"zero timeout":    "[editor.auto-save.after-delay]\ntimeout = 0\n",
		"float days":      "[server]\nidle_stop_days = 1.5\n",
		"keys not string": "[keys.normal.g]\na = \"no_op\"\n",
	} {
		write("config.toml", c)
		if _, err := Load(root); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestEveryProblemIsListed(t *testing.T) {
	root, _, write := setup(t)
	write("config.toml", "theme = 1\nbogus = 2\n")
	write("languages.toml", "[[language]]\nname = 3\n")
	ps := problems(t, root)
	if len(ps) != 3 {
		t.Fatalf("problems = %+v", ps)
	}
}

func TestLanguagesMistakes(t *testing.T) {
	root, _, write := setup(t)
	path := write("languages.toml", `
[language-server.my]
command = "x"
timeout = 5

[[language]]
name = "a"

[[language]]
name = "b"
file-types = ["b", 7]

[language.indent]
unit = 4

[[language]]
file-types = ["c"]
`)
	ps := problems(t, root)
	got := map[string]int{}
	for _, p := range ps {
		if p.File != path {
			t.Errorf("file = %s", p.File)
		}
		got[p.Key] = p.Line
	}
	want := map[string]int{
		"language-server.my.timeout": 4,
		"language[1].file-types[1]":  11,
		"language[1].indent.unit":    14,
		"language[2]":                16,
	}
	if len(got) != len(want) {
		t.Fatalf("problems = %+v", ps)
	}
	for k, l := range want {
		if got[k] != l {
			t.Errorf("%s: line %d, want %d (all: %v)", k, got[k], l, got)
		}
	}
}

func TestProjectFileIsChecked(t *testing.T) {
	root, _, _ := setup(t)
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	p := filepath.Join(root, ".shijimi-editor", "config.toml")
	os.WriteFile(p, []byte("[editor]\nmouse-hover = 1\n"), 0o644)
	ps := problems(t, root)
	if len(ps) != 1 || ps[0].File != p || ps[0].Line != 2 {
		t.Fatalf("problems = %+v", ps)
	}
}

func TestNixMistakes(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}
	root, _, write := setup(t)
	path := write("config.nix", `{ editor.file-icons = "yes"; nope = 1; }`)
	ps := problems(t, root)
	if len(ps) != 2 || ps[0].File != path || ps[0].Line != 0 {
		t.Fatalf("problems = %+v", ps)
	}
	write("config.nix", `{ editor = ; }`)
	ps = problems(t, root)
	if len(ps) != 1 || ps[0].File != path || !strings.Contains(ps[0].Error(), path) {
		t.Fatalf("problems = %+v", ps)
	}
	write("config.nix", `[ 1 ]`)
	if ps = problems(t, root); len(ps) != 1 {
		t.Fatalf("problems = %+v", ps)
	}
}

func TestNixWithoutNixIsAnError(t *testing.T) {
	root, _, write := setup(t)
	write("config.nix", `{ }`)
	t.Setenv("PATH", t.TempDir())
	if ps := problems(t, root); len(ps) != 1 {
		t.Fatalf("problems = %+v", ps)
	}
}

// The examples the manual and the README show must keep loading.
func TestDocumentedExamplesLoad(t *testing.T) {
	root, userDir, write := setup(t)
	write("config.toml", `
theme = "github_dark"
keymap = "helix"
ui-language = "en"

[editor]
line-number = "relative"
file-icons = true
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"
watch-files = true
mouse-hover = true

[editor.indent-guides]
render = true

[editor.auto-save.after-delay]
enable = true
timeout = 1000

[search]
exclude = ["*.lock", "dist/**"]
include = []

[browser]
listen-host = "127.0.0.1"

[terminal]
multiplexer = "auto"

[experimental]
multiplexer = true
herdr = true

[agent]
command = "claude-agent-acp"
args = []
environment = { A = "1" }

[server]
idle_stop_days = 7

[keys.normal]
"C-s" = ":write"
`)
	write("languages.toml", `
[language-server.my-gopls]
command = "gopls"
args = ["serve"]
config = { gofumpt = true }
environment = { GOFLAGS = "-tags=integration" }

[[language]]
name = "go"
language-servers = ["my-gopls"]
roots = ["go.mod"]
indent = { tab-width = 4, unit = "\t" }
formatter = { command = "gofumpt", args = ["--path", "%{buffer_name}"] }

[[language]]
name = "mylang"
file-types = ["ml2", { glob = "*.special" }]
language-servers = [{ name = "my-gopls", except-features = ["format"] }]

[language.indent]
unit = "  "
`)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	_ = userDir
}

func TestValidConfigHasNoError(t *testing.T) {
	root, _, _ := setup(t)
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
}

// The whole message is in the language that is set when it is printed, the
// type names too, not in the language of the moment Load ran.
func TestMessageIsInOneLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang(i18n.Default) })
	root, _, write := setup(t)
	write("config.toml", "ui-language = \"ja\"\n[editor]\nfile-icons = \"yes\"\n")
	c, err := Load(root)
	if err == nil {
		t.Fatal("no error")
	}
	i18n.SetLang(c.Editor.UILanguage)
	got := err.Error()
	if !strings.Contains(got, "true か false で書いてください（今は 文字列 です）") {
		t.Fatalf("message = %q", got)
	}
	i18n.SetLang(i18n.English)
	if got := err.Error(); !strings.Contains(got, "must be true or false, but it is a string") {
		t.Fatalf("message = %q", got)
	}
}

func TestLanguageNameIsRequired(t *testing.T) {
	root, _, write := setup(t)
	for _, c := range []string{"[[language]]\nfile-types = [\"x\"]\n", "[[language]]\nname = \"\"\n"} {
		write("languages.toml", c)
		if _, err := Load(root); err == nil {
			t.Errorf("no error for %q", c)
		}
	}
}
