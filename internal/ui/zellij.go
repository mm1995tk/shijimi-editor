package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// zellijBackend is the muxBackend for zellij. Every operation is a call of
// "zellij action ...", which talks to the session that the editor runs in
// (through $ZELLIJ_SESSION_NAME). It needs a zellij whose actions take
// --pane-id and whose new-pane prints the id of the pane it created (0.44 or
// later; checked with 0.45.1). Panes are always named by id, so the focus does
// not decide which pane an action works on.

const (
	envZellij         = "ZELLIJ"
	envZellijPaneID   = "ZELLIJ_PANE_ID"
	zellijTimeout     = 5 * time.Second
	zellijEnterByte   = "13"
	zellijTerminalPfx = "terminal_"
)

type zellijBackend struct {
	// editorPane is the id of the pane the editor runs in ("terminal_3"),
	// "" when unknown.
	editorPane string
	// call runs "zellij action <args>" and returns its output; tests replace it.
	call func(args ...string) ([]byte, error)
}

func newZellijBackend() *zellijBackend {
	b := &zellijBackend{}
	if v := os.Getenv(envZellijPaneID); v != "" {
		b.editorPane = zellijPaneName(v)
	}
	return b
}

func (*zellijBackend) name() string { return "zellij" }

// zellijPaneName turns the number of ZELLIJ_PANE_ID into a pane id.
func zellijPaneName(v string) string {
	if _, err := strconv.Atoi(v); err == nil {
		return zellijTerminalPfx + v
	}
	return v
}

func zellijExec(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), zellijTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zellij", append([]string{"action"}, args...)...)
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

func (z *zellijBackend) do(args ...string) ([]byte, error) {
	if z.call != nil {
		return z.call(args...)
	}
	return zellijExec(args...)
}

// zellijPane is an entry of "list-panes --json".
type zellijPane struct {
	ID           int  `json:"id"`
	IsPlugin     bool `json:"is_plugin"`
	IsFullscreen bool `json:"is_fullscreen"`
	Exited       bool `json:"exited"`
}

func (p zellijPane) name() string {
	if p.IsPlugin {
		return "plugin_" + strconv.Itoa(p.ID)
	}
	return zellijTerminalPfx + strconv.Itoa(p.ID)
}

func (z *zellijBackend) panes() ([]zellijPane, error) {
	out, err := z.do("list-panes", "--json")
	if err != nil {
		return nil, err
	}
	var ps []zellijPane
	if err := json.Unmarshal(out, &ps); err != nil {
		return nil, errors.New(i18n.T("zellij.badreply"))
	}
	return ps, nil
}

func (z *zellijBackend) alive(id string) bool {
	ps, err := z.panes()
	if err != nil {
		return false
	}
	for _, p := range ps {
		if p.name() == id {
			return !p.Exited
		}
	}
	return false
}

// paneArg is the --pane-id argument of an action on pane id.
func paneArg(id string) []string {
	if id == "" {
		return nil
	}
	return []string{"--pane-id", id}
}

func (z *zellijBackend) split(cwd string, env []string) (string, error) {
	// new-pane has no way to set the environment of the pane, so the shell is
	// started through env(1). -c closes the pane when the shell exits.
	args := []string{"new-pane", "--direction", "down", "--close-on-exit", "--near-current-pane", "--cwd", cwd, "--", "env"}
	args = append(args, env...)
	args = append(args, shell())
	out, err := z.do(args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	if !strings.HasPrefix(id, zellijTerminalPfx) {
		return "", errors.New(i18n.T("zellij.badreply"))
	}
	if _, err := z.do("focus-pane-id", id); err != nil {
		return id, err
	}
	return id, nil
}

// fullscreen reports whether the editor's pane is shown in fullscreen.
func (z *zellijBackend) fullscreen() (bool, error) {
	ps, err := z.panes()
	if err != nil {
		return false, err
	}
	for _, p := range ps {
		if p.name() == z.editorPane {
			return p.IsFullscreen, nil
		}
	}
	return false, nil
}

// toggleFullscreen is toggle-fullscreen on the editor's pane, which has no
// "on" and "off" either.
func (z *zellijBackend) toggleFullscreen() error {
	_, err := z.do(append([]string{"toggle-fullscreen"}, paneArg(z.editorPane)...)...)
	return err
}

func (z *zellijBackend) show(id string) error {
	f, err := z.fullscreen()
	if err != nil {
		return err
	}
	if f {
		if err := z.toggleFullscreen(); err != nil {
			return err
		}
	}
	_, err = z.do("focus-pane-id", id)
	return err
}

func (z *zellijBackend) hide() error {
	f, err := z.fullscreen()
	if err != nil || f {
		return err
	}
	return z.toggleFullscreen()
}

func (z *zellijBackend) send(id, text string, enter bool) error {
	if enter {
		if _, err := z.do(append(append([]string{"write-chars"}, paneArg(id)...), "--", text)...); err != nil {
			return err
		}
		_, err := z.do(append([]string{"write"}, append(paneArg(id), zellijEnterByte)...)...)
		return err
	}
	// A bracketed paste keeps a multi-line text from running.
	_, err := z.do(append(append([]string{"paste"}, paneArg(id)...), "--", text)...)
	return err
}

func (z *zellijBackend) tail(id string, n int) []string {
	out, err := z.do(append([]string{"dump-screen"}, paneArg(id)...)...)
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

func (z *zellijBackend) close(id string) {
	_, _ = z.do(append([]string{"close-pane"}, paneArg(id)...)...)
}

func (*zellijBackend) shellEnv(sh, exe string) []string { return muxShellIntegration(sh, exe) }
