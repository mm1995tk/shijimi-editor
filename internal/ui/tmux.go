package ui

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// tmuxBackend is the muxBackend for tmux. Every operation is a call of the
// tmux command, which talks to the server of the session that the editor runs
// in (through $TMUX). It needs tmux 3.1 or later: split-window -e is 3.0, and
// -l with a percentage is 3.1.

const (
	envTmux     = "TMUX"
	envTmuxPane = "TMUX_PANE"

	tmuxSize    = "30%"
	tmuxTimeout = 5 * time.Second
)

type tmuxBackend struct {
	// editorPane is the pane the editor runs in (TMUX_PANE), "" when unknown.
	editorPane string
	// call runs a tmux command and returns its output; tests replace it.
	call func(args ...string) ([]byte, error)
}

func newTmuxBackend() *tmuxBackend { return &tmuxBackend{editorPane: os.Getenv(envTmuxPane)} }

func (*tmuxBackend) name() string { return "tmux" }

func tmuxExec(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tmux", args...)
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

func (t *tmuxBackend) do(args ...string) ([]byte, error) {
	if t.call != nil {
		return t.call(args...)
	}
	return tmuxExec(args...)
}

// target is the -t argument for the editor's own pane.
func (t *tmuxBackend) target() []string {
	if t.editorPane == "" {
		return nil
	}
	return []string{"-t", t.editorPane}
}

func (t *tmuxBackend) alive(id string) bool {
	out, err := t.do("list-panes", "-a", "-F", "#{pane_id} #{pane_dead}")
	if err != nil {
		return false
	}
	for l := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(l) == id+" 0" {
			return true
		}
	}
	return false
}

func (t *tmuxBackend) split(cwd string, env []string) (string, error) {
	args := []string{"split-window", "-v", "-l", tmuxSize, "-c", cwd, "-P", "-F", "#{pane_id}"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, t.target()...)
	out, err := t.do(args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", errors.New(i18n.T("tmux.badreply"))
	}
	return id, nil
}

// zoomed reports whether the window of the editor's pane is zoomed.
func (t *tmuxBackend) zoomed() (bool, error) {
	args := append([]string{"display-message", "-p"}, t.target()...)
	out, err := t.do(append(args, "#{window_zoomed_flag}")...)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "1", nil
}

// toggleZoom is resize-pane -Z on the editor's pane, which zooms it when it is
// not zoomed and restores the layout when it is. tmux has no "on" and "off".
func (t *tmuxBackend) toggleZoom() error {
	_, err := t.do(append([]string{"resize-pane", "-Z"}, t.target()...)...)
	return err
}

func (t *tmuxBackend) show(id string) error {
	z, err := t.zoomed()
	if err != nil {
		return err
	}
	if z {
		if err := t.toggleZoom(); err != nil {
			return err
		}
	}
	_, err = t.do("select-pane", "-t", id)
	return err
}

func (t *tmuxBackend) hide() error {
	z, err := t.zoomed()
	if err != nil || z {
		return err
	}
	return t.toggleZoom()
}

func (t *tmuxBackend) send(id, text string, enter bool) error {
	if _, err := t.do("send-keys", "-t", id, "-l", "--", text); err != nil {
		return err
	}
	if enter {
		_, err := t.do("send-keys", "-t", id, "Enter")
		return err
	}
	return nil
}

func (t *tmuxBackend) tail(id string, n int) []string {
	out, err := t.do("capture-pane", "-p", "-J", "-t", id, "-S", "-"+strconv.Itoa(n+200))
	if err != nil {
		return nil
	}
	// The screen is padded with empty lines below the cursor.
	lines := strings.Split(strings.TrimRight(string(out), " \t\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func (t *tmuxBackend) close(id string) { _, _ = t.do("kill-pane", "-t", id) }

func (*tmuxBackend) shellEnv(sh, exe string) []string { return muxShellIntegration(sh, exe) }
