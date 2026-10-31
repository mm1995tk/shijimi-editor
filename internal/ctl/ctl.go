// Package ctl implements the control socket that lets other processes (for
// example AI agents) drive a running shijimi. Messages are newline-delimited
// JSON: one Request in, one Response out, per connection.
package ctl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/channel"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// EnvSocket is the environment variable that carries the socket path.
const EnvSocket = "SHIJIMI_SOCKET"

// The herdr terminal multiplexer sets these in the panes it runs. The editor
// uses them to publish its socket under a name that shj can build from the
// same variables when SHIJIMI_SOCKET is not passed along (herdr's key bound
// shell commands, for example).
const (
	EnvHerdr          = "HERDR_ENV"
	EnvHerdrWorkspace = "HERDR_WORKSPACE_ID"
	EnvHerdrPane      = "HERDR_PANE_ID"
)

// Subcommands are matched before file paths; write "./diff" to open a file
// with the same name.
var Subcommands = []string{"diff", "exec", "command-end", "skill"}

// IsSubcommand reports whether arg selects a subcommand.
func IsSubcommand(arg string) bool {
	return slices.Contains(Subcommands, arg)
}

// Request is sent by the CLI to the editor.
type Request struct {
	Cmd  string `json:"cmd"` // diff | exec | command-end
	Path string `json:"path,omitempty"`
	Line string `json:"line,omitempty"` // exec: the ":" command to run; command-end: the exit code
}

// Response is the editor's reply.
type Response struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"` // exec: the status line after the command
}

// Handler runs a request inside the editor and returns the reply.
type Handler func(Request) Response

// Server accepts connections on a Unix socket.
type Server struct {
	Path string
	link string // the herdr link to Path, or ""
	ln   net.Listener
}

// socketDir returns the private (0700) directory that holds the sockets of
// this user's editors: $XDG_RUNTIME_DIR/shijimi-<uid>, or shijimi-nightly-<uid>
// for a nightly build, so that shj and shj-nightly do not touch each other's
// sockets. Without XDG_RUNTIME_DIR it is under the temporary directory. With
// create, the directory is made and its mode is enforced.
func socketDir(create bool) (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, fmt.Sprintf("shijimi%s-%d", channel.Suffix(), os.Getuid()))
	if !create {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SocketPath returns a fresh socket path inside the socket directory:
// <socketDir>/<pid>.sock. shj lists this directory to find a running editor
// when SHIJIMI_SOCKET is not set, so the directory holds nothing else but the
// per-workspace directories of the herdr links.
func SocketPath() (string, error) {
	dir, err := socketDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%d.sock", os.Getpid())), nil
}

// ServeSocketPath returns the socket of "shj serve": <socketDir>/serve.rpc.
// The name does not end in ".sock" on purpose: FindSocket treats every
// "*.sock" in the directory as an editor and would count the server as one.
func ServeSocketPath() (string, error) {
	dir, err := socketDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "serve.rpc"), nil
}

// ServeLockPath returns the lock file that "shj attach" takes with flock while
// it starts "shj serve": <socketDir>/serve.lock.
func ServeLockPath() (string, error) {
	dir, err := socketDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "serve.lock"), nil
}

// Listen creates the socket (mode 0600) and serves h until Close. Inside a
// herdr pane it also links the socket under a name built from the herdr
// environment (see herdrLink), so that shj finds it without SHIJIMI_SOCKET.
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
	// The link is best-effort: shj still works through SHIJIMI_SOCKET.
	if link := herdrLink(filepath.Dir(path)); link != "" {
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err == nil {
			_ = os.Remove(link)
			if err := os.Symlink(path, link); err == nil {
				s.link = link
			}
		}
	}
	go s.serve(h)
	return s, nil
}

// Close stops serving and removes the socket file and the herdr link.
func (s *Server) Close() {
	s.ln.Close()
	_ = os.Remove(s.Path)
	if s.link != "" {
		_ = os.Remove(s.link)
		// The workspace directory goes too once it is empty.
		_ = os.Remove(filepath.Dir(s.link))
	}
}

