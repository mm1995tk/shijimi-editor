package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startFast(t *testing.T) (*Server, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "shjsrv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	srv, err := Listen(Options{SocketPath: filepath.Join(dir, "serve.rpc"), CwdInterval: 20 * time.Millisecond, TreeInterval: 20 * time.Millisecond, GitInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { srv.Serve(context.Background()); close(done) }()
	t.Cleanup(func() { srv.Close(); <-done })
	return srv, filepath.Join(dir, "serve.rpc")
}

// all は method の通知をすべて返す。
func (n *notes) all(method string) []*json.RawMessage {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []*json.RawMessage
	for _, m := range n.got {
		if m.Method == method {
			p := m.Params
			out = append(out, &p)
		}
	}
	return out
}

type wsList struct {
	Workspaces []wsInfo `json:"workspaces"`
	Focus      *string  `json:"focus"`
}

func TestWorkspaceFollowsShellCd(t *testing.T) {
	if _, err := shellCwd(os.Getpid()); err != nil {
		t.Skipf("cannot read cwd: %v", err)
	}
	_, sock := startFast(t)
	c, nt := termClient(t, sock)
	ctx := context.Background()

	home, _ := os.UserHomeDir()
	repo := normalizePath(t.TempDir())
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "pkg")
	os.Mkdir(sub, 0o755)

	// 目印の無い所（~）から始める。ワークスペースにならない。
	id := createTerm(t, c, map[string]any{"cwd": home})
	var l wsList
	if err := c.Call(ctx, "workspace.list", map[string]any{}, &l); err != nil {
		t.Fatal(err)
	}
	if len(l.Workspaces) != 0 || l.Focus != nil {
		t.Fatalf("home must not be a workspace: %+v", l)
	}

	// cd すると term.cwd と workspace.changed(added) が届く。
	c.Call(ctx, "term.input", map[string]any{"termId": id, "data": b64("cd '" + sub + "'\n")}, nil)
	waitFor(t, "term.cwd", func() bool { return len(nt.all("term.cwd")) > 0 })
	var cwdN struct {
		TermID      string  `json:"termId"`
		Cwd         string  `json:"cwd"`
		WorkspaceID *string `json:"workspaceId"`
	}
	json.Unmarshal(*nt.all("term.cwd")[0], &cwdN)
	if cwdN.TermID != id || cwdN.Cwd != normalizePath(sub) || cwdN.WorkspaceID == nil || *cwdN.WorkspaceID != workspaceID(repo) {
		t.Fatalf("term.cwd: %+v", cwdN)
	}
	waitFor(t, "workspace.changed", func() bool { return len(nt.all("workspace.changed")) > 0 })
	var ch struct {
		Added   []wsInfo `json:"added"`
		Removed []string `json:"removed"`
	}
	json.Unmarshal(*nt.all("workspace.changed")[0], &ch)
	if len(ch.Added) != 1 || ch.Added[0].Root != repo || ch.Added[0].Kind != "git" || len(ch.Added[0].Terms) != 1 || ch.Added[0].Terms[0] != id {
		t.Fatalf("workspace.changed: %+v", ch)
	}

	// 変化が無い間は通知が増えない。
	time.Sleep(150 * time.Millisecond)
	if n := len(nt.all("term.cwd")); n != 1 {
		t.Errorf("term.cwd sent %d times", n)
	}

	// term.list にも載る。2 つめのターミナルは同じワークスペースにまとまる。
	id2 := createTerm(t, c, map[string]any{"cwd": repo})
	if err := c.Call(ctx, "workspace.list", map[string]any{}, &l); err != nil {
		t.Fatal(err)
	}
	if len(l.Workspaces) != 1 || len(l.Workspaces[0].Terms) != 2 {
		t.Fatalf("list: %+v", l)
	}

	// フォーカス: rev が進む。最後に入力したターミナルが返る。
	var f struct {
		Rev    int64  `json:"rev"`
		TermID string `json:"termId"`
	}
	if err := c.Call(ctx, "workspace.focus", map[string]any{"id": l.Workspaces[0].ID}, &f); err != nil {
		t.Fatal(err)
	}
	if f.Rev != 1 || f.TermID != id {
		t.Errorf("focus: %+v", f)
	}
	c.Call(ctx, "term.input", map[string]any{"termId": id2, "data": b64("\n")}, nil)
	c.Call(ctx, "workspace.focus", map[string]any{"id": l.Workspaces[0].ID}, &f)
	if f.Rev != 2 || f.TermID != id2 {
		t.Errorf("focus again: %+v", f)
	}
	if err := c.Call(ctx, "workspace.close_files", map[string]any{"id": l.Workspaces[0].ID}, nil); err != nil {
		t.Error(err)
	}
	if err := c.Call(ctx, "workspace.focus", map[string]any{"id": "nope"}, nil); err == nil {
		t.Error("unknown workspace must fail")
	}

	// フォーカスしているワークスペースのルートが term.create の既定の cwd になる。
	var cr struct {
		TermID string
		Cwd    string
	}
	c.Call(ctx, "term.create", map[string]any{"cols": 80, "rows": 24, "shell": "/bin/sh"}, &cr)
	if cr.Cwd != repo {
		t.Errorf("default cwd %q, want %q", cr.Cwd, repo)
	}

	// 全部のターミナルを閉じるとワークスペースが消える。
	for _, x := range []string{id, id2, cr.TermID} {
		c.Call(ctx, "term.close", map[string]any{"termId": x}, nil)
	}
	waitFor(t, "removed", func() bool {
		for _, raw := range nt.all("workspace.changed") {
			var m struct{ Removed []string }
			json.Unmarshal(*raw, &m)
			if len(m.Removed) == 1 {
				return true
			}
		}
		return false
	})
}
