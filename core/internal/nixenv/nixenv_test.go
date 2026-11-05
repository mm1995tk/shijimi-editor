package nixenv

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestApply(t *testing.T) {
	out := `{"variables":{
	  "PATH":{"type":"exported","value":"/nix/gopls/bin:/usr/bin"},
	  "HOME":{"type":"exported","value":"/homeless-shelter"},
	  "buildInputs":{"type":"var","value":"/nix/x"},
	  "arr":{"type":"array","value":["a"]},
	  "GOFLAGS":{"type":"exported","value":"-mod=mod"}}}`
	got, err := Apply([]string{"HOME=/home/u", "PATH=/usr/bin", "A=1"}, []byte(out))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{"A=1", "GOFLAGS=-mod=mod", "HOME=/home/u", "PATH=/nix/gopls/bin:/usr/bin"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := Apply(nil, []byte("nope")); err == nil {
		t.Fatal("bad JSON was accepted")
	}
}

func TestAnswers(t *testing.T) {
	dir := t.TempDir()
	a := NewAnswers(filepath.Join(dir, "state"))
	if _, ok := a.Get("/p"); ok {
		t.Fatal("answered before asking")
	}
	if err := a.Set("/p", true); err != nil {
		t.Fatal(err)
	}
	if err := a.Set("/q", false); err != nil {
		t.Fatal(err)
	}
	b := NewAnswers(filepath.Join(dir, "state"))
	if al, ok := b.Get("/p"); !ok || !al {
		t.Fatalf("/p = %v %v", al, ok)
	}
	if al, ok := b.Get("/q"); !ok || al {
		t.Fatalf("/q = %v %v", al, ok)
	}
	if err := NewAnswers("").Set("/p", true); err != nil {
		t.Fatal(err)
	}
}

func TestWanted(t *testing.T) {
	root := t.TempDir()
	if Wanted(root) {
		t.Fatal("no flake.nix")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "nix"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if Wanted(root) {
		t.Fatal("no flake.nix")
	}
	if err := os.WriteFile(filepath.Join(root, "flake.nix"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !Wanted(root) {
		t.Fatal("flake.nix and nix")
	}
}
