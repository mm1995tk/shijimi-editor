// Package ctl is what the control socket promises its two sides: the messages
// (newline-delimited JSON, one Request in and one Response out per
// connection), the names of the environment variables and where the sockets
// live. The server is core/internal/ctlserver and the client is
// ui/cli/ctlclient.
package ctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/buildinfo"
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

// SocketDir returns the private (0700) directory that holds the sockets of
// this user's editors: $XDG_RUNTIME_DIR/shijimi-<uid>, or shijimi-nightly-<uid>
// for a nightly build, so that shj and shj-nightly do not touch each other's
// sockets. Without XDG_RUNTIME_DIR it is under the temporary directory. With
// create, the directory is made and its mode is enforced.
func SocketDir(create bool) (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, fmt.Sprintf("shijimi%s-%d", buildinfo.Suffix(), os.Getuid()))
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
// <SocketDir>/<pid>.sock. shj lists this directory to find a running editor
// when SHIJIMI_SOCKET is not set, so the directory holds nothing else but the
// per-workspace directories of the herdr links.
func SocketPath() (string, error) {
	dir, err := SocketDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%d.sock", os.Getpid())), nil
}

// ServeSocketPath returns the socket of "shj serve": <SocketDir>/serve.rpc.
// The name does not end in ".sock" on purpose: ctlclient.FindSocket treats
// every "*.sock" in the directory as an editor and would count the server as
// one.
func ServeSocketPath() (string, error) {
	dir, err := SocketDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "serve.rpc"), nil
}

// ServeLockPath returns the lock file that "shj attach" takes with flock while
// it starts "shj serve": <SocketDir>/serve.lock.
func ServeLockPath() (string, error) {
	dir, err := SocketDir(true)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "serve.lock"), nil
}

// HerdrDir is the directory of the links of the editors of one herdr workspace.
func HerdrDir(dir, ws string) string {
	return filepath.Join(dir, "herdr-"+SafeName(ws))
}

// SafeName makes a herdr id usable as a file name: anything but letters,
// digits, ".", "_" and "-" becomes "_".
func SafeName(id string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		}
		return '_'
	}, id)
}
