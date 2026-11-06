package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/config"
)

// badProject makes a project whose config.toml has a mistake on line 2.
func badProject(t *testing.T) (dir, file string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir = t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".shijimi-editor"), 0o755)
	file = filepath.Join(dir, ".shijimi-editor", "config.toml")
	if err := os.WriteFile(file, []byte("theme = \"github-dark\"\nunknown-key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

func TestNewFailsOnConfigMistake(t *testing.T) {
	dir, file := badProject(t)
	c, err := New(&testFront{}, dir, "", testBuild())
	var ce *config.Error
	if c != nil || !errors.As(err, &ce) || len(ce.Problems) != 1 {
		t.Fatalf("core = %v, err = %v", c, err)
	}
	if p := ce.Problems[0]; p.File != file || p.Line != 2 || p.Key != "unknown-key" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestNewSingleFailsOnConfigMistake(t *testing.T) {
	dir, _ := badProject(t)
	if _, err := newSingleCore(filepath.Join(dir, "a.txt")); err == nil {
		t.Fatal("no error")
	}
}

// A running editor keeps its settings when :config-reload finds a mistake, and
// says where it is.
func TestConfigReloadKeepsSettingsOnMistake(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".shijimi-editor"), 0o755)
	file := filepath.Join(dir, ".shijimi-editor", "config.toml")
	os.WriteFile(file, []byte("[editor]\nline-number = \"relative\"\n"), 0o644)
	a := newApp(dir, "")
	if a.Cfg.Editor.LineNumber != "relative" {
		t.Fatal("setup")
	}
	os.WriteFile(file, []byte("[editor]\nline-number = \"absolute\"\nfile-icons = 3\n"), 0o644)
	cfg := a.Cfg
	a.ExecCommand("config-reload")
	if a.Cfg != cfg {
		t.Fatal("the settings changed")
	}
	if !a.MsgErr || !strings.Contains(a.text(a.Msg), file+":3") || !strings.Contains(a.text(a.Msg), "editor.file-icons") {
		t.Fatalf("msg = %q (err %v)", a.Msg, a.MsgErr)
	}
}
