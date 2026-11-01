package core

import (
	"path/filepath"
	"testing"
)

func TestCopyPathActiveTab(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.copyPath(a.activePath(), false)
	if a.clipboard != "a.txt" || a.msgErr {
		t.Fatalf("relative: clipboard=%q msg=%q", a.clipboard, a.msg)
	}
	a.copyPath(a.activePath(), true)
	if want := filepath.Join(a.root, "a.txt"); a.clipboard != want {
		t.Fatalf("absolute: clipboard=%q want %q", a.clipboard, want)
	}
	if a.msg == "" {
		t.Fatal("copied path is not shown")
	}
}

func TestCopyPathNoFile(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.copyPath("", false)
	if !a.msgErr {
		t.Fatalf("expected error message, got %q", a.msg)
	}
}

func TestCopyPathExplorerKeys(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.explorer.reveal(filepath.Join(a.root, "a.txt"))
	a.focus = FocusExplorer
	keys(a, "y")
	if a.clipboard != "a.txt" {
		t.Fatalf("y -> %q", a.clipboard)
	}
	keys(a, "Y")
	if want := filepath.Join(a.root, "a.txt"); a.clipboard != want {
		t.Fatalf("Y -> %q want %q", a.clipboard, want)
	}
}
