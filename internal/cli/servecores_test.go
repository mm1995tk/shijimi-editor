package cli

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// editedFile opens a.txt in a core of root's, with an unsaved edit.
func editedFile(t *testing.T, ws *core.Workspaces, id, root string) string {
	t.Helper()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := ws.Open(id, root)
	if err != nil {
		t.Fatal(err)
	}
	w.Do(func(c *core.Core) {
		if !c.OpenFile(path) {
			t.Fatalf("open: %s", c.Msg)
		}
		c.CurText().Buf.Lines[0] = []rune("ONE")
		c.CurText().Buf.Dirty = true
	})
	return path
}

func writeProjectConfig(t *testing.T, root, text string) {
	t.Helper()
	dir := filepath.Join(root, ".shijimi-editor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// When a workspace switches folders, its files are saved at once (with
// auto-save on, or on-switch = "save") or the edits are thrown away
// (on-switch = "discard"); either way the core is stopped.
func TestReleaseSavesOrDiscards(t *testing.T) {
	for _, c := range []struct {
		name, config, want string
	}{
		{"auto-save on", "", "ONE\n"},
		{"off, save", "[editor.auto-save]\non-switch = \"save\"\n[editor.auto-save.after-delay]\nenable = false\n", "ONE\n"},
		{"off, discard", "[editor.auto-save]\non-switch = \"discard\"\n[editor.auto-save.after-delay]\nenable = false\n", "one\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if c.config != "" {
				writeProjectConfig(t, root, c.config)
			}
			ws := core.NewWorkspaces(core.NewShared(), nil)
			defer ws.CloseAll()
			path := editedFile(t, ws, "w1", root)
			serveCores{ws, slog.Default()}.Release("w1")
			if b, _ := os.ReadFile(path); string(b) != c.want {
				t.Errorf("a.txt = %q, want %q", b, c.want)
			}
			if ws.Lookup("w1") != nil {
				t.Error("the core is still there")
			}
		})
	}
}

// SaveAll keeps the core and its files.
func TestSaveAllKeepsFilesOpen(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	ws := core.NewWorkspaces(core.NewShared(), nil)
	defer ws.CloseAll()
	path := editedFile(t, ws, "w1", root)
	if failed := (serveCores{ws, slog.Default()}).SaveAll("w1"); len(failed) != 0 {
		t.Fatalf("failed: %v", failed)
	}
	if b, _ := os.ReadFile(path); string(b) != "ONE\n" {
		t.Errorf("a.txt = %q", b)
	}
	if got := (serveCores{ws, slog.Default()}).Files("w1"); len(got) != 1 {
		t.Errorf("files = %v", got)
	}
}