// herdrLink returns where the editor of this herdr pane links its socket:
// <dir>/herdr-<workspace>/<pane>.sock, or <dir>/herdr-<workspace>/pid-<pid>.sock
// when herdr did not tell the pane. It returns "" outside herdr.
func herdrLink(dir string) string {
	if os.Getenv(EnvHerdr) != "1" {
		return ""
	}
	ws := os.Getenv(EnvHerdrWorkspace)
	if ws == "" {
		return ""
	}
	name := "pid-" + strconv.Itoa(os.Getpid())
	if pane := os.Getenv(EnvHerdrPane); pane != "" {
		name = safeName(pane)
	}
	return filepath.Join(herdrDir(dir, ws), name+".sock")
}

// herdrDir is the directory of the links of the editors of one herdr workspace.
func herdrDir(dir, ws string) string {
	return filepath.Join(dir, "herdr-"+safeName(ws))
}

// safeName makes a herdr id usable as a file name: anything but letters,
// digits, ".", "_" and "-" becomes "_".
func safeName(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		}
		return '_'
	}, id)
}

// FindSocket returns the socket shj talks to: SHIJIMI_SOCKET when it is set;
// otherwise, inside a herdr workspace, the link of the pane HERDR_PANE_ID or
// the only live link of the workspace; otherwise the only live socket of this
// user. Sockets that do not accept a connection (left behind by an editor that
// was killed) are skipped. More than one candidate is an error, because shj
// cannot tell which editor is meant.
func FindSocket() (string, error) {
	if sock := os.Getenv(EnvSocket); sock != "" {
		return sock, nil
	}
	dir, err := socketDir(false)
	if err != nil {
		return "", fmt.Errorf(i18n.T("ctl.nosocket"), EnvSocket)
	}
	var live []string
	if ws := os.Getenv(EnvHerdrWorkspace); ws != "" {
		wsDir := herdrDir(dir, ws)
		if pane := os.Getenv(EnvHerdrPane); pane != "" {
			if p := filepath.Join(wsDir, safeName(pane)+".sock"); alive(p) {
				return p, nil
			}
		}
		live = liveSockets(wsDir)
	} else {
		live = liveSockets(dir)
	}
	switch len(live) {
	case 0:
		return "", fmt.Errorf(i18n.T("ctl.nosocket"), EnvSocket)
	case 1:
		return live[0], nil
	}
	return "", fmt.Errorf(i18n.T("ctl.ambiguous"), EnvSocket, len(live), EnvSocket)
}

// liveSockets lists the *.sock entries of dir that accept a connection.
func liveSockets(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var live []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sock") {
			continue
		}
		if p := filepath.Join(dir, e.Name()); alive(p) {
			live = append(live, p)
		}
	}
	return live
}

// alive reports whether a socket accepts a connection. The editor answers the
// empty connection with a "bad request" reply that nobody reads.
func alive(path string) bool {
	c, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
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
				resp = Response{Error: i18n.T("ctl.badrequest") + err.Error()}
			} else {
				resp = h(req)
			}
			_ = json.NewEncoder(c).Encode(resp)
		}()
	}
}

// Call sends req to the editor found by FindSocket.
func Call(req Request) (Response, error) {
	sock, err := FindSocket()
	if err != nil {
		return Response{}, err
	}
	c, err := net.DialTimeout("unix", sock, 3*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf(i18n.T("ctl.connect"), sock, err)
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
			stderr("usage: shj diff [path]")
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
	case "exec":
		if len(args) < 2 {
			stderr(i18n.T("ctl.usage.exec"))
			return 2
		}
		req = Request{Cmd: "exec", Line: strings.Join(args[1:], " ")}
	case "command-end":
		if len(args) != 2 {
			stderr(i18n.T("ctl.usage.commandend"))
			return 2
		}
		req = Request{Cmd: "command-end", Line: args[1]}
	case "skill":
		return runSkill(args[1:], stdout, stderr)
	default:
		stderr(i18n.T("ctl.unknownsub") + args[0])
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
	if resp.Message != "" {
		stdout(resp.Message)
	}
	return 0
}
