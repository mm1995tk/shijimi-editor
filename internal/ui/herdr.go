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

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
)

// Inside the herdr terminal multiplexer, the terminal commands (Ctrl+`, :sh,
// :run, :!, Space t, Space a) use a pane of herdr instead of the built-in
// terminal panel, so that herdr detects, notifies about and restores the
// agents run there. The pane is created below the editor's own pane, hidden
// by zooming the editor's pane, and closed when the editor quits. The
// built-in panel stays in use when [experimental] herdr is off or the editor
// does not run in herdr (HERDR_ENV=1).

const (
	envHerdr    = "HERDR_ENV"
	envHerdrBin = "HERDR_BIN_PATH"

	herdrRatio   = "0.3"
	herdrTimeout = 5 * time.Second

	// herdrFailTab is the termFailure.tab of failures seen in the herdr pane.
	herdrFailTab = -1
)

// herdrPane is the pane of herdr that stands in for the terminal panel.
type herdrPane struct {
	id      string // "" until a pane has been created
	visible bool
	// call runs a herdr command and returns its output; tests replace it.
	call func(args ...string) ([]byte, error)
}

// herdrOn reports whether the terminal commands should use a herdr pane.
func (a *App) herdrOn() bool {
	return !a.single && a.cfg != nil && a.cfg.Experimental.Herdr && os.Getenv(envHerdr) == "1"
}

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
			err = errors.New(msg)
		}
		return out, err
	}
	return out, nil
}

func (h *herdrPane) do(args ...string) ([]byte, error) {
	if h.call != nil {
		return h.call(args...)
	}
	return herdrExec(args...)
}

// alive reports whether the pane still exists.
func (h *herdrPane) alive() bool {
	if h.id == "" {
		return false
	}
	_, err := h.do("pane", "get", h.id)
	return err == nil
}

// paneEnv is the environment the pane's shell starts with: the IDE
// integration and the control socket of this editor (as set up in cli.Main),
// and the shell integration that reports exit codes through shj.
func (a *App) herdrPaneEnv() []string {
	var env []string
	for _, k := range []string{ctl.EnvSocket, ide.EnvPort, ide.EnvEnable, ide.EnvForceTerminal} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	if exe, err := os.Executable(); err == nil {
		env = append(env, herdrShellIntegration(shell(), exe)...)
	}
	return env
}

// create splits a new pane off the editor's pane and focuses it.
func (a *App) herdrCreate() error {
	h := &a.hd
	args := []string{"pane", "split", "--current", "--direction", "down", "--ratio", herdrRatio, "--cwd", a.root, "--focus"}
	for _, e := range a.herdrPaneEnv() {
		args = append(args, "--env", e)
	}
	out, err := h.do(args...)
	if err != nil {
		return err
	}
	var r struct {
		Result struct {
			Pane struct {
				ID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &r); err != nil || r.Result.Pane.ID == "" {
		return errors.New(i18n.T("herdr.badreply"))
	}
	h.id, h.visible = r.Result.Pane.ID, true
	return nil
}

// herdrOpen shows the pane and moves focus to it, creating it the first time
// and again after it has ended.
func (a *App) herdrOpen() error {
	h := &a.hd
	if !h.alive() {
		h.id, h.visible = "", false
		return a.herdrCreate()
	}
	if _, err := h.do("pane", "zoom", "--current", "--off"); err != nil {
		return err
	}
	h.visible = true
	_, err := h.do("pane", "focus", "--current", "--direction", "down")
	return err
}

// herdrHide hides the pane by zooming the editor's pane.
func (a *App) herdrHide() error {
	h := &a.hd
	if _, err := h.do("pane", "zoom", "--current", "--on"); err != nil {
		return err
	}
	h.visible = false
	return nil
}

// herdrToggle is the toggle of Ctrl+`, Space t and :sh.
func (a *App) herdrToggle() tea.Cmd {
	h := &a.hd
	var err error
	if h.visible && h.alive() {
		err = a.herdrHide()
	} else {
		err = a.herdrOpen()
	}
	return a.herdrResult(err)
}

// herdrSend opens the pane and sends text to it: a command line that is run
// (Enter included), or pasted text that is left for the person to confirm.
func (a *App) herdrSend(text string, enter bool) tea.Cmd {
	if err := a.herdrOpen(); err != nil {
		return a.herdrResult(err)
	}
	verb := "send-text"
	if enter {
		verb = "run"
	}
	_, err := a.hd.do("pane", verb, a.hd.id, "--", text)
	return a.herdrResult(err)
}

// herdrClose closes the pane that this editor created, if it is still there.
func (a *App) herdrClose() {
	h := &a.hd
	if h.id != "" && h.alive() {
		_, _ = h.do("pane", "close", h.id)
	}
	h.id, h.visible = "", false
}

func (a *App) herdrResult(err error) tea.Cmd {
	if err != nil {
		a.fail(fmt.Errorf(i18n.T("herdr.failed"), err))
	}
	return nil
}

// commandEnded handles the exit code that the shell in the herdr pane
// reports with "shj command-end <code>" (see herdrShellIntegration). It
// plays the part of scanTerminals for the built-in terminal.
func (a *App) commandEnded(code int) {
	switch code {
	case 0:
		a.clearFailure(herdrFailTab)
	case 130:
		// Ctrl+C: the user stopped it on purpose.
	default:
		var tail []string
		if a.hd.id != "" {
			if out, err := a.hd.do("pane", "read", a.hd.id, "--lines", fmt.Sprint(failTail)); err == nil {
				tail = strings.Split(strings.TrimRight(string(out), "\n"), "\n")
			}
		}
		a.addFailureTab(herdrFailTab, 1, code, "", tail)
	}
}

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
