package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/config"
)

// The keys of kisui-terminal are checked like shj's own, and a file that has
// them starts shj without a warning.
func TestKisuiKeysAreKnown(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", home)
	os.Mkdir(filepath.Join(home, "work"), 0o755)
	dir := filepath.Join(cfg, "shijimi-editor")
	os.MkdirAll(dir, 0o755)
	write := func(s string) (*config.Config, error) {
		os.WriteFile(filepath.Join(dir, "config.toml"), []byte(s), 0o644)
		return config.Load(root, opts())
	}
	for name, c := range map[string]string{
		"machines":  "[[gui.machines]]\nssh = \"dev-vm\"\nname = \"dev\"\n\n[[gui.machines]]\nssh = \"build\"\n",
		"server":    "[server]\nidle_stop_days = 3\nstart_dir = \"~/work\"\n",
		"zero days": "[server]\nidle_stop_days = 0\n",
	} {
		if cfg, err := write(c); err != nil || len(cfg.Warnings) != 0 {
			t.Errorf("%s: err = %v, warnings = %v", name, err, cfg.Warnings)
		}
	}
	for name, c := range map[string]string{
		"missing ssh":       "[[gui.machines]]\nname = \"dev\"\n",
		"wrong type":        "[[gui.machines]]\nssh = 1\n",
		"negative days":     "[server]\nidle_stop_days = -1\n",
		"float days":        "[server]\nidle_stop_days = 1.5\n",
		"string days":       "[server]\nidle_stop_days = \"x\"\n",
		"missing start_dir": "[server]\nstart_dir = \"~/missing\"\n",
		"relative dir":      "[server]\nstart_dir = \"relative/dir\"\n",
	} {
		if _, err := write(c); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A key of [gui] or [server] this version does not read is a warning, not a stop.
func TestKisuiUnknownKeyIsWarning(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir, _ := config.UserDir(opts())
	os.MkdirAll(dir, 0o755)
	for _, c := range []string{"[[gui.machines]]\nssh = \"a\"\nhost = \"b\"\n", "[gui]\ncolor = 1\n", "[server]\nnew_key = 1\n"} {
		os.WriteFile(filepath.Join(dir, "config.toml"), []byte(c), 0o644)
		cfg, err := config.Load(root, opts())
		if err != nil || len(cfg.Warnings) != 1 {
			t.Errorf("%q: err = %v, warnings = %v", c, err, cfg.Warnings)
		}
	}
}
