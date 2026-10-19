// Package ide implements the server side of Claude Code's IDE integration: a
// WebSocket MCP server on the loopback interface, advertised to Claude Code
// through a lock file in ~/.claude/ide. The protocol is the one used by the
// VS Code extension and claudecode.nvim.
package ide

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Name is shown by Claude Code as the connected IDE.
	Name = "dageki"
	// AuthHeader carries the token from the lock file.
	AuthHeader = "x-claude-code-ide-authorization"
	// EnvPort, EnvEnable and EnvForceTerminal make a Claude Code started in the terminal panel
	// connect to this server.
	EnvPort          = "CLAUDE_CODE_SSE_PORT"
	EnvEnable        = "ENABLE_IDE_INTEGRATION"
	EnvForceTerminal = "FORCE_CODE_TERMINAL"

	protocolVersion = "2024-11-05"
	writeTimeout    = 5 * time.Second
)

// Content is one MCP result block.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Text returns a text block.
func Text(s string) Content { return Content{Type: "text", Text: s} }

// JSON returns a text block holding v as JSON, which is how the IDE tools of
// Claude Code return structured data.
func JSON(v any) Content {
	b, _ := json.Marshal(v)
	return Text(string(b))
}

// Tool describes a tool offered to Claude Code.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Handler runs a tool call. It may block (openDiff waits for the user); ctx
// is cancelled when the connection closes.
type Handler func(ctx context.Context, tool string, args json.RawMessage) ([]Content, error)

// Server is the WebSocket MCP server.
type Server struct {
	ln       net.Listener
	http     *http.Server
	lock     string
	token    string
	tools    []Tool
	handler  Handler
	upgrader websocket.Upgrader
	out      chan []byte

	mu    sync.Mutex
	conns map[*conn]struct{}
}

type conn struct {
	ws *websocket.Conn
	mu sync.Mutex // serialises writes
}

func (c *conn) write(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.ws.WriteMessage(websocket.TextMessage, b)
}

// ConfigDir returns Claude Code's configuration directory.
func ConfigDir() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// Start listens on a random loopback port, writes the lock file for the
// workspace root and serves until Close.
func Start(root string, tools []Tool, h Handler) (*Server, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	return start(filepath.Join(dir, "ide"), root, tools, h)
}

func start(lockDir, root string, tools []Tool, h Handler) (*Server, error) {
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, err
	}
	removeStale(lockDir)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{
		ln:      ln,
		token:   hex.EncodeToString(tok),
		tools:   tools,
		handler: h,
		out:     make(chan []byte, 64),
		conns:   map[*conn]struct{}{},
	}
	s.upgrader = websocket.Upgrader{Subprotocols: []string{"mcp"}}
	s.http = &http.Server{Handler: http.HandlerFunc(s.serveWS), ReadHeaderTimeout: 10 * time.Second}
	lock, _ := json.Marshal(map[string]any{
		"pid":              os.Getpid(),
		"workspaceFolders": []string{root},
		"ideName":          Name,
		"transport":        "ws",
		"runningInWindows": false,
		"authToken":        s.token,
	})
	s.lock = filepath.Join(lockDir, strconv.Itoa(s.Port())+".lock")
	if err := os.WriteFile(s.lock, lock, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	go func() { _ = s.http.Serve(ln) }()
	go s.broadcast()
	return s, nil
}

// removeStale deletes lock files left by dageki processes that are gone.
func removeStale(dir string) {
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".lock") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var l struct {
			PID     int    `json:"pid"`
			IDEName string `json:"ideName"`
		}
		if json.Unmarshal(b, &l) != nil || l.IDEName != Name || l.PID <= 0 {
			continue
		}
		if err := syscall.Kill(l.PID, 0); errors.Is(err, syscall.ESRCH) {
			_ = os.Remove(p)
		}
	}
}

// Port is the TCP port the server listens on.
func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// Token is the authorization token written to the lock file.
func (s *Server) Token() string { return s.token }

// LockFile is the path of the lock file.
func (s *Server) LockFile() string { return s.lock }

// Connected reports whether a Claude Code client is attached.
func (s *Server) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns) > 0
}

// Close stops the server and removes the lock file.
func (s *Server) Close() {
	_ = os.Remove(s.lock)
	_ = s.http.Close()
	s.mu.Lock()
	for c := range s.conns {
		c.ws.Close()
	}
	s.mu.Unlock()
}

// Notify sends a JSON-RPC notification to every client. It never blocks; if
// the queue is full the notification is dropped.
func (s *Server) Notify(method string, params any) {
	b, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return
	}
	select {
	case s.out <- b:
	default:
	}
}

func (s *Server) broadcast() {
	for b := range s.out {
		s.mu.Lock()
		cs := make([]*conn, 0, len(s.conns))
		for c := range s.conns {
			cs = append(cs, c)
		}
		s.mu.Unlock()
		for _, c := range cs {
			_ = c.write(b)
		}
	}
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	// Browsers always send Origin; the real clients do not. Refusing it keeps
	// web pages from reaching the server.
	if r.Header.Get("Origin") != "" || r.Header.Get(AuthHeader) != s.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &conn{ws: ws}
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		ws.Close()
	}()
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}
		go s.handle(ctx, c, msg)
	}
}

type rpcRequest struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handle(ctx context.Context, c *conn, msg []byte) {
	var req rpcRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		s.reply(c, nil, nil, &rpcError{-32700, "parse error"})
		return
	}
	if len(req.ID) == 0 {
		return // notifications (notifications/initialized, ...) need no reply
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := p.ProtocolVersion
		if v == "" {
			v = protocolVersion
		}
		s.reply(c, req.ID, map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
			"serverInfo":      map[string]any{"name": Name, "version": "0.1.0"},
		}, nil)
	case "ping":
		s.reply(c, req.ID, map[string]any{}, nil)
	case "tools/list":
		s.reply(c, req.ID, map[string]any{"tools": s.tools}, nil)
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			s.reply(c, req.ID, nil, &rpcError{-32602, "invalid params"})
			return
		}
		content, err := s.handler(ctx, p.Name, p.Arguments)
		if err != nil {
			s.reply(c, req.ID, map[string]any{
				"content": []Content{Text(err.Error())},
				"isError": true,
			}, nil)
			return
		}
		s.reply(c, req.ID, map[string]any{"content": content}, nil)
	default:
		s.reply(c, req.ID, nil, &rpcError{-32601, fmt.Sprintf("method not found: %s", req.Method)})
	}
}

func (s *Server) reply(c *conn, id json.RawMessage, result any, e *rpcError) {
	m := map[string]any{"jsonrpc": "2.0"}
	if len(id) == 0 {
		m["id"] = nil
	} else {
		m["id"] = id
	}
	if e != nil {
		m["error"] = e
	} else {
		m["result"] = result
	}
	b, _ := json.Marshal(m)
	_ = c.write(b)
}
