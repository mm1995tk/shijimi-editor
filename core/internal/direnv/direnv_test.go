package direnv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestApply(t *testing.T) {
	base := []string{"A=1", "B=2", "PATH=/usr/bin"}
	got, err := Apply(base, []byte(`{"B":null,"PATH":"/nix/bin:/usr/bin","C":"3"}`))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{"A=1", "C=3", "PATH=/nix/bin:/usr/bin"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, _ := Apply(base, nil); !slices.Equal(got, base) {
		t.Fatalf("empty output changed the environment: %v", got)
	}
	if _, err := Apply(base, []byte("nope")); err == nil {
		t.Fatal("bad JSON was accepted")
	}
}

func TestLookPathUsesEnvPATH(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakels")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := LookPath([]string{"PATH=" + dir}, "fakels")
	if err != nil || got != bin {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := LookPath([]string{"PATH=" + t.TempDir()}, "fakels"); err == nil {
		t.Fatal("found a command that is not on the PATH")
	}
}

func TestWanted(t *testing.T) {
	dir := t.TempDir()
	if Wanted(dir) {
		t.Fatal("a folder with no .envrc is wanted")
	}
}

// fakeDirenv puts a direnv script in PATH that behaves as script says.
func fakeDirenv(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "direnv"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".envrc"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fakeDirenv(t, `echo '{"SHJ_TEST":"yes"}'`)
	if !Wanted(root) {
		t.Fatal("not wanted")
	}
	env, err := Load(context.Background(), root, nil)
	if err != nil || Getenv(env, "SHJ_TEST") != "yes" {
		t.Fatalf("env %v, err %v", Getenv(env, "SHJ_TEST"), err)
	}

	fakeDirenv(t, "echo \"direnv: error $PWD/.envrc is blocked. Run direnv allow to approve\" >&2")
	if _, err := Load(context.Background(), root, nil); !errors.Is(err, ErrBlocked) {
		t.Fatalf("got %v, want ErrBlocked", err)
	}

	fakeDirenv(t, `echo "boom" >&2; exit 1`)
	if _, err := Load(context.Background(), root, nil); err == nil || errors.Is(err, ErrBlocked) || err.Error() != "boom" {
		t.Fatalf("got %v", err)
	}
}
