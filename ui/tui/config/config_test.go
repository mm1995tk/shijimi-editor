package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/config"
)

var nightly = config.Options{DirSuffix: "-nightly", Sections: []config.Section{Section(true)}}

func opts(experimental bool) config.Options {
	return config.Options{Sections: []config.Section{Section(experimental)}}
}

// load is config.Load for a configuration that must be valid.
func load(root string, o config.Options) *Settings {
	c, err := config.Load(root, o)
	if err != nil {
		panic(err)
	}
	return Of(c)
}

func TestExperimentalMultiplexer(t *testing.T) {
	o := opts(false)
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	if load(root, o).Experimental.Multiplexer {
		t.Fatal("herdr must default to false on the stable channel")
	}
	o = nightly
	if !load(root, o).Experimental.Multiplexer {
		t.Fatal("herdr must default to true on the nightly channel")
	}
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor-nightly"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor-nightly", "config.toml"), []byte("[experimental]\nherdr = false\n"), 0o644)
	if load(root, o).Experimental.Multiplexer {
		t.Fatal("herdr = false must win over the nightly default")
	}
	o = opts(false)
	os.MkdirAll(filepath.Join(cfg, "shijimi-editor"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shijimi-editor", "config.toml"), []byte("[experimental]\nherdr = true\n"), 0o644)
	if !load(root, o).Experimental.Multiplexer {
		t.Fatal("herdr = true must win over the stable default")
	}
}

func TestMultiplexerKeys(t *testing.T) {
	root := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(cfg, "shijimi-editor")
	os.MkdirAll(dir, 0o755)
	write := func(s string) *Settings {
		os.WriteFile(filepath.Join(dir, "config.toml"), []byte(s), 0o644)
		return load(root, opts(false))
	}
	if c := write(""); c.Terminal.Multiplexer != "auto" {
		t.Fatalf("default: %q", c.Terminal.Multiplexer)
	}
	if c := write("[experimental]\nmultiplexer = true\n"); !c.Experimental.Multiplexer {
		t.Fatal("multiplexer = true must enable it")
	}
	// The new key wins over the old one, whatever the order.
	if c := write("[experimental]\nmultiplexer = false\nherdr = true\n"); c.Experimental.Multiplexer {
		t.Fatal("multiplexer = false must win over herdr = true")
	}
	if c := write("[terminal]\nmultiplexer = \"None\"\n"); c.Terminal.Multiplexer != "none" {
		t.Fatalf("none: %q", c.Terminal.Multiplexer)
	}
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[terminal]\nmultiplexer = \"screen\"\n"), 0o644)
	if _, err := config.Load(root, opts(false)); err == nil {
		t.Fatal("an unknown multiplexer must be an error")
	}
}
