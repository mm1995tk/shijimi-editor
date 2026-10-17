package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectHelixLanguages(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".helix"), 0o755)
	os.WriteFile(filepath.Join(root, ".helix", "languages.toml"), []byte(`
[language-server.my-gopls]
command = "gopls"
args = ["serve"]
config = { gofumpt = true }

[[language]]
name = "go"
language-servers = ["my-gopls"]
indent = { tab-width = 4, unit = "\t" }

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
	os.MkdirAll(filepath.Join(root, ".gh-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".gh-editor", "config.toml"), []byte("[editor]\nfile-icons = false\n"), 0o644)
	if Load(root).Editor.FileIcons {
		t.Fatal("file-icons = false should disable icons")
	}
}

func TestSearchConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".gh-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".gh-editor", "config.toml"), []byte(`
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
	os.MkdirAll(filepath.Join(root, ".helix"), 0o755)
	os.WriteFile(filepath.Join(root, ".helix", "config.toml"), []byte("[browser]\nchromium = \"/helix\"\nlisten-host = \"0.0.0.0\"\n"), 0o644)
	if got := Load(root).Browser.ListenHost; got != "127.0.0.1" {
		t.Fatalf(".helix listen-host must be ignored, got %q", got)
	}
	if got := Load(root).Browser.Chromium; got != "" {
		t.Fatalf(".helix must be ignored, got %q", got)
	}
	os.MkdirAll(filepath.Join(cfg, "gh-editor"), 0o755)
	os.WriteFile(filepath.Join(cfg, "gh-editor", "config.toml"), []byte("[browser]\nchromium = \"/user\"\n"), 0o644)
	if got := Load(root).Browser.Chromium; got != "/user" {
		t.Fatalf("user = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".gh-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".gh-editor", "config.toml"), []byte("[browser]\nchromium = \"/proj\"\nlisten-host = \"::1\"\n"), 0o644)
	if got := Load(root).Browser.ListenHost; got != "::1" {
		t.Fatalf("listen-host = %q", got)
	}
	if got := Load(root).Browser.Chromium; got != "/proj" {
		t.Fatalf("project = %q", got)
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
	os.MkdirAll(filepath.Join(root, ".helix"), 0o755)
	os.WriteFile(filepath.Join(root, ".helix", "config.toml"), []byte(`
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
	os.MkdirAll(filepath.Join(root, ".helix"), 0o755)
	os.WriteFile(filepath.Join(root, ".helix", "config.toml"), []byte("theme = \"github_dark\"\n"), 0o644)
	if got := Load(root).Editor.Theme; got != "github_dark" {
		t.Fatalf("top-level theme = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".gh-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".gh-editor", "config.toml"), []byte("[editor]\ntheme = \"light\"\n"), 0o644)
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
	os.MkdirAll(filepath.Join(root, ".gh-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".gh-editor", "config.nix"), []byte(`{
  keymap = "vscode";
  search.exclude = [ "*.lock" ];
  editor = {
    line-number = "relative";
    auto-save.after-delay = { enable = true; timeout = 250; };
  };
}`), 0o644)
	os.WriteFile(filepath.Join(root, ".gh-editor", "languages.nix"), []byte(`{
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
	os.MkdirAll(filepath.Join(root, ".gh-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".gh-editor", "config.toml"), []byte("[editor]\nmouse-hover = false\n"), 0o644)
	if Load(root).Editor.MouseHover {
		t.Fatal("mouse-hover = false should disable it")
	}
}
