package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/msg"
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
	_, err := Load(root, Options{})
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
	path := write("config.toml", "keymap = \"x\"\n\n[editor\nwatch-files = true\n")
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
	path := write("config.toml", "keymap = \"x\"\n\n[editor]\nwatch-files = true\nline-numbers = \"relative\"\n\n[editr]\na = 1\n")
	// an unknown key is a warning: Load goes on, and the known keys are read
	c, err := Load(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ps := c.Warnings
	if len(ps) != 2 {
		t.Fatalf("warnings = %+v", ps)
	}
	if !ps[0].Warning || ps[0].File != path || ps[0].Line != 5 || ps[0].Key != "editor.line-numbers" {
		t.Fatalf("first = %+v", ps[0])
	}
	if ps[1].Line != 7 || ps[1].Key != "editr" {
		t.Fatalf("second = %+v", ps[1])
	}
}

// A wrong value of a known key stays an error, with the unknown key beside it only a warning.
func TestUnknownKeyBesideMistake(t *testing.T) {
	root, _, write := setup(t)
	write("config.toml", "keymap = 1\nbogus = 2\n")
	c, err := Load(root, Options{})
	var ce *Error
	if !errors.As(err, &ce) || len(ce.Problems) != 1 || ce.Problems[0].Key != "keymap" {
		t.Fatalf("err = %v", err)
	}
	if len(c.Warnings) != 1 || c.Warnings[0].Key != "bogus" {
		t.Fatalf("warnings = %+v", c.Warnings)
	}
}

func TestWrongType(t *testing.T) {
	root, _, write := setup(t)
	write("config.toml", `
[editor]
watch-files = "yes"

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
		{3, "editor.watch-files"},
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
		"zero timeout":    "[editor.auto-save.after-delay]\ntimeout = 0\n",
		"keys not string": "[keys.normal.g]\na = \"no_op\"\n",
	} {
		write("config.toml", c)
		if _, err := Load(root, Options{}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestEveryProblemIsListed(t *testing.T) {
	root, _, write := setup(t)
	write("config.toml", "keymap = 1\nbogus = 2\n")
	write("languages.toml", "[[language]]\nname = 3\n")
	ps := problems(t, root)
	if len(ps) != 2 {
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
		"language[1].file-types[1]": 11,
		"language[1].indent.unit":   14,
		"language[2]":               16,
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
	os.WriteFile(p, []byte("[editor]\nwatch-files = 1\n"), 0o644)
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
	path := write("config.nix", `{ editor.watch-files = "yes"; nope = 1; }`)
	ps := problems(t, root)
	if len(ps) != 1 || ps[0].File != path || ps[0].Line != 0 {
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

func TestValidConfigHasNoError(t *testing.T) {
	root, _, _ := setup(t)
	if _, err := Load(root, Options{}); err != nil {
		t.Fatal(err)
	}
}

// The reason is data, type names included: the UI translates all of it in the
// language it has selected, not in the language of the moment Load ran.
func TestReasonIsAMsg(t *testing.T) {
	root, _, write := setup(t)
	write("config.toml", "[editor]\nwatch-files = \"yes\"\n")
	_, err := Load(root, Options{})
	ce, ok := err.(*Error)
	if !ok || len(ce.Problems) != 1 {
		t.Fatalf("err = %v", err)
	}
	want := msg.T("config.err.type", msg.T("config.type.bool"), msg.T("config.type.string"))
	if got := ce.Problems[0].Reason(); got.String() != want.String() {
		t.Fatalf("reason = %v, want %v", got, want)
	}
}

func TestLanguageNameIsRequired(t *testing.T) {
	root, _, write := setup(t)
	for _, c := range []string{"[[language]]\nfile-types = [\"x\"]\n", "[[language]]\nname = \"\"\n"} {
		write("languages.toml", c)
		if _, err := Load(root, Options{}); err == nil {
			t.Errorf("no error for %q", c)
		}
	}
}
