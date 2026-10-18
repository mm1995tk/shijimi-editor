// Package ctl implements the control socket that lets other processes (for
// example AI agents) drive a running gh-editor. Messages are newline-delimited
// JSON: one Request in, one Response out, per connection.
package ctl

import (
	"slices"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnvSocket is the environment variable that carries the socket path.
const EnvSocket = "GH_EDITOR_SOCKET"

// Subcommands are matched before file paths; write "./diff" to open a file
// with the same name.
var Subcommands = []string{"diff", "skill"}

// IsSubcommand reports whether arg selects a subcommand.
func IsSubcommand(arg string) bool {
	return slices.Contains(Subcommands, arg)
}

// Request is sent by the CLI to the editor.
type Request struct {
	Cmd  string `json:"cmd"` // diff
	Path string `json:"path,omitempty"`
}

// Response is the editor's reply.
type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Handler runs a request inside the editor and returns the reply.
type Handler func(Request) Response

// Server accepts connections on a Unix socket.
type Server struct {
	Path string
	ln   net.Listener
}

// SocketPath returns a fresh socket path inside a private (0700) directory.
func SocketPath() (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, fmt.Sprintf("gh-editor-%d", os.Getuid()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%d.sock", os.Getpid())), nil
}

// Listen creates the socket (mode 0600) and serves h until Close.
func Listen(path string, h Handler) (*Server, error) {
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{Path: path, ln: ln}
	go s.serve(h)
	return s, nil
}

// Close stops serving and removes the socket file.
func (s *Server) Close() {
	s.ln.Close()
	_ = os.Remove(s.Path)
}

func (s *Server) serve(h Handler) {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(10 * time.Second))
			var req Request
			var resp Response
			line, err := bufio.NewReader(c).ReadBytes('\n')
			if err == nil || len(line) > 0 {
				err = json.Unmarshal(line, &req)
			}
			if err != nil {
				resp = Response{Error: "リクエストを解釈できません: " + err.Error()}
			} else {
				resp = h(req)
			}
			_ = json.NewEncoder(c).Encode(resp)
		}()
	}
}

// Call sends req to the editor named by GH_EDITOR_SOCKET.
func Call(req Request) (Response, error) {
	sock := os.Getenv(EnvSocket)
	if sock == "" {
		return Response{}, fmt.Errorf("%s が設定されていません。gh-editor のターミナルパネル内で実行してください", EnvSocket)
	}
	c, err := net.DialTimeout("unix", sock, 3*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("gh-editor に接続できません (%s): %w", sock, err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.NewDecoder(c).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// Run executes a subcommand and returns the process exit code.
func Run(args []string, stdout, stderr func(string)) int {
	var req Request
	switch args[0] {
	case "diff":
		if len(args) > 2 {
			stderr("usage: gh-editor diff [path]")
			return 2
		}
		req = Request{Cmd: "diff"}
		if len(args) == 2 {
			abs, err := filepath.Abs(args[1])
			if err != nil {
				stderr(err.Error())
				return 1
			}
			req.Path = abs
		}
	case "skill":
		return runSkill(args[1:], stdout, stderr)
	default:
		stderr("不明なサブコマンド: " + args[0])
		return 2
	}
	resp, err := Call(req)
	if err != nil {
		stderr(err.Error())
		return 1
	}
	if !resp.OK {
		stderr(resp.Error)
		return 1
	}
	return 0
}
