package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// herdrBackend is the muxBackend for the herdr terminal multiplexer. Every
// operation is a call of its CLI.

const (
	envHerdr    = "HERDR_ENV"
	envHerdrBin = "HERDR_BIN_PATH"

	herdrRatio   = "0.3"
	herdrTimeout = 5 * time.Second
)

type herdrBackend struct {
	// call runs a herdr command and returns its output; tests replace it.
	call func(args ...string) ([]byte, error)
}

func (*herdrBackend) name() string { return "herdr" }

func herdrBin() string {
	if b := os.Getenv(envHerdrBin); b != "" {
		return b
	}
	return "herdr"
}

func herdrExec(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), herdrTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, herdrBin(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			var e struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(msg), &e) == nil && e.Error.Message != "" {
				msg = e.Error.Message
			}
			err = errors.New(msg)
		}
		return out, err
	}
	return out, nil
}

func (h *herdrBackend) do(args ...string) ([]byte, error) {
	if h.call != nil {
		return h.call(args...)
	}
	return herdrExec(args...)
}

func (h *herdrBackend) alive(id string) bool {
	_, err := h.do("pane", "get", id)
	return err == nil
}

func (h *herdrBackend) split(cwd string, env []string) (string, error) {
	args := []string{"pane", "split", "--current", "--direction", "down", "--ratio", herdrRatio, "--cwd", cwd, "--focus"}
	for _, e := range env {
		args = append(args, "--env", e)
	}
	out, err := h.do(args...)
	if err != nil {
		return "", err
	}
	var r struct {
		Result struct {
			Pane struct {
				ID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil || r.Result.Pane.ID == "" {
		return "", errors.New(i18n.T("herdr.badreply"))
	}
	return r.Result.Pane.ID, nil
}

func (h *herdrBackend) show(string) error {
	if _, err := h.do("pane", "zoom", "--current", "--off"); err != nil {
		return err
	}
	_, err := h.do("pane", "focus", "--current", "--direction", "down")
	return err
}

func (h *herdrBackend) hide() error {
	_, err := h.do("pane", "zoom", "--current", "--on")
	return err
}

func (h *herdrBackend) send(id, text string, enter bool) error {
	verb := "send-text"
	if enter {
		verb = "run"
	}
	_, err := h.do("pane", verb, id, text)
	return err
}

func (h *herdrBackend) tail(id string, n int) []string {
	out, err := h.do("pane", "read", id, "--source", "recent", "--lines", fmt.Sprint(n))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(out), "\n"), "\n")
}

func (h *herdrBackend) close(id string) { _, _ = h.do("pane", "close", id) }

func (*herdrBackend) shellEnv(sh, exe string) []string { return herdrShellIntegration(sh, exe) }

// ---- shell integration for the pane ----

var (
	herdrShellOnce sync.Once
	herdrShellDir  string
)

// The hooks of shellint.go print OSC 133;D, which herdr does not pass on to
// the editor. In the pane they run "shj command-end <code>" in the background
// instead.
const zshReportOSC = `print -n "\e]133;D;$s\a"`

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func herdrZshEnv(exe string) string {
	return strings.ReplaceAll(zshEnv, zshReportOSC, "{ "+shQuote(exe)+` command-end $s >/dev/null 2>&1 &! }`)
}

// herdrBashPrompt is the PROMPT_COMMAND of a bash in the pane. A herdr pane
// takes no arguments, so bash cannot be given an rcfile; the hook is
// handed over through the environment instead, and a startup file that
// replaces PROMPT_COMMAND (instead of extending it) drops it.
func herdrBashPrompt(exe string) string {
	return `__shijimi_s=$?;( ` + shQuote(exe) + ` command-end "$__shijimi_s" >/dev/null 2>&1 & );(exit $__shijimi_s)`
}

// herdrShellIntegration returns the environment that makes shell sh report
// its exit codes through exe (the shj that runs the editor). Unsupported
// shells get none.
func herdrShellIntegration(sh, exe string) []string {
	switch filepath.Base(sh) {
	case "zsh":
		herdrShellOnce.Do(func() {
			d, err := os.MkdirTemp("", "shijimi-herdr-shell-")
			if err != nil {
				return
			}
			if os.WriteFile(filepath.Join(d, ".zshenv"), []byte(herdrZshEnv(exe)), 0o600) != nil {
				os.RemoveAll(d)
				return
			}
			herdrShellDir = d
		})
		if herdrShellDir == "" {
			return nil
		}
		env := []string{"ZDOTDIR=" + herdrShellDir}
		if v, ok := os.LookupEnv("ZDOTDIR"); ok {
			env = append(env, "SHIJIMI_ZDOTDIR="+v)
		}
		return env
	case "bash":
		return []string{"PROMPT_COMMAND=" + herdrBashPrompt(exe)}
	}
	return nil
}

func removeHerdrShellIntegration() {
	if herdrShellDir != "" {
		os.RemoveAll(herdrShellDir)
	}
}
