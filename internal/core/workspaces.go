package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
)

// Workspaces is the cores of `shj serve`: one per workspace root, on one
// Shared (docs/design/gui-protocol.md, 3 章「ワークスペース」). A core keeps
// its files when another workspace takes the focus, and lives until the
// workspace is closed. The TUI does not use it: it has its one core.
type Workspaces struct {
	shared *Shared
	// front makes the front end of a new core. nil makes a Headless.
	front func() FrontEnd

	mu    sync.Mutex
	cores map[string]*Workspace // root -> its core
}

// Workspace is the core of one workspace root. Nothing runs a core's events on
// its own goroutine yet, so whoever touches the core does it inside Do, which
// runs one caller at a time.
type Workspace struct {
	Root string
	mu   sync.Mutex
	core *Core
}

// Do runs fn with the core, one caller at a time.
func (w *Workspace) Do(fn func(c *Core)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn(w.core)
}

// NewWorkspaces makes the cores' registry on sh. front makes the front end of
// each new core; nil gives each a Headless.
func NewWorkspaces(sh *Shared, front func() FrontEnd) *Workspaces {
	return &Workspaces{shared: sh, front: front, cores: map[string]*Workspace{}}
}

func cleanRoot(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return filepath.Clean(root)
}

// Open is the workspace at root, made with a new core if it has none.
func (ws *Workspaces) Open(root string) *Workspace {
	root = cleanRoot(root)
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if w := ws.cores[root]; w != nil {
		return w
	}
	var fe FrontEnd
	if ws.front != nil {
		fe = ws.front()
	} else {
		fe = NewHeadless()
	}
	w := &Workspace{Root: root, core: NewIn(ws.shared, fe, root, "")}
	ws.cores[root] = w
	return w
}

// Lookup is the workspace at root, or nil if it has no core.
func (ws *Workspaces) Lookup(root string) *Workspace {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.cores[cleanRoot(root)]
}

// List is the workspaces that have a core, by root.
func (ws *Workspaces) List() []*Workspace {
	ws.mu.Lock()
	out := make([]*Workspace, 0, len(ws.cores))
	for _, w := range ws.cores {
		out = append(out, w)
	}
	ws.mu.Unlock()
	slices.SortFunc(out, func(a, b *Workspace) int {
		switch {
		case a.Root < b.Root:
			return -1
		case a.Root > b.Root:
			return 1
		}
		return 0
	})
	return out
}

// Close stops the core at root (its language servers, its agent) and forgets
// it. Unsaved edits are dropped: the caller closes a workspace only when it
// has no open files.
func (ws *Workspaces) Close(root string) {
	root = cleanRoot(root)
	ws.mu.Lock()
	w := ws.cores[root]
	delete(ws.cores, root)
	ws.mu.Unlock()
	if w != nil {
		w.Do(func(c *Core) { c.Close() })
	}
}

// CloseAll stops every core. The Shared stays open.
func (ws *Workspaces) CloseAll() {
	for _, w := range ws.List() {
		ws.Close(w.Root)
	}
}

// WriteBackups writes the unsaved edits of every core with write (see
// Core.WriteBackups). It returns the files written and the errors met.
func (ws *Workspaces) WriteBackups(write func(path string, data []byte) (string, error)) ([]string, error) {
	var written []string
	var errs []error
	for _, w := range ws.List() {
		w.Do(func(c *Core) {
			out, err := c.WriteBackups(write)
			written = append(written, out...)
			if err != nil {
				errs = append(errs, err)
			}
		})
	}
	return written, errors.Join(errs...)
}

// OpenPaths is the paths of the files the core has open, in their order. A
// buffer that has never been saved has no path and is left out.
func (a *Core) OpenPaths() []string {
	var out []string
	for _, t := range a.Tabs {
		if p := t.Path(); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// HasOpenFiles reports whether the core has any file open, named or not.
func (a *Core) HasOpenFiles() bool { return len(a.Tabs) > 0 }

// CloseFiles closes the files that have no unsaved edits and keeps the others
// open, so that closing never loses an edit. It returns how many it kept.
func (a *Core) CloseFiles() int {
	for i, v := range slices.Backward(a.Tabs) {
		if !v.dirty() {
			a.dropTab(i)
		}
	}
	if a.Split && (a.Other == nil || !slices.Contains(a.Tabs, a.Other)) {
		a.Split, a.Pane, a.Other, a.otherPrev = false, 0, nil, nil
	}
	if !slices.Contains(a.Tabs, a.PrevTab) {
		a.PrevTab = nil
	}
	a.Active = min(a.Active, len(a.Tabs)-1)
	if len(a.Tabs) == 0 {
		a.Active = 0
		a.Comp = nil
		a.Mode = ModeNormal
		if a.Focus == FocusEditor {
			a.Focus = FocusExplorer
		}
	}
	return len(a.Tabs)
}

// WriteBackups writes each file with unsaved edits through write, which puts
// it beside the file (server.WriteBackup: <name>.bk, never over a file). The
// files themselves are not touched. A buffer with no path has nowhere to go
// and is reported as an error.
func (a *Core) WriteBackups(write func(path string, data []byte) (string, error)) ([]string, error) {
	var written []string
	var errs []error
	for _, t := range a.Tabs {
		if !t.dirty() {
			continue
		}
		if t.Buf.Path == "" {
			errs = append(errs, fmt.Errorf("%s: an unsaved buffer with no file name in %s", t.Buf.Name(), a.Root))
			continue
		}
		name, err := write(t.Buf.Path, []byte(t.Buf.Encoded()))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.Buf.Path, err))
			continue
		}
		written = append(written, name)
	}
	return written, errors.Join(errs...)
}
