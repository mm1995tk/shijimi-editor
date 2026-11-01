package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/term"
)

// cmuxBackend is the muxBackend for cmux (the macOS terminal app for coding
// agents). Every operation is a call of the "cmux" CLI, which talks to the app
// through its socket. Panes are cmux "surfaces" and are always named by their
// uuid (--id-format uuids), so the focus does not decide which one is meant.
//
// cmux has no zoom, so a hidden pane is not hidden: hide() only moves focus
// back to the editor's surface. new-split has neither --cwd nor --env, so the
// working directory and the environment are given in the command that cmux
// types into the new pane's shell (--command).

const (
	envCmuxWorkspace = "CMUX_WORKSPACE_ID"
	envCmuxSurface   = "CMUX_SURFACE_ID"
	cmuxOS           = "darwin"
	cmuxTimeout      = 5 * time.Second
)

// cmuxGOOS is runtime.GOOS; tests replace it.
var cmuxGOOS = runtime.GOOS

type cmuxBackend struct {
	// editorPane is the surface the editor runs in, "" when unknown.
	editorPane string
	workspace  string
	// call runs "cmux <args>" and returns its output; tests replace it.
	call func(args ...string) ([]byte, error)
}

func newCmuxBackend() *cmuxBackend {
	return &cmuxBackend{
		editorPane: os.Getenv(envCmuxSurface),
		workspace:  os.Getenv(envCmuxWorkspace),
	}
}

func (*cmuxBackend) name() string { return "cmux" }

func cmuxExec(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmuxTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "cmux", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = errors.New(msg)
		}
		return out, err
	}
	return out, nil
}

func (c *cmuxBackend) do(args ...string) ([]byte, error) {
	if c.call != nil {
		return c.call(args...)
	}
	return cmuxExec(args...)
}

// cmuxIDs makes cmux print uuids instead of refs ("surface:3").
var cmuxIDs = []string{"--id-format", "uuids"}

func cmuxArgs(rest ...string) []string {
	return append(append([]string{"--json"}, cmuxIDs...), rest...)
}

// containsString reports whether the JSON value v holds the string s anywhere.
func containsString(v any, s string) bool {
	switch x := v.(type) {
	case string:
		return x == s
	case []any:
		for _, e := range x {
			if containsString(e, s) {
				return true
			}
		}
	case map[string]any:
		for _, e := range x {
			if containsString(e, s) {
				return true
			}
		}
	}
	return false
}

// alive reports whether the surface is still in the workspace. A surface is
// closed when its shell exits.
func (c *cmuxBackend) alive(id string) bool {
	out, err := c.do(cmuxArgs("list-panels")...)
	if err != nil {
		return false
	}
	var v any
	if json.Unmarshal(out, &v) != nil {
		return false
	}
	return containsString(v, id)
}

// cmuxCommand is the command line that is typed into the new pane's shell: it
// goes to cwd and replaces the shell with one that has env.
func cmuxCommand(cwd string, env []string, sh string) string {
	var b strings.Builder
	b.WriteString("cd " + shQuote(cwd) + " && exec env")
	for _, e := range env {
		b.WriteString(" " + shQuote(e))
	}
	b.WriteString(" " + shQuote(sh))
	return b.String()
}

func (c *cmuxBackend) split(cwd string, env []string) (string, error) {
	args := []string{"new-split", "down", "--focus", "true"}
	if c.editorPane != "" {
		args = append(args, "--surface", c.editorPane)
	}
	args = append(args, "--command", cmuxCommand(cwd, env, term.DefaultShell()))
	out, err := c.do(cmuxArgs(args...)...)
	if err != nil {
		return "", err
	}
	var p struct {
		SurfaceID  string `json:"surface_id"`
		SurfaceRef string `json:"surface_ref"`
	}
	if json.Unmarshal(out, &p) != nil {
		return "", errors.New(i18n.T("cmux.badreply"))
	}
	id := p.SurfaceID
	if id == "" {
		id = p.SurfaceRef
	}
	if id == "" {
		return "", errors.New(i18n.T("cmux.badreply"))
	}
	return id, nil
}

func (c *cmuxBackend) focus(id string) error {
	_, err := c.do("focus-panel", "--panel", id)
	return err
}

func (c *cmuxBackend) show(id string) error { return c.focus(id) }

// hide moves focus back to the editor; cmux cannot hide a pane.
func (c *cmuxBackend) hide() error {
	if c.editorPane == "" {
		return errors.New(i18n.T("cmux.noeditor"))
	}
	return c.focus(c.editorPane)
}

func (c *cmuxBackend) send(id, text string, enter bool) error {
	// paste sends the text unchanged (send would turn "\n" into Enter). The
	// guard that refuses to type over an agent's draft is kept for a line that
	// is run, and skipped for a paste that the person confirms anyway.
	args := []string{"paste", "--surface", id}
	if !enter {
		args = append(args, "--force")
	}
	if _, err := c.do(append(args, "--", text)...); err != nil {
		return err
	}
	if enter {
		_, err := c.do("send-key", "--surface", id, "enter")
		return err
	}
	return nil
}

func (c *cmuxBackend) tail(id string, n int) []string {
	out, err := c.do("read-screen", "--surface", id, "--lines", "50")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(out), " \t\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func (c *cmuxBackend) close(id string) {
	args := []string{"close-surface", "--surface", id, "--force"}
	if c.workspace != "" {
		args = append(args, "--workspace", c.workspace)
	}
	_, _ = c.do(args...)
}

func (*cmuxBackend) shellEnv(sh, exe string) []string { return muxShellIntegration(sh, exe) }
