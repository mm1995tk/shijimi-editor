package ide

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func testServer(t *testing.T, h Handler) *Server {
	t.Helper()
	dir := t.TempDir()
	tools := []Tool{{Name: "echo", Description: "d", InputSchema: map[string]any{"type": "object"}}}
	s, err := start(dir, "/work", tools, h)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func dial(t *testing.T, s *Server, hdr http.Header) (*websocket.Conn, *http.Response, error) {
	d := websocket.Dialer{Subprotocols: []string{"mcp"}}
	return d.Dial(fmt.Sprintf("ws://127.0.0.1:%d", s.Port()), hdr)
}

func authed(s *Server) http.Header { return http.Header{AuthHeader: {s.Token()}} }

func rpc(t *testing.T, c *websocket.Conn, id int, method string, params any) map[string]any {
	t.Helper()
	if err := c.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var m map[string]any
	if err := c.ReadJSON(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLockFile(t *testing.T) {
	s := testServer(t, nil)
	b, err := os.ReadFile(s.LockFile())
	if err != nil {
		t.Fatal(err)
	}
	var l map[string]any
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	if l["ideName"] != Name || l["transport"] != "ws" || l["authToken"] != s.Token() ||
		l["pid"] != float64(os.Getpid()) || l["workspaceFolders"].([]any)[0] != "/work" {
		t.Errorf("lock = %v", l)
	}
	if st, _ := os.Stat(s.LockFile()); st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", st.Mode())
	}
	s.Close()
	if _, err := os.Stat(s.LockFile()); !os.IsNotExist(err) {
		t.Errorf("lock file remains: %v", err)
	}
}

func TestRemoveStale(t *testing.T) {
	dir := t.TempDir()
	stale := dir + "/1.lock"
	other := dir + "/2.lock"
	live := dir + "/3.lock"
	os.WriteFile(stale, []byte(`{"pid":2147483646,"ideName":"dageki"}`), 0o600)
	os.WriteFile(other, []byte(`{"pid":2147483646,"ideName":"Neovim"}`), 0o600)
	os.WriteFile(live, []byte(fmt.Sprintf(`{"pid":%d,"ideName":"dageki"}`, os.Getpid())), 0o600)
	removeStale(dir)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale lock kept")
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("other IDE's lock removed")
	}
	if _, err := os.Stat(live); err != nil {
		t.Error("live lock removed")
	}
}

func TestAuthRequired(t *testing.T) {
	s := testServer(t, nil)
	if _, resp, err := dial(t, s, nil); err == nil || resp == nil || resp.StatusCode != 401 {
		t.Errorf("no token: err=%v resp=%v", err, resp)
	}
	if _, _, err := dial(t, s, http.Header{AuthHeader: {"wrong"}}); err == nil {
		t.Error("wrong token accepted")
	}
	h := authed(s)
	h.Set("Origin", "http://evil.example")
	if _, _, err := dial(t, s, h); err == nil {
		t.Error("browser origin accepted")
	}
	c, _, err := dial(t, s, authed(s))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestProtocol(t *testing.T) {
	s := testServer(t, func(ctx context.Context, tool string, args json.RawMessage) ([]Content, error) {
		if tool == "boom" {
			return nil, fmt.Errorf("failed")
		}
		return []Content{Text(tool + string(args))}, nil
	})
	c, _, err := dial(t, s, authed(s))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Subprotocol() != "mcp" {
		t.Errorf("subprotocol = %q", c.Subprotocol())
	}
	r := rpc(t, c, 1, "initialize", map[string]any{"protocolVersion": "2025-03-26"})
	if res := r["result"].(map[string]any); res["protocolVersion"] != "2025-03-26" ||
		res["serverInfo"].(map[string]any)["name"] != Name {
		t.Errorf("initialize = %v", r)
	}
	c.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	r = rpc(t, c, 2, "tools/list", nil)
	if ts := r["result"].(map[string]any)["tools"].([]any); len(ts) != 1 || ts[0].(map[string]any)["name"] != "echo" {
		t.Errorf("tools/list = %v", r)
	}
	r = rpc(t, c, 3, "tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"a": 1}})
	got := r["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"]
	if got != `echo{"a":1}` {
		t.Errorf("tools/call = %v", got)
	}
	r = rpc(t, c, 4, "tools/call", map[string]any{"name": "boom"})
	if res := r["result"].(map[string]any); res["isError"] != true {
		t.Errorf("error call = %v", r)
	}
	r = rpc(t, c, 5, "nope", nil)
	if r["error"].(map[string]any)["code"] != float64(-32601) {
		t.Errorf("unknown method = %v", r)
	}
	if r = rpc(t, c, 6, "ping", nil); r["error"] != nil {
		t.Errorf("ping = %v", r)
	}
}

func TestNotifyAndConnected(t *testing.T) {
	s := testServer(t, nil)
	if s.Connected() {
		t.Fatal("connected before dial")
	}
	c, _, err := dial(t, s, authed(s))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	rpc(t, c, 1, "ping", nil)
	if !s.Connected() {
		t.Fatal("not connected")
	}
	s.Notify("selection_changed", map[string]any{"filePath": "/a"})
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var m map[string]any
	if err := c.ReadJSON(&m); err != nil {
		t.Fatal(err)
	}
	if m["method"] != "selection_changed" || m["id"] != nil {
		t.Errorf("notification = %v", m)
	}
}

func TestCancelOnDisconnect(t *testing.T) {
	started := make(chan struct{})
	done := make(chan struct{})
	s := testServer(t, func(ctx context.Context, tool string, args json.RawMessage) ([]Content, error) {
		close(started)
		<-ctx.Done()
		close(done)
		return nil, ctx.Err()
	})
	c, _, err := dial(t, s, authed(s))
	if err != nil {
		t.Fatal(err)
	}
	c.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "wait"}})
	<-started
	c.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler context not cancelled")
	}
}
