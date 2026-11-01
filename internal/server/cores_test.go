package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// fakeCores is a Cores whose files the test sets.
type fakeCores struct {
	mu      sync.Mutex
	files   map[string][]string // root -> open files
	opened  []string
	closed  []string
	cleared []string
}

func (f *fakeCores) Open(root string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, root)
}

func (f *fakeCores) Files(root string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.files[root])
}

func (f *fakeCores) Kept() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for r, l := range f.files {
		if len(l) > 0 {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeCores) CloseFiles(root string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleared = append(f.cleared, root)
	delete(f.files, root)
}

func (f *fakeCores) Close(root string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = append(f.closed, root)
}

func (f *fakeCores) seen(l *[]string, root string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(*l, root)
}

// A workspace with files open in its core stays without a shell in it, lists
// the files relative to its root, gets its core on focus, and goes (with its
// core) once its files are closed.
func TestWorkspaceKeptByOpenFiles(t *testing.T) {
	srv, sock := startFast(t)
	root := normalizePath(t.TempDir())
	fc := &fakeCores{files: map[string][]string{root: {filepath.Join(root, "pkg", "a.go"), "/elsewhere/b.go"}}}
	srv.SetCores(fc)
	c, nt := termClient(t, sock)
	ctx := context.Background()

	var l wsList
	waitFor(t, "workspace with files", func() bool {
		l = wsList{}
		c.Call(ctx, "workspace.list", map[string]any{}, &l)
		return len(l.Workspaces) == 1
	})
	x := l.Workspaces[0]
	if x.Root != root || x.Kind != "cwd" || len(x.Terms) != 0 || !slices.Equal(x.Files, []string{"pkg/a.go", "/elsewhere/b.go"}) {
		t.Fatalf("workspace: %+v", x)
	}

	if err := c.Call(ctx, "workspace.focus", map[string]any{"id": x.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if !fc.seen(&fc.opened, root) {
		t.Error("focus did not open the workspace's core")
	}

	if err := c.Call(ctx, "workspace.close_files", map[string]any{"id": x.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if !fc.seen(&fc.cleared, root) {
		t.Error("close_files did not reach the core")
	}
	waitFor(t, "removed", func() bool {
		for _, raw := range nt.all("workspace.changed") {
			var m struct{ Removed []string }
			if json.Unmarshal(*raw, &m); slices.Contains(m.Removed, x.ID) {
				return true
			}
		}
		return false
	})
	waitFor(t, "core closed", func() bool { return fc.seen(&fc.closed, root) })
}
