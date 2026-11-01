package core

import (
	"path/filepath"
	"testing"
)

func TestCopyPathActiveTab(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.copyPath(a.activePath(), false)
	if a.Clipboard != "a.txt" || a.MsgErr {
		t.Fatalf("relative: clipboard=%q msg=%q", a.Clipboard, a.Msg)
	}
	a.copyPath(a.activePath(), true)
	if want := filepath.Join(a.Root, "a.txt"); a.Clipboard != want {
		t.Fatalf("absolute: clipboard=%q want %q", a.Clipboard, want)
	}
	if a.Msg == "" {
		t.Fatal("copied path is not shown")
	}
}

func TestCopyPathNoFile(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.copyPath("", false)
	if !a.MsgErr {
		t.Fatalf("expected error message, got %q", a.Msg)
	}
}

func TestCopyPathExplorerKeys(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Explorer.Reveal(filepath.Join(a.Root, "a.txt"))
	a.Focus = FocusExplorer
	keys(a, "y")
	if a.Clipboard != "a.txt" {
		t.Fatalf("y -> %q", a.Clipboard)
	}
	keys(a, "Y")
	if want := filepath.Join(a.Root, "a.txt"); a.Clipboard != want {
		t.Fatalf("Y -> %q want %q", a.Clipboard, want)
	}
}
