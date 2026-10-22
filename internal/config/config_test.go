package config

import (
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "languages.toml"), []byte(`
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[editor]\nfile-icons = false\n"), 0o644)
	if Load(root).Editor.FileIcons {
		t.Fatal("file-icons = false should disable icons")
	}
}

func TestSearchConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte(`
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
	os.MkdirAll(filepath.Join(cfg, "shijimi"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi", "config.toml"), []byte("[browser]\nlisten-host = \"10.0.0.1\"\n"), 0o644)
	if got := Load(root).Browser.ListenHost; got != "10.0.0.1" {
		t.Fatalf("user = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[browser]\nlisten-host = \"::1\"\n"), 0o644)
	if got := Load(root).Browser.ListenHost; got != "::1" {
		t.Fatalf("listen-host = %q", got)
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte(`
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("theme = \"github_dark\"\n"), 0o644)
	if got := Load(root).Editor.Theme; got != "github_dark" {
		t.Fatalf("top-level theme = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[editor]\ntheme = \"light\"\n"), 0o644)
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.nix"), []byte(`{
  keymap = "vscode";
  search.exclude = [ "*.lock" ];
  editor = {
    line-number = "relative";
    auto-save.after-delay = { enable = true; timeout = 250; };
  };
}`), 0o644)
	os.WriteFile(filepath.Join(root, ".shijimi", "languages.nix"), []byte(`{
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[editor]\nmouse-hover = false\n"), 0o644)
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[editor.indent-guides]\nrender = false\n"), 0o644)
	if Load(root).Editor.IndentGuides {
		t.Fatal("render = false should disable indent guides")
	}
}

func TestAgentIsReadOnlyFromTheUserConfig(t *testing.T) {
	root := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(xdg, "shijimi"), 0o755)
	os.WriteFile(filepath.Join(xdg, "shijimi", "config.toml"), []byte(`
[agent]
command = "claude-agent-acp"
args = ["--x"]
environment = { A = "1" }
`), 0o644)
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[agent]\ncommand = \"evil\"\n"), 0o644)
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
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	write := func(s string) {
		os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte(s), 0o644)
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
	os.MkdirAll(filepath.Join(home, "shijimi"), 0o755)
	os.MkdirAll(filepath.Join(root, ".shijimi"), 0o755)
	os.WriteFile(filepath.Join(home, "shijimi", "config.toml"), []byte("[keys.normal]\nh = \"move_char_right\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".shijimi", "config.toml"), []byte("[keys.normal]\nl = \":sh evil\"\n"), 0o644)
	c := Load(root)
	if c.Keys.Normal["h"] != "move_char_right" {
		t.Fatalf("user keys not read: %v", c.Keys.Normal)
	}
	if _, ok := c.Keys.Normal["l"]; ok {
		t.Fatalf("project keys must not be read: %v", c.Keys.Normal)
	}
}
