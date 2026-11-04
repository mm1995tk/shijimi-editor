// Package ctlclient is the side of the control socket that runs in the shj
// subcommands ("shj diff", "shj exec", ...): it finds the editor's socket,
// sends one request and prints the reply.
package ctlclient

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Subcommands are matched before file paths; write "./diff" to open a file
// with the same name.
var Subcommands = []string{"diff", "exec", "command-end", "skill"}

// IsSubcommand reports whether arg selects a subcommand.
func IsSubcommand(arg string) bool {
	return slices.Contains(Subcommands, arg)
}

// FindSocket returns the socket shj talks to: SHIJIMI_SOCKET when it is set;
// otherwise, inside a herdr workspace, the link of the pane HERDR_PANE_ID or
// the only live link of the workspace; otherwise the only live socket of this
// user. Sockets that do not accept a connection (left behind by an editor that
// was killed) are skipped. More than one candidate is an error, because shj
// cannot tell which editor is meant.
func FindSocket() (string, error) {
	if sock := os.Getenv(ctl.EnvSocket); sock != "" {
		return sock, nil
	}
	dir, err := ctl.SocketDir(false)
	if err != nil {
		return "", fmt.Errorf(i18n.T("ctl.nosocket"), ctl.EnvSocket)
	}
	var live []string
	if ws := os.Getenv(ctl.EnvHerdrWorkspace); ws != "" {
		wsDir := ctl.HerdrDir(dir, ws)
		if pane := os.Getenv(ctl.EnvHerdrPane); pane != "" {
			if p := filepath.Join(wsDir, ctl.SafeName(pane)+".sock"); alive(p) {
				return p, nil
			}
		}
		live = liveSockets(wsDir)
	} else {
		live = liveSockets(dir)
	}
	switch len(live) {
	case 0:
		return "", fmt.Errorf(i18n.T("ctl.nosocket"), ctl.EnvSocket)
	case 1:
		return live[0], nil
	}
	return "", fmt.Errorf(i18n.T("ctl.ambiguous"), ctl.EnvSocket, len(live), ctl.EnvSocket)
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

// Call sends req to the editor found by FindSocket.
func Call(req ctl.Request) (ctl.Response, error) {
	sock, err := FindSocket()
	if err != nil {
		return ctl.Response{}, err
	}
	c, err := net.DialTimeout("unix", sock, 3*time.Second)
	if err != nil {
		return ctl.Response{}, fmt.Errorf(i18n.T("ctl.connect"), sock, err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return ctl.Response{}, err
	}
	var resp ctl.Response
	if err := json.NewDecoder(c).Decode(&resp); err != nil {
		return ctl.Response{}, err
	}
	return resp, nil
}

// Run executes a subcommand and returns the process exit code.
func Run(args []string, stdout, stderr func(string)) int {
	var req ctl.Request
	switch args[0] {
	case "diff":
		if len(args) > 2 {
			stderr("usage: shj diff [path]")
			return 2
		}
		req = ctl.Request{Cmd: "diff"}
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
		req = ctl.Request{Cmd: "exec", Line: strings.Join(args[1:], " ")}
	case "command-end":
		if len(args) != 2 {
			stderr(i18n.T("ctl.usage.commandend"))
			return 2
		}
		req = ctl.Request{Cmd: "command-end", Line: args[1]}
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
