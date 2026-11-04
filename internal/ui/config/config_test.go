package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/config"
)

func opts() config.Options { return config.Options{Sections: []config.Section{Section}} }

// load is config.Load for a configuration that must be valid.
func load(root string) *Settings {
	c, err := config.Load(root, opts())
	if err != nil {
		panic(err)
	}
	return Of(c)
}

func TestFileIconsOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if !load(root).FileIcons {
		t.Fatal("file-icons should default to true")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\nfile-icons = false\n"), 0o644)
	if load(root).FileIcons {
		t.Fatal("file-icons = false should disable icons")
	}
}

func TestThemeOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if load(root).Theme != "" {
		t.Fatal("theme should default to empty")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("theme = \"github_dark\"\n"), 0o644)
	if got := load(root).Theme; got != "github_dark" {
		t.Fatalf("top-level theme = %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\ntheme = \"light\"\n"), 0o644)
	if got := load(root).Theme; got != "light" {
		t.Fatalf("editor.theme = %q", got)
	}
}

func TestMouseHoverOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if !load(root).MouseHover {
		t.Fatal("mouse-hover should default to true")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\nmouse-hover = false\n"), 0o644)
	if load(root).MouseHover {
		t.Fatal("mouse-hover = false should disable it")
	}
}

func TestIndentGuidesOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if !load(root).IndentGuides {
		t.Fatal("indent-guides should default to true")
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor.indent-guides]\nrender = false\n"), 0o644)
	if load(root).IndentGuides {
		t.Fatal("render = false should disable indent guides")
	}
}

func TestUILanguageOption(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if got := load(root).UILanguage; got != "en" {
		t.Fatalf("ui-language should default to en, got %q", got)
	}
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	write := func(s string) {
		os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte(s), 0o644)
	}
	write("ui-language = \"ja\"\n")
	if got := load(root).UILanguage; got != "ja" {
		t.Fatalf("ui-language = %q, want ja", got)
	}
	write("ui-language = \"fr\"\n")
	if c, err := config.Load(root, opts()); err == nil || Of(c).UILanguage != "en" {
		t.Fatalf("an unsupported language is an error, got %q, %v", Of(c).UILanguage, err)
	}
}

func TestUILanguageIsChecked(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("ui-language = \"fr\"\n"), 0o644)
	if _, err := config.Load(root, opts()); err == nil {
		t.Fatal("an unsupported language must be an error")
	}
}
