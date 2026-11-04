package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectLanguages(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "languages.toml"), []byte(`
[language-server.my-gopls]
command = "gopls"
args = ["serve"]
config = { gofumpt = true }

[[language]]
name = "go"
language-servers = ["my-gopls"]
indent = { tab-width = 4, unit = "\t" }
formatter = { command = "gofumpt", args = ["--path", "%{buffer_name}"] }

[[language]]
name = "mylang"
file-types = ["ml2", { glob = "*.special" }]
language-servers = [{ name = "my-gopls", except-features = ["format"] }]
`), 0o644)
	c := load(root)
	l := c.LanguageFor(filepath.Join(root, "main.go"))
	if l == nil || l.Name != "go" || len(l.Servers) != 1 || l.Servers[0] != "my-gopls" || l.IndentUnit != "\t" {
		t.Fatalf("go = %+v", l)
	}
	if f := l.Formatter; f == nil || f.Command != "gofumpt" || len(f.Args) != 2 || f.Args[1] != "%{buffer_name}" {
		t.Fatalf("formatter = %+v", f)
	}
	if len(l.Roots) == 0 {
		t.Fatal("roots from defaults should be kept when not overridden")
	}
	s := c.Servers["my-gopls"]
	if s == nil || s.Args[0] != "serve" || s.Config.(map[string]any)["gofumpt"] != true {
		t.Fatalf("server = %+v", s)
	}
	if l := c.LanguageFor("x/a.special"); l == nil || l.Name != "mylang" || l.Servers[0] != "my-gopls" {
		t.Fatalf("glob = %+v", l)
	}
}

func TestSearchConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte(`
[search]
exclude = ["*.lock", "dist/**"]
include = ["*.go"]
`), 0o644)
	c := load(root)
	if len(c.Search.Exclude) != 2 || c.Search.Exclude[1] != "dist/**" || len(c.Search.Include) != 1 {
		t.Fatalf("search = %+v", c.Search)
	}
}

func TestBrowserConfigOnlyFromOwnDirs(t *testing.T) {
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor", "config.toml"), []byte("[browser]\nlisten-host = \"10.0.0.1\"\n"), 0o644)
	if got := load(root).Browser.ListenHost; got != "10.0.0.1" {
		t.Fatalf("user = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[browser]\nlisten-host = \"::1\"\n"), 0o644)
	if got := load(root).Browser.ListenHost; got != "::1" {
		t.Fatalf("listen-host = %q", got)
	}
}

