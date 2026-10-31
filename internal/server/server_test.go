package server

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/rpc"
)

func start(t *testing.T) (*Server, string) {
	t.Helper()
	// Unix ソケットのパスは短くないと作れない。
	dir, err := os.MkdirTemp("", "shjsrv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "serve.rpc")
	srv, err := Listen(Options{SocketPath: sock, Shj: Shj{Version: "v1", Commit: "abc", Channel: "stable"}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { srv.Serve(context.Background()); close(done) }()
	t.Cleanup(func() { srv.Close(); <-done })
	return srv, sock
}

func dial(t *testing.T, sock string) *rpc.Conn {
	t.Helper()
	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	cn := rpc.NewConn(c, rpc.JSON{}, nil)
	t.Cleanup(func() { cn.Close() })
	return cn
}

func hello(major int) map[string]any {
	return map[string]any{
		"protocol": map[string]int{"major": major, "minor": 0},
		"client":   map[string]string{"name": "test", "version": "0", "kind": "web"},
		"encoding": []string{"json"},
	}
}

func TestHelloInfoBye(t *testing.T) {
	srv, sock := start(t)
	c := dial(t, sock)
	ctx := context.Background()

	var info InfoResult
	err := c.Call(ctx, "server.info", struct{}{}, &info)
	if e, ok := err.(*rpc.Error); !ok || e.Code != rpc.CodeInvalidRequest {
		t.Fatalf("server.info before hello: %v", err)
	}

	var h HelloResult
	if err := c.Call(ctx, "session.hello", hello(1), &h); err != nil {
		t.Fatal(err)
	}
	if h.Protocol != Protocol || h.Encoding != "json" || h.ServerID != srv.ServerID() || h.Shj.Version != "v1" {
		t.Errorf("hello result: %+v", h)
	}
	if h.Capabilities == nil || h.Workspaces == nil {
		t.Errorf("capabilities and workspaces must be arrays")
	}
	if err := c.Call(ctx, "session.ping", struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "server.info", struct{}{}, &info); err != nil {
		t.Fatal(err)
	}
	if info.ServerID != h.ServerID || info.PID != os.Getpid() || info.LastClientAt == nil || info.IdleStopAt != nil {
		t.Errorf("info: %+v", info)
	}
	if err := c.Call(ctx, "session.bye", struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "no.such", struct{}{}, nil); err == nil || err.(*rpc.Error).Code != rpc.CodeMethodNotFound {
		t.Errorf("unknown method: %v", err)
	}
	// 知らない通知は無視され、接続は生きている。
	if err := c.Notify("no.such", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session.ping", struct{}{}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestIncompatibleProtocol(t *testing.T) {
	_, sock := start(t)
	c := dial(t, sock)
	err := c.Call(context.Background(), "session.hello", hello(2), nil)
	e, ok := err.(*rpc.Error)
	if !ok || e.Code != rpc.CodeIncompatibleProtocol {
		t.Fatalf("got %v", err)
	}
	// 新しい minor は互換。
	p := hello(1)
	p["protocol"] = map[string]int{"major": 1, "minor": 99}
	if err := c.Call(context.Background(), "session.hello", p, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStateSurvivesDisconnect(t *testing.T) {
	srv, sock := start(t)
	c := dial(t, sock)
	if err := c.Call(context.Background(), "session.hello", hello(1), nil); err != nil {
		t.Fatal(err)
	}
	if srv.lastClientAt() == nil {
		t.Fatal("connected: lastClientAt must be set")
	}
	c.Close()
	deadline := time.Now().Add(2 * time.Second)
	for {
		srv.mu.Lock()
		n := len(srv.sessions)
		srv.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session not dropped")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if srv.lastClientAt() == nil {
		t.Error("lastClientAt must be remembered after disconnect")
	}
	// サーバーは生きていて、つなぎ直せる。
	c2 := dial(t, sock)
	var h HelloResult
	if err := c2.Call(context.Background(), "session.hello", hello(1), &h); err != nil || h.ServerID != srv.ServerID() {
		t.Fatalf("reconnect: %v %+v", err, h)
	}
}

func TestAlreadyRunning(t *testing.T) {
	_, sock := start(t)
	if _, err := Listen(Options{SocketPath: sock}); err != ErrRunning {
		t.Errorf("got %v", err)
	}
}

func TestStaleSocketIsReplaced(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close() // ファイルだけ残る
	srv, err := Listen(Options{SocketPath: sock})
	if err != nil {
		t.Fatal(err)
	}
	srv.Close()
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("socket must be removed on close: %v", err)
	}
}

func TestLogTrimsHead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d", "serve.log")
	w, err := OpenLog(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for range 100 {
		line := strings.Repeat("x", 30) + "\n"
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	b, _ := os.ReadFile(path)
	if len(b) > 1000 || len(b) == 0 {
		t.Errorf("size %d", len(b))
	}
	if !strings.HasPrefix(string(b), "xxx") || !strings.HasSuffix(string(b), "\n") {
		t.Errorf("must keep whole lines")
	}
	// 開くときに上限を超えていれば捨てる。
	os.WriteFile(path, []byte(strings.Repeat("y\n", 2000)), 0o600)
	w, err = OpenLog(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	if st, _ := os.Stat(path); st.Size() > 1000 {
		t.Errorf("size after open %d", st.Size())
	}
}

func TestLogPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/s")
	if p, _ := LogPath(); p != "/s/shijimi-editor/serve.log" {
		t.Error(p)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/h")
	if p, _ := LogPath(); p != "/h/.local/state/shijimi-editor/serve.log" {
		t.Error(p)
	}
}
