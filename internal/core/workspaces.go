package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Workspaces is the cores of `shj serve`: one per workspace root, on one
// Shared (docs/design/gui-protocol.md, 3 章「ワークスペース」). A core keeps
// its files when another workspace takes the focus, and lives until the
// workspace is closed. The TUI does not use it: it has its one core.
type Workspaces struct {
	shared *Shared
	// front makes the front end of a new core. nil makes a headless.
	front func() FrontEnd

	mu       sync.Mutex
	cores    map[string]*Workspace // root -> its core
	onChange func()                // SetOnChange
}

// Workspace is the core of one workspace root. Whoever touches the core does
// it inside Do, Update or Apply, which run one caller at a time. Update and
// Apply also carry out the Effects the core asks for, as the TUI's event loop
// does: the work runs on goroutines of its own and its Events come back
// through Update.
type Workspace struct {
	Root     string // the workspace root, an absolute path
	mu       sync.Mutex
	core     *Core
	closed   bool          // Close stopped the core; late Events are dropped
	done     chan struct{} // closed by Close: the goroutines that wait on the core's channels stop
	onChange func()        // called after an Event that came back changed the core
}

// Do runs fn with the core, one caller at a time.
func (w *Workspace) Do(fn func(c *Core)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn(w.core)
}

// Update hands msg (an Input or an Event) to the core and carries out the
// Effects it asks for. It reports whether the core took it (see Apply).
func (w *Workspace) Update(msg any) bool {
	return w.Apply(func(c *Core) []Effect { return c.Update(msg) })
}

// Apply runs fn with the core, one caller at a time, and carries out the
// Effects fn returns. fn calls Core.Update or Core.Apply. It reports whether fn
// ran: not once the workspace is closed.
func (w *Workspace) Apply(fn func(c *Core) []Effect) bool {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return false
	}
	effs := fn(w.core)
	w.mu.Unlock()
	w.run(effs)
	return true
}

// deliver hands an Event that came back from an Effect to the core.
func (w *Workspace) deliver(ev Event) {
	if ev == nil {
		return
	}
	if w.Update(ev) && w.onChange != nil {
		w.onChange()
	}
}

// wait delivers the next Event from, unless the workspace is closed first: the
// core does not close its channels, and a goroutine left waiting would keep the
// core of a closed workspace.
func (w *Workspace) wait(from <-chan Event) {
	select {
	case ev := <-from:
		w.deliver(ev)
	case <-w.done:
	}
}

// run carries out effs as internal/tui's effectCmd does. A core of `shj serve`
// never quits: Quit is dropped (closing the GUI leaves the server running).
func (w *Workspace) run(effs []Effect) {
	for _, e := range effs {
		switch e := e.(type) {
		case StartTimer:
			fire := e.Fire
			time.AfterFunc(e.After, func() { w.deliver(fire) })
		case RunLSP:
			run := e.Run
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), LspTimeout)
				defer cancel()
				w.deliver(run(ctx))
			}()
		case WaitLSP:
			from := e.From
			go func() {
				select {
				case ev := <-from:
					w.deliver(LspEventMsg(ev))
				case <-w.done:
				}
			}()
		case RunSearch:
			go func() { w.deliver(e.Run()) }()
		case RunProbe:
			go func() { w.deliver(e.Run()) }()
		case Run:
			go func() { w.deliver(e.Run()) }()
		case WaitTerm:
			go w.wait(e.From)
		case WaitAgent:
			go w.wait(e.From)
		case RunAgent:
			go e.Run()
		case ReplyCtl:
			e.To <- e.Resp
		case ReplyIDE:
			e.To <- e.Reply
		}
	}
}

// NewWorkspaces makes the cores' registry on sh. front makes the front end of
// each new core; nil gives each a headless.
func NewWorkspaces(sh *Shared, front func() FrontEnd) *Workspaces {
	return &Workspaces{shared: sh, front: front, cores: map[string]*Workspace{}}
}

// SetOnChange sets what is called when a core changed on its own: an Event
// came back from the work it asked for (a language server's reply, a timer).
// The server rebuilds the screen then. It applies to the cores made after.
func (ws *Workspaces) SetOnChange(fn func()) {
	ws.mu.Lock()
	ws.onChange = fn
	ws.mu.Unlock()
}

func cleanRoot(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return filepath.Clean(root)
}

// Open is the workspace at root, made with a new core if it has none. A new
// core starts as the TUI's does: it waits on its language servers and watches
// its files on disk.
func (ws *Workspaces) Open(root string) *Workspace {
	root = cleanRoot(root)
	if w := ws.Lookup(root); w != nil {
		return w
	}
	// Making a core reads the config and runs git, so it is done without the
	// lock: a slow workspace does not hold up the others.
	var c *Core
	if ws.front != nil {
		c = NewIn(ws.shared, ws.front(), root, "")
	} else {
		c = newHeadlessCore(ws.shared, root)
	}
	ws.mu.Lock()
	w := &Workspace{Root: root, core: c, onChange: ws.onChange, done: make(chan struct{})}
	if got := ws.cores[root]; got != nil {
		// someone made it meanwhile; theirs stays
		ws.mu.Unlock()
		w.core.Close()
		return got
	}
	ws.cores[root] = w
	ws.mu.Unlock()
	w.run([]Effect{WaitLSP{c.LspEvents()}, WatchTimer()})
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
	slices.SortFunc(out, func(a, b *Workspace) int { return cmp.Compare(a.Root, b.Root) })
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
		w.Do(func(c *Core) {
			c.Close()
			w.closed = true
			close(w.done)
		})
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
	cur := a.Cur()
	for i, v := range slices.Backward(a.Tabs) {
		if !v.dirty() {
			a.dropTab(i)
		}
	}
	curKept := slices.Contains(a.Tabs, cur)
	otherKept := a.Other != nil && slices.Contains(a.Tabs, a.Other)
	if a.Split && (!curKept || !otherKept) {
		// one pane lost its file: the file left fills the area
		if !curKept && otherKept {
			cur, curKept = a.Other, true
		}
		a.Split, a.Pane, a.Other, a.otherPrev = false, 0, nil, nil
	}
	if !slices.Contains(a.Tabs, a.PrevTab) {
		a.PrevTab = nil
	}
	if curKept {
		a.Active = a.tabIndex(cur)
	} else {
		a.Active = min(a.Active, len(a.Tabs)-1)
	}
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