func TestUserConfigDirFallsBackToDotConfig(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".config", "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(home, ".config", "shijimi-editor", "languages.toml"), []byte(`
[language-server.nixd]
command = "nixd"

[[language]]
name = "nix"
language-servers = ["nixd"]
`), 0o644)
	if got := load(root).Languages["nix"].Servers; len(got) != 1 || got[0] != "nixd" {
		t.Fatalf("nix servers = %v", got)
	}
}

func TestAutoSave(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	c := load(root)
	if !c.Editor.AutoSave || c.Editor.AutoSaveDelay != time.Second {
		t.Fatalf("default = %+v", c.Editor)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte(`
[editor.auto-save]
on-switch = "discard"

[editor.auto-save.after-delay]
enable = false
timeout = 2500
`), 0o644)
	c = load(root)
	if c.Editor.AutoSave || c.Editor.AutoSaveDelay != 2500*time.Millisecond || c.Editor.OnSwitch != "discard" {
		t.Fatalf("override = %+v", c.Editor)
	}
}

// With auto-save off there is no default for what a workspace does with its
// files when it switches folders: Load reports the missing on-switch, at the
// line that turned auto-save off.
func TestOnSwitchRequiredWithoutAutoSave(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	path := filepath.Join(root, ".shijimi-editor", "config.toml")
	os.WriteFile(path, []byte("[editor.auto-save.after-delay]\nenable = false\n"), 0o644)
	_, err := Load(root, Options{})
	var ce *Error
	if !errors.As(err, &ce) || len(ce.Problems) != 1 {
		t.Fatalf("err = %v", err)
	}
	p := ce.Problems[0]
	if p.File != path || p.Line != 2 || p.Key != "editor.auto-save.on-switch" {
		t.Errorf("problem = %+v", p)
	}
	// Auto-save on needs no answer.
	os.WriteFile(path, []byte("[editor.auto-save.after-delay]\nenable = true\n"), 0o644)
	if _, err := Load(root, Options{}); err != nil {
		t.Errorf("auto-save on: %v", err)
	}
	// A value that is neither is a mistake of its own.
	os.WriteFile(path, []byte("[editor.auto-save]\non-switch = \"maybe\"\n"), 0o644)
	if _, err := Load(root, Options{}); err == nil {
		t.Error("on-switch = maybe was accepted")
	}
}

func TestNixConfig(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not available")
	}
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.nix"), []byte(`{
  keymap = "vscode";
  search.exclude = [ "*.lock" ];
  editor = {
    watch-files = false;
    auto-save.after-delay = { enable = true; timeout = 250; };
  };
}`), 0o644)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "languages.nix"), []byte(`{
  language-server.my-gopls = {
    command = "gopls";
    args = [ "serve" ];
    config.gofumpt = true;
    environment.GOFLAGS = "-tags=integration";
  };
  language = [
    { name = "go"; language-servers = [ "my-gopls" ]; indent = { tab-width = 4; unit = "\t"; }; }
    { name = "mylang"; file-types = [ "ml2" { glob = "*.special"; } ]; language-servers = [ { name = "my-gopls"; } ]; }
  ];
}`), 0o644)
	c := load(root)
	if c.Editor.Keymap != "vscode" || c.Editor.WatchFiles || c.Editor.AutoSaveDelay != 250*time.Millisecond {
		t.Fatalf("editor = %+v", c.Editor)
	}
	if len(c.Search.Exclude) != 1 || c.Search.Exclude[0] != "*.lock" {
		t.Fatalf("search = %+v", c.Search)
	}
	l := c.LanguageFor(filepath.Join(root, "main.go"))
	if l == nil || len(l.Servers) != 1 || l.Servers[0] != "my-gopls" || l.IndentUnit != "\t" || len(l.Roots) == 0 {
		t.Fatalf("go = %+v", l)
	}
	s := c.Servers["my-gopls"]
	if s == nil || s.Args[0] != "serve" || s.Env["GOFLAGS"] != "-tags=integration" || s.Config.(map[string]any)["gofumpt"] != true {
		t.Fatalf("server = %+v", s)
	}
	if l := c.LanguageFor("x.special"); l == nil || l.Name != "mylang" {
		t.Fatalf("glob = %+v", l)
	}
}

func TestAgentIsReadOnlyFromTheUserConfig(t *testing.T) {
	root := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(xdg, "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(xdg, "shijimi-editor", "config.toml"), []byte(`
[agent]
command = "claude-agent-acp"
args = ["--x"]
environment = { A = "1" }
`), 0o644)
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[agent]\ncommand = \"evil\"\n"), 0o644)
	c, err := Load(root, Options{})
	if err == nil {
		t.Fatal("a project's [agent] must be an error")
	}
	if c.Agent.Command != "claude-agent-acp" || len(c.Agent.Args) != 1 || c.Agent.Env["A"] != "1" {
		t.Fatalf("agent = %+v", c.Agent)
	}
}

func TestHelixDirsAreIgnored(t *testing.T) {
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	for _, d := range []string{filepath.Join(cfg, "helix"), filepath.Join(root, ".helix")} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "config.toml"), []byte("keymap = \"vscode\"\n[editor]\nwatch-files = false\n"), 0o644)
		os.WriteFile(filepath.Join(d, "languages.toml"), []byte("[[language]]\nname = \"mylang\"\nfile-types = [\"ml2\"]\n"), 0o644)
	}
	c := load(root)
	if c.Editor.Keymap != "helix" || !c.Editor.WatchFiles {
		t.Fatalf("helix config.toml must be ignored: %+v", c.Editor)
	}
	if l := c.LanguageFor("a.ml2"); l != nil {
		t.Fatalf("helix languages.toml must be ignored: %+v", l)
	}
}

func TestMergeKeys(t *testing.T) {
	root := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(xdg, "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(xdg, "shijimi-editor", "config.toml"), []byte(`
[keys.normal]
"C-s" = ":write"
h = "move_char_right"
[keys.insert]
"A-x" = "normal_mode"
[keys.select]
j = "extend_line_below"
`), 0o644)
	c := load(root)
	if c.Keys.Normal["C-s"] != ":write" || c.Keys.Normal["h"] != "move_char_right" {
		t.Fatalf("normal = %v", c.Keys.Normal)
	}
	if c.Keys.Insert["A-x"] != "normal_mode" || c.Keys.Select["j"] != "extend_line_below" {
		t.Fatalf("insert = %v, select = %v", c.Keys.Insert, c.Keys.Select)
	}
}

// [agent], [server] and [keys] in a project's config are not read, and say so
// instead of being ignored in silence.
func TestUserOnlySectionsInProjectAreErrors(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[keys.normal]\nl = \":sh evil\"\n"), 0o644)
	c, err := Load(root, Options{})
	if err == nil {
		t.Fatal("a project's [keys] must be an error")
	}
	if _, ok := c.Keys.Normal["l"]; ok {
		t.Fatalf("project keys must not be read: %v", c.Keys.Normal)
	}
}

func goplsConfig(t *testing.T, root string) map[string]any {
	t.Helper()
	s := load(root).Servers["gopls"]
	if s == nil {
		t.Fatal("no gopls server")
	}
	m, ok := s.Config.(map[string]any)
	if !ok {
		t.Fatalf("gopls config = %#v", s.Config)
	}
	return m
}

func TestGoplsSemanticTokensDefault(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if m := goplsConfig(t, root); m["semanticTokens"] != true {
		t.Fatalf("config = %#v", m)
	}
}

func TestServerConfigMergesByKey(t *testing.T) {
	cases := []struct {
		name, toml string
		want       map[string]any
	}{
		{"add key", `config = { gofumpt = true }`, map[string]any{"gofumpt": true, "semanticTokens": true}},
		{"override", `config = { semanticTokens = false }`, map[string]any{"semanticTokens": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("HOME", t.TempDir())
			os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
			os.WriteFile(filepath.Join(root, ".shijimi-editor", "languages.toml"),
				[]byte("[language-server.gopls]\n"+tc.toml+"\n"), 0o644)
			m := goplsConfig(t, root)
			if len(m) != len(tc.want) {
				t.Fatalf("config = %#v, want %#v", m, tc.want)
			}
			for k, v := range tc.want {
				if m[k] != v {
					t.Fatalf("config = %#v, want %#v", m, tc.want)
				}
			}
		})
	}
}

func TestMergeConfigNested(t *testing.T) {
	base := map[string]any{"a": map[string]any{"x": 1, "y": 2}, "l": []any{1, 2}}
	over := map[string]any{"a": map[string]any{"y": 3}, "l": []any{9}}
	m := mergeConfig(base, over).(map[string]any)
	a := m["a"].(map[string]any)
	if a["x"] != 1 || a["y"] != 3 || len(m["l"].([]any)) != 1 {
		t.Fatalf("merged = %#v", m)
	}
}

var nightly = Options{DirSuffix: "-nightly"}

func TestNightlyReadsItsOwnUserDir(t *testing.T) {
	o := Options{}
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor", "config.toml"), []byte("keymap = \"stable\"\n[agent]\ncommand = \"a\"\n"), 0o644)

	// A nightly build without its own directory reads the stable one, [agent] included.
	o = nightly
	c := loadWith(root, o)
	if c.Editor.Keymap != "stable" || c.Agent.Command != "a" {
		t.Fatalf("nightly without its own dir must read shijimi-editor: %+v %+v", c.Editor, c.Agent)
	}

	// Once shijimi-editor-nightly exists, the stable directory is not read at all.
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor-nightly"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor-nightly", "config.toml"), []byte("[agent]\ncommand = \"b\"\n"), 0o644)
	c = loadWith(root, o)
	if c.Editor.Keymap != "helix" || c.Agent.Command != "b" {
		t.Fatalf("nightly must read only shijimi-editor-nightly: %+v %+v", c.Editor, c.Agent)
	}
	if want := filepath.Join(cfg, "shijimi-editor-nightly", "config.toml"); len(c.Sources) != 1 || c.Sources[0] != want {
		t.Fatalf("sources = %v", c.Sources)
	}

	// The stable build never looks at the nightly directory.
	o = Options{}
	c = loadWith(root, o)
	if c.Editor.Keymap != "stable" || c.Agent.Command != "a" {
		t.Fatalf("stable must read shijimi-editor: %+v %+v", c.Editor, c.Agent)
	}
}

// load is Load for a configuration that must be valid, with no Options.
func load(root string) *Config { return loadWith(root, Options{}) }

func loadWith(root string, o Options) *Config {
	c, err := Load(root, o)
	if err != nil {
		panic(err)
	}
	return c
}

// testValue is a section's setting that counts how it was merged.
type testValue struct{ merged []bool }

func (v *testValue) Merge(raw map[string]any, user bool) {
	if _, ok := raw["colour"]; ok {
		v.merged = append(v.merged, user)
	}
}

// A section's keys are checked with the core's, shared tables included, and
// its Value reads each file in order.
func TestSections(t *testing.T) {
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(cfg, "shijimi-editor")
	os.MkdirAll(dir, 0o755)
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("colour = \"red\"\n[editor]\nwatch-files = false\nsize = 3\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("colour = \"blue\"\n"), 0o644)
	sec := Section{
		Name: "test",
		Schema: map[string]Field{
			"colour": String,
			"editor": Table(map[string]Field{"size": Int}),
		},
		New: func() Value { return &testValue{} },
	}
	c, err := Load(root, Options{Sections: []Section{sec}})
	if err != nil {
		t.Fatal(err)
	}
	if v := c.Value("test").(*testValue); len(v.merged) != 2 || !v.merged[0] || v.merged[1] {
		t.Fatalf("merged = %v, want the user's file then the project's", v.merged)
	}
	if c.Editor.WatchFiles {
		t.Fatal("the core's key of a table the section shares was not read")
	}
	if c.Value("none") != nil {
		t.Fatal("a section that was not registered has a Value")
	}
	// Without the section, its keys are mistakes.
	if _, err := Load(root, Options{}); err == nil {
		t.Fatal("a key no one reads was accepted")
	}
}

// loadIn is load for a project whose config.toml says text.
func loadIn(t *testing.T, text string) *Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if text != "" {
		os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
		os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte(text), 0o644)
	}
	return load(root)
}

func TestFileIconsOption(t *testing.T) {
	if !loadIn(t, "").Editor.FileIcons {
		t.Fatal("file-icons should default to true")
	}
	if loadIn(t, "[editor]\nfile-icons = false\n").Editor.FileIcons {
		t.Fatal("file-icons = false should disable icons")
	}
}

func TestThemeOption(t *testing.T) {
	if got := loadIn(t, "").Editor.Theme; got != "" {
		t.Fatalf("theme should default to empty, got %q", got)
	}
	if got := loadIn(t, "theme = \"github_dark\"\n").Editor.Theme; got != "github_dark" {
		t.Fatalf("top-level theme = %q", got)
	}
	if got := loadIn(t, "[editor]\ntheme = \"light\"\n").Editor.Theme; got != "light" {
		t.Fatalf("editor.theme = %q", got)
	}
}

func TestMouseHoverOption(t *testing.T) {
	if !loadIn(t, "").Editor.MouseHover {
		t.Fatal("mouse-hover should default to true")
	}
	if loadIn(t, "[editor]\nmouse-hover = false\n").Editor.MouseHover {
		t.Fatal("mouse-hover = false should disable it")
	}
}

func TestIndentGuidesOption(t *testing.T) {
	if !loadIn(t, "").Editor.IndentGuides {
		t.Fatal("indent-guides should default to true")
	}
	if loadIn(t, "[editor.indent-guides]\nrender = false\n").Editor.IndentGuides {
		t.Fatal("render = false should disable indent guides")
	}
}

func TestLineNumberOption(t *testing.T) {
	if got := loadIn(t, "").Editor.LineNumber; got != "absolute" {
		t.Fatalf("line-number should default to absolute, got %q", got)
	}
	if got := loadIn(t, "[editor]\nline-number = \"relative\"\n").Editor.LineNumber; got != "relative" {
		t.Fatalf("line-number = %q", got)
	}
}
