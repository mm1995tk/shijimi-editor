package cli

import (
	"context"
	"encoding/json"
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

// With a real core behind `shj serve`, typing one character sends only the
// line it changed and the status line (and where the cursor is).
func TestServeTypingSendsChangedLine(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
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
	srv.SetCores(serveCores{cores})
	srv.SetScreens(serveCores{cores})
	done := make(chan struct{})
	go func() { srv.Serve(context.Background()); close(done) }()
	t.Cleanup(func() { srv.Close(); <-done })

	w := cores.Open(root)
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

	nc, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	nt := &notes{}
	c := rpc.NewConn(nc, rpc.JSON{}, nt.handle)
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
	if err := c.Call(ctx, "workspace.focus", map[string]any{"id": l.Workspaces[0].ID}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "view.subscribe", map[string]any{"regions": []string{"files", "status", "cmdline", "overlay"}}, nil); err != nil {
		t.Fatal(err)
	}
	first := nt.wait(t, 1)[0]
	var rev int64
	json.Unmarshal(first["rev"], &rev)
	c.Notify("screen.ack", map[string]any{"rev": rev})

	w.Do(func(c *core.Core) { key(c, "Y") })
	srv.ScreenChanged()
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
	var files struct {
		Panes []struct {
			ID    string           `json:"id"`
			Title *json.RawMessage `json:"title"`
			Lines struct {
				Rows  map[string]screen.Line `json:"rows"`
				Count int                    `json:"count"`
			} `json:"lines"`
		} `json:"panes"`
	}
	if err := json.Unmarshal(u["files"], &files); err != nil || len(files.Panes) != 1 {
		t.Fatalf("files: %s", u["files"])
	}
	p := files.Panes[0]
	if p.ID != path || p.Title != nil || p.Lines.Count != 4 || len(p.Lines.Rows) != 1 {
		t.Fatalf("pane: %s", u["files"])
	}
	line, ok := p.Lines.Rows["1"]
	text := ""
	for _, s := range line.Spans {
		text += s.Text
	}
	if !ok || line.No != 1 || text != "XYtwo" {
		t.Errorf("changed line: %+v", p.Lines.Rows)
	}
}
