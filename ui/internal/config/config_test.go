package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
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

// The whole message is in the language that is set when it is printed, the
// type names too, not in the language of the moment Load ran.
func TestMessageIsInOneLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang(i18n.Default) })
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	os.MkdirAll(filepath.Join(root, ".shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(root, ".shijimi-editor", "config.toml"), []byte("[editor]\nwatch-files = \"yes\"\n"), 0o644)
	_, err := config.Load(root, opts())
	ce, ok := errors.AsType[*config.Error](err)
	if !ok {
		t.Fatalf("err = %v", err)
	}
	i18n.SetLang(i18n.Japanese)
	if got := i18n.ErrText(err); !strings.Contains(got, "true か false で書いてください（今は 文字列 です）") {
		t.Fatalf("message = %q", got)
	}
	i18n.SetLang(i18n.English)
	if got := i18n.Text(ce.Summary()); !strings.Contains(got, "must be true or false, but it is a string") {
		t.Fatalf("message = %q", got)
	}
}
