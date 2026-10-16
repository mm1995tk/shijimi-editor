package config

import (
	"os"
	"path/filepath"
	"testing"
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
