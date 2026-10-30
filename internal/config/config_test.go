package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/channel"
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
	c := Load(root)
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

func TestFileIconsOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if !Load(root).Editor.FileIcons {
		t.Fatal("file-icons should default to true")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\nfile-icons = false\n"), 0o644)
	if Load(root).Editor.FileIcons {
		t.Fatal("file-icons = false should disable icons")
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
	c := Load(root)
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
	if got := Load(root).Browser.ListenHost; got != "10.0.0.1" {
		t.Fatalf("user = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[browser]\nlisten-host = \"::1\"\n"), 0o644)
	if got := Load(root).Browser.ListenHost; got != "::1" {
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
	if got := Load(root).Languages["nix"].Servers; len(got) != 1 || got[0] != "nixd" {
		t.Fatalf("nix servers = %v", got)
	}
}

func TestAutoSave(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	c := Load(root)
	if !c.Editor.AutoSave || c.Editor.AutoSaveDelay != time.Second {
		t.Fatalf("default = %+v", c.Editor)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte(`
[editor.auto-save.after-delay]
enable = false
timeout = 2500
`), 0o644)
	c = Load(root)
	if c.Editor.AutoSave || c.Editor.AutoSaveDelay != 2500*time.Millisecond {
		t.Fatalf("override = %+v", c.Editor)
	}
}

func TestThemeOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if Load(root).Editor.Theme != "" {
		t.Fatal("theme should default to empty")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("theme = \"github_dark\"\n"), 0o644)
	if got := Load(root).Editor.Theme; got != "github_dark" {
		t.Fatalf("top-level theme = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\ntheme = \"light\"\n"), 0o644)
	if got := Load(root).Editor.Theme; got != "light" {
		t.Fatalf("editor.theme = %q", got)
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
    line-number = "relative";
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
	c := Load(root)
	if c.Editor.Keymap != "vscode" || c.Editor.LineNumber != "relative" || c.Editor.AutoSaveDelay != 250*time.Millisecond {
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

func TestMouseHoverOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if !Load(root).Editor.MouseHover {
		t.Fatal("mouse-hover should default to true")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\nmouse-hover = false\n"), 0o644)
	if Load(root).Editor.MouseHover {
		t.Fatal("mouse-hover = false should disable it")
	}
}

func TestIndentGuidesOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if !Load(root).Editor.IndentGuides {
		t.Fatal("indent-guides should default to true")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor.indent-guides]\nrender = false\n"), 0o644)
	if Load(root).Editor.IndentGuides {
		t.Fatal("render = false should disable indent guides")
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
	c := Load(root)
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
		os.WriteFile(filepath.Join(d, "config.toml"), []byte("theme = \"github_dark\"\n[editor]\nfile-icons = false\n"), 0o644)
		os.WriteFile(filepath.Join(d, "languages.toml"), []byte("[[language]]\nname = \"mylang\"\nfile-types = [\"ml2\"]\n"), 0o644)
	}
	c := Load(root)
	if c.Editor.Theme != "" || !c.Editor.FileIcons {
		t.Fatalf("helix config.toml must be ignored: %+v", c.Editor)
	}
	if l := c.LanguageFor("a.ml2"); l != nil {
		t.Fatalf("helix languages.toml must be ignored: %+v", l)
	}
}

func TestUILanguageOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if got := Load(root).Editor.UILanguage; got != "en" {
		t.Fatalf("ui-language should default to en, got %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	write := func(s string) {
		os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte(s), 0o644)
	}
	write("ui-language = \"ja\"\n")
	if got := Load(root).Editor.UILanguage; got != "ja" {
		t.Fatalf("ui-language = %q, want ja", got)
	}
	write("ui-language = \"fr\"\n")
	if got := Load(root).Editor.UILanguage; got != "en" {
		t.Fatalf("an unsupported language should keep en, got %q", got)
	}
}

func TestMergeKeys(t *testing.T) {
	c := &Config{}
	c.mergeKeys(`
[keys.normal]
"C-s" = ":write"
h = "move_char_right"
[keys.normal.g]
a = "no_op"
[keys.insert]
"A-x" = "normal_mode"
[keys.select]
j = "extend_line_below"
`)
	if c.Keys.Normal["C-s"] != ":write" || c.Keys.Normal["h"] != "move_char_right" {
		t.Fatalf("normal = %v", c.Keys.Normal)
	}
	if c.Keys.Insert["A-x"] != "normal_mode" || c.Keys.Select["j"] != "extend_line_below" {
		t.Fatalf("insert = %v, select = %v", c.Keys.Insert, c.Keys.Select)
	}
	if len(c.Keys.Invalid) != 1 || c.Keys.Invalid[0] != "keys.normal.g" {
		t.Fatalf("invalid = %v", c.Keys.Invalid)
	}
}

func TestKeysReadOnlyFromUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(home, "shijimi-editor"), 0o755)
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(home, "shijimi-editor", "config.toml"), []byte("[keys.normal]\nh = \"move_char_right\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[keys.normal]\nl = \":sh evil\"\n"), 0o644)
	c := Load(root)
	if c.Keys.Normal["h"] != "move_char_right" {
		t.Fatalf("user keys not read: %v", c.Keys.Normal)
	}
	if _, ok := c.Keys.Normal["l"]; ok {
		t.Fatalf("project keys must not be read: %v", c.Keys.Normal)
	}
}

func goplsConfig(t *testing.T, root string) map[string]any {
	t.Helper()
	s := Load(root).Servers["gopls"]
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

func TestNightlyReadsItsOwnUserDir(t *testing.T) {
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor", "config.toml"), []byte("theme = \"stable\"\n[agent]\ncommand = \"a\"\n"), 0o644)

	// A nightly build without its own directory reads the stable one, [agent] included.
	channel.SetForTest(t, channel.Nightly)
	c := Load(root)
	if c.Editor.Theme != "stable" || c.Agent.Command != "a" {
		t.Fatalf("nightly without its own dir must read shijimi-editor: %+v %+v", c.Editor, c.Agent)
	}
	if !c.Experimental.Default {
		t.Fatal("experimental must default to true on nightly")
	}

	// Once shijimi-editor-nightly exists, the stable directory is not read at all.
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor-nightly"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor-nightly", "config.toml"), []byte("[agent]\ncommand = \"b\"\n"), 0o644)
	c = Load(root)
	if c.Editor.Theme != "" || c.Agent.Command != "b" {
		t.Fatalf("nightly must read only shijimi-editor-nightly: %+v %+v", c.Editor, c.Agent)
	}
	if want := filepath.Join(cfg, "shijimi-editor-nightly", "config.toml"); len(c.Sources) != 1 || c.Sources[0] != want {
		t.Fatalf("sources = %v", c.Sources)
	}

	// The stable build never looks at the nightly directory.
	channel.SetForTest(t, "")
	c = Load(root)
	if c.Editor.Theme != "stable" || c.Agent.Command != "a" || c.Experimental.Default {
		t.Fatalf("stable must read shijimi-editor: %+v %+v %+v", c.Editor, c.Agent, c.Experimental)
	}
}
