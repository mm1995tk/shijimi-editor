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

// Workspaces is the cores of `shj serve`: one per workspace, on one Shared
// (docs/design/gui-protocol.md, 3 章「ワークスペース」). A workspace is named by
// a key (the server's workspace id), not by its folder: two workspaces may
// point at the same folder and each has a core of its own. A core keeps its
// files when another workspace takes the focus, and lives until the workspace
// is closed or switches to another folder. The TUI does not use it: it has its
// one core.
type Workspaces struct {
	shared *Shared
	// front makes the front end of a new core. nil makes a headless.
	front func() FrontEnd

	mu       sync.Mutex
	cores    map[string]*Workspace // key -> its core
	onChange func()                // SetOnChange
}

// Workspace is the core of one workspace root. Whoever touches the core does
// it inside Do, Update or Apply, which run one caller at a time. Update and
// Apply also carry out the Effects the core asks for, as the TUI's event loop
// does: the work runs on goroutines of its own and its Events come back
// through Update.
type Workspace struct {
	Key      string // the workspace's key
	Root     string // the workspace's folder, an absolute path
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

// Open is the workspace key, made with a new core on the folder root if it has
// none. A new core starts as the TUI's does: it waits on its language servers
// and watches its files on disk. A mistake in the workspace's configuration
// files (a *config.Error) is returned and no core is made; the next Open reads
// the files again.
func (ws *Workspaces) Open(key, root string) (*Workspace, error) {
	root = cleanRoot(root)
	if w := ws.Lookup(key); w != nil {
		return w, nil
	}
	// Making a core reads the config and runs git, so it is done without the
	// lock: a slow workspace does not hold up the others.
	var c *Core
	var err error
	if ws.front != nil {
		c, err = NewIn(ws.shared, ws.front(), root, "")
	} else {
		c, err = newHeadlessCore(ws.shared, root)
	}
	if err != nil {
		return nil, err
	}
	ws.mu.Lock()
	w := &Workspace{Key: key, Root: root, core: c, onChange: ws.onChange, done: make(chan struct{})}
	if got := ws.cores[key]; got != nil {
		// someone made it meanwhile; theirs stays
		ws.mu.Unlock()
		w.core.Close()
		return got, nil
	}
	ws.cores[key] = w
	ws.mu.Unlock()
	w.run([]Effect{WaitLSP{c.LspEvents()}, WatchTimer()})
	return w, nil
}

// Lookup is the workspace key, or nil if it has no core.
func (ws *Workspaces) Lookup(key string) *Workspace {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.cores[key]
}

// List is the workspaces that have a core, by key.
func (ws *Workspaces) List() []*Workspace {
	ws.mu.Lock()
	out := make([]*Workspace, 0, len(ws.cores))
	for _, w := range ws.cores {
		out = append(out, w)
	}
	ws.mu.Unlock()
	slices.SortFunc(out, func(a, b *Workspace) int { return cmp.Compare(a.Key, b.Key) })
	return out
}

// Close stops the core of key (its language servers, its agent) and forgets
// it. Unsaved edits are dropped: the caller saves them first, or means to
// throw them away.
func (ws *Workspaces) Close(key string) {
	ws.mu.Lock()
	w := ws.cores[key]
	delete(ws.cores, key)
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
		ws.Close(w.Key)
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

// SavesOnSwitch reports whether the files are saved when the workspace
// switches to another folder: they are while auto-save is on, and otherwise as
// [editor.auto-save] on-switch says (config.Load insists it is written).
func (a *Core) SavesOnSwitch() bool {
	return a.Cfg.Editor.AutoSave || a.Cfg.Editor.OnSwitch != "discard"
}

// SaveFiles saves every file with unsaved edits now, and returns the paths it
// could not save: the file changed on disk meanwhile (it is never overwritten
// silently), or writing failed. The files stay open. It only writes the files: it is used just before the core is closed,
// so it tells no language server and rereads no git marks.
func (a *Core) SaveFiles() (failed []string) {
	for _, t := range a.Tabs {
		if !t.dirty() || t.Buf.Path == "" {
			continue
		}
		if t.Buf.DiskChanged() || t.Buf.Save() != nil {
			failed = append(failed, t.Buf.Path)
		}
	}
	return failed
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
