package ctl

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buildinfo"
)

func TestSocketPathIsSeparatePerChannel(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	buildinfo.SetForTest(t, "")
	stable, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	buildinfo.SetForTest(t, buildinfo.Nightly)
	nightly, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(stable) == filepath.Dir(nightly) {
		t.Fatalf("stable and nightly share the directory %s", filepath.Dir(stable))
	}
	if base := filepath.Base(filepath.Dir(nightly)); !strings.HasPrefix(base, "shijimi-nightly-") {
		t.Fatalf("nightly dir = %s", base)
	}
	if base := filepath.Base(filepath.Dir(stable)); !strings.HasPrefix(base, "shijimi-") || strings.Contains(base, "nightly") {
		t.Fatalf("stable dir = %s", base)
	}
}
