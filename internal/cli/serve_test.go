package cli

import (
	"context"
	"encoding/json"
	"maps"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/acp/acptest"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/rpc"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
	"github.com/mm1995tk/shijimi-editor/internal/server"
)

func TestMain(m *testing.M) {
	// A core may start the agent as a child of the test binary.
	acptest.Main()
	// Keep the developer's own config out of the cores.
	dir, err := os.MkdirTemp("", "shijimi-cli-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type notes struct {
	mu  sync.Mutex
	got []json.RawMessage
}

func (n *notes) handle(_ context.Context, m *rpc.Message) (any, error) {
	if m.IsNotification() && m.Method == "screen.update" {
		n.mu.Lock()
		n.got = append(n.got, m.Params)
		n.mu.Unlock()
	}
	return nil, nil
}

func (n *notes) wait(t *testing.T, count int) []map[string]json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		n.mu.Lock()
		got := append([]json.RawMessage(nil), n.got...)
		n.mu.Unlock()
		if len(got) >= count {
			var out []map[string]json.RawMessage
			for _, raw := range got {
				var m map[string]json.RawMessage
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Fatal(err)
				}
				out = append(out, m)
			}
			return out
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %d screen.update", count)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// serveEnv is `shj serve` with real cores behind it and a workspace at root.
type serveEnv struct {
	srv   *server.Server
	cores *core.Workspaces
	root  string
	sock  string
}

func newServeEnv(t *testing.T) *serveEnv {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sockDir, err := os.MkdirTemp("", "shjcli")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	sock := filepath.Join(sockDir, "serve.rpc")
	srv, err := server.Listen(server.Options{SocketPath: sock, CwdInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	shared := core.NewShared()
	cores := core.NewWorkspaces(shared, nil)
	t.Cleanup(func() { cores.CloseAll(); shared.Close() })
	cores.SetOnChange(srv.ScreenChanged)
	srv.SetCores(serveCores{cores})
	srv.SetScreens(serveCores{cores})
	srv.SetInputs(serveCores{cores})
	done := make(chan struct{})
	go func() { srv.Serve(context.Background()); close(done) }()
	t.Cleanup(func() { srv.Close(); <-done })
	return &serveEnv{srv: srv, cores: cores, root: root, sock: sock}
}

// connect is a GUI that focused the workspace (whose core must be open) and
// subscribed to the GUI's regions. It returns the workspace's id and the
// first screen.update, acknowledged.
func (e *serveEnv) connect(t *testing.T) (c *rpc.Conn, nt *notes, id string, first map[string]json.RawMessage) {
	t.Helper()
	nc, err := net.Dial("unix", e.sock)
	if err != nil {
		t.Fatal(err)
	}
	nt = &notes{}
	c = rpc.NewConn(nc, rpc.JSON{}, nt.handle)
	t.Cleanup(func() { c.Close() })
	ctx := context.Background()
	hello := map[string]any{
		"protocol": map[string]int{"major": 1},
		"client":   map[string]string{"name": "test", "version": "0", "kind": "web"},
	}
	if err := c.Call(ctx, "session.hello", hello, nil); err != nil {
		t.Fatal(err)
	}
	var l struct {
		Workspaces []struct {
			ID string `json:"id"`
		} `json:"workspaces"`
	}
	deadline := time.Now().Add(10 * time.Second)
	for len(l.Workspaces) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no workspace")
		}
		time.Sleep(10 * time.Millisecond)
		c.Call(ctx, "workspace.list", map[string]any{}, &l)
	}
	id = l.Workspaces[0].ID
	if err := c.Call(ctx, "workspace.focus", map[string]any{"id": id}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "view.subscribe", map[string]any{"regions": []string{"files", "status", "cmdline", "overlay"}}, nil); err != nil {
		t.Fatal(err)
	}
	first = nt.wait(t, 1)[0]
	ackUpdate(t, c, first)
	return c, nt, id, first
}

func ackUpdate(t *testing.T, c *rpc.Conn, u map[string]json.RawMessage) {
	t.Helper()
	var rev int64
	json.Unmarshal(u["rev"], &rev)
	if err := c.Notify("screen.ack", map[string]any{"rev": rev}); err != nil {
		t.Fatal(err)
	}
}

// filesUpdate is the files region of a screen.update.
type filesUpdate struct {
	Panes []struct {
		ID    string           `json:"id"`
		Title *json.RawMessage `json:"title"`
		Lines struct {
			Rows  map[string]screen.Line `json:"rows"`
			Count int                    `json:"count"`
		} `json:"lines"`
	} `json:"panes"`
}

func lineText(l screen.Line) string {
	text := ""
	for _, s := range l.Spans {
		text += s.Text
	}
	return text
}

// With a real core behind `shj serve`, typing one character sends only the
// line it changed and the status line (and where the cursor is).
func TestServeTypingSendsChangedLine(t *testing.T) {
	e := newServeEnv(t)
	path := filepath.Join(e.root, "a.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := e.cores.Open(e.root)
	key := func(c *core.Core, s string) {
		if s == "esc" {
			c.Update(core.Key{Code: core.KeyEscape})
			return
		}
		c.Update(core.Key{Code: []rune(s)[0], Text: s})
	}
	w.Do(func(c *core.Core) {
		if !c.OpenFile(path) {
			t.Fatalf("open: %s", c.Msg)
		}
		c.Focus = core.FocusEditor
		// go down a line and start typing, so the file is unsaved already
		key(c, "j")
		key(c, "i")
		key(c, "X")
	})
	_, nt, _, _ := e.connect(t)

	w.Do(func(c *core.Core) { key(c, "Y") })
	e.srv.ScreenChanged()
	u := nt.wait(t, 2)[1]
	for _, k := range []string{"focus", "cmdline", "overlay"} {
		if _, ok := u[k]; ok {
			t.Errorf("update has %s: %s", k, u[k])
		}
	}
	var st screen.Status
	if err := json.Unmarshal(u["status"], &st); err != nil || st.Position != "2:3" {
		t.Errorf("status: %s", u["status"])
	}
	var files filesUpdate
	if err := json.Unmarshal(u["files"], &files); err != nil || len(files.Panes) != 1 {
		t.Fatalf("files: %s", u["files"])
	}
	p := files.Panes[0]
	if p.ID != path || p.Title != nil || p.Lines.Count != 4 || len(p.Lines.Rows) != 1 {
		t.Fatalf("pane: %s", u["files"])
	}
	line, ok := p.Lines.Rows["1"]
	if !ok || line.No != 1 || lineText(line) != "XYtwo" {
		t.Errorf("changed line: %+v", p.Lines.Rows)
	}
}

// The GUI opens a file of the tree with file.open, and the characters it
// types with input.key reach the screen: the screen.update that has an input
// says so with ackSeq.
func TestServeInputKeyAndFileOpen(t *testing.T) {
	e := newServeEnv(t)
	path := filepath.Join(e.root, "a.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A workspace stays while its core has a file open (no shell is in it here).
	other := filepath.Join(e.root, "b.txt")
	if err := os.WriteFile(other, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.cores.Open(e.root).Do(func(c *core.Core) { c.OpenFile(other) })
	c, nt, id, _ := e.connect(t)
	ctx := context.Background()
	seen := 1
	// call sends an input and acknowledges the screen.updates after it until one
	// has it (ackSeq). It returns the panes' rows and the status line they sent.
	call := func(method string, seq uint64, params map[string]any) (rows map[string]screen.Line, ids []string, st *screen.Status) {
		t.Helper()
		params["seq"] = seq
		if err := c.Call(ctx, method, params, nil); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		rows = map[string]screen.Line{}
		for {
			us := nt.wait(t, seen+1)
			for _, u := range us[seen:] {
				seen++
				ackUpdate(t, c, u)
				var files filesUpdate
				if json.Unmarshal(u["files"], &files) == nil {
					ids = nil
					for _, p := range files.Panes {
						ids = append(ids, p.ID)
						maps.Copy(rows, p.Lines.Rows)
					}
				}
				if raw, ok := u["status"]; ok {
					st = &screen.Status{}
					json.Unmarshal(raw, st)
				}
				var ackSeq uint64
				json.Unmarshal(u["ackSeq"], &ackSeq)
				if ackSeq >= seq {
					return rows, ids, st
				}
			}
		}
	}

	_, ids, _ := call("file.open", 1, map[string]any{"workspaceId": id, "path": "a.txt", "line": 1})
	if len(ids) != 1 || ids[0] != path {
		t.Fatalf("file.open: panes %v", ids)
	}
	key := func(seq uint64, k, text string) (map[string]screen.Line, *screen.Status) {
		rows, _, st := call("input.key", seq, map[string]any{"region": "files", "key": k, "mods": []string{}, "text": text, "phase": "press"})
		return rows, st
	}
	key(2, "i", "i")
	rows, _ := key(3, "Z", "Z")
	if line, ok := rows["1"]; !ok || lineText(line) != "Ztwo" {
		t.Errorf("typed line: %+v", rows)
	}
	// Escape (the cursor stays after the Z), then a key named as the GUI names it moves it up.
	key(4, "Escape", "")
	if _, st := key(5, "ArrowUp", ""); st == nil || st.Position != "1:2" {
		t.Errorf("status after ArrowUp: %+v", st)
	}
}
