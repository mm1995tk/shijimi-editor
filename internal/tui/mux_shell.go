package tui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// Shell integration of the multiplexer panes: the shell reports its exit codes
// through "shj command-end <code>".

var (
	muxShellOnce sync.Once
	muxShellDir  string
)

// The hooks of shellint.go print OSC 133;D, which a multiplexer does not pass on to
// the editor. In the pane they run "shj command-end <code>" in the background
// instead.
const zshReportOSC = `print -n "\e]133;D;$s\a"`

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func muxZshEnv(exe string) string {
	return strings.ReplaceAll(core.ZshEnv, zshReportOSC, "{ "+shQuote(exe)+` command-end $s >/dev/null 2>&1 &! }`)
}

// muxBashPrompt is the PROMPT_COMMAND of a bash in the pane. A multiplexer pane
// takes no arguments, so bash cannot be given an rcfile; the hook is
// handed over through the environment instead, and a startup file that
// replaces PROMPT_COMMAND (instead of extending it) drops it.
func muxBashPrompt(exe string) string {
	return `__shijimi_s=$?;( ` + shQuote(exe) + ` command-end "$__shijimi_s" >/dev/null 2>&1 & );(exit $__shijimi_s)`
}

// muxShellIntegration returns the environment that makes shell sh report
// its exit codes through exe (the shj that runs the editor). Unsupported
// shells get none.
func muxShellIntegration(sh, exe string) []string {
	switch filepath.Base(sh) {
	case "zsh":
		muxShellOnce.Do(func() {
			d, err := os.MkdirTemp("", "shijimi-mux-shell-")
			if err != nil {
				return
			}
			if os.WriteFile(filepath.Join(d, ".zshenv"), []byte(muxZshEnv(exe)), 0o600) != nil {
				os.RemoveAll(d)
				return
			}
			muxShellDir = d
		})
		if muxShellDir == "" {
			return nil
		}
		env := []string{"ZDOTDIR=" + muxShellDir}
		if v, ok := os.LookupEnv("ZDOTDIR"); ok {
			env = append(env, "SHIJIMI_ZDOTDIR="+v)
		}
		return env
	case "bash":
		return []string{"PROMPT_COMMAND=" + muxBashPrompt(exe)}
	}
	return nil
}

func removeMuxShellIntegration() {
	if muxShellDir != "" {
		os.RemoveAll(muxShellDir)
	}
}
