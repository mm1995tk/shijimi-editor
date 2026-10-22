package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// New picks the UI language from ui-language, and English without it.
func TestNewSelectsTheConfiguredLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang(i18n.Japanese) })
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := t.TempDir()

	New(dir, "")
	if i18n.Lang() != i18n.English {
		t.Fatalf("language without ui-language = %q, want en", i18n.Lang())
	}
	if got := i18n.T("cmd.quit"); got != "Quit" {
		t.Fatalf("cmd.quit = %q", got)
	}

	if err := os.MkdirAll(filepath.Join(cfg, "shijimi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "shijimi", "config.toml"), []byte("ui-language = \"ja\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	New(dir, "")
	if i18n.Lang() != i18n.Japanese {
		t.Fatalf("language with ui-language = ja is %q", i18n.Lang())
	}
	if got := i18n.T("cmd.quit"); got != "終了" {
		t.Fatalf("cmd.quit = %q", got)
	}
}

// Menu labels and symbol kinds are looked up by key at draw time, so a key
// that is not in the table would show up as the key itself.
func TestTableDrivenLabelsResolve(t *testing.T) {
	for _, items := range [][]spaceItem{spaceItems, reviewItems} {
		for _, it := range items {
			if i18n.T(it.label) == it.label {
				t.Errorf("menu item %q has no string for %q", it.key, it.label)
			}
		}
	}
	for kind := 0; kind <= 27; kind++ {
		if name := lsp.SymbolKindName(kind); name == "" || strings.HasPrefix(name, "sym.") {
			t.Errorf("symbol kind %d has no name: %q", kind, name)
		}
	}
}
