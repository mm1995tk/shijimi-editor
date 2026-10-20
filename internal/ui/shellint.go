package ui

import (
	"os"
	"path/filepath"
	"sync"
)

// Shell integration: the shells of the terminal panel report each command's
// exit code with OSC 133;D, so a failed command can be suggested for
// analysis (suggest.go). Only zsh is set up. Its ZDOTDIR points at a
// directory whose .zshenv puts the user's ZDOTDIR back, reads their
// .zshenv and adds the hooks; zsh then reads the rest of the user's files
// as usual.

const zshEnv = `# shijimi: report each command's exit code to the terminal panel (OSC 133;D)
if [[ -n ${SHIJIMI_ZDOTDIR+x} ]]; then
  ZDOTDIR=$SHIJIMI_ZDOTDIR
else
  unset ZDOTDIR
fi
unset SHIJIMI_ZDOTDIR
[[ -f ${ZDOTDIR:-$HOME}/.zshenv ]] && source ${ZDOTDIR:-$HOME}/.zshenv
if [[ -o interactive ]]; then
  typeset -g _shijimi_ran=0
  _shijimi_preexec() { _shijimi_ran=1 }
  _shijimi_precmd() {
    local s=$?
    (( _shijimi_ran )) && print -n "\e]133;D;$s\a"
    _shijimi_ran=0
  }
  typeset -ga preexec_functions precmd_functions
  preexec_functions+=(_shijimi_preexec)
  precmd_functions=(_shijimi_precmd $precmd_functions)
fi
`

var (
	zshDirOnce sync.Once
	zshDir     string
)

// shellIntegration returns the environment that makes shell sh report exit
// codes, or nil when sh is not supported or the setup failed.
func shellIntegration(sh string) []string {
	if filepath.Base(sh) != "zsh" {
		return nil
	}
	zshDirOnce.Do(func() {
		d, err := os.MkdirTemp("", "shijimi-zsh-")
		if err != nil {
			return
		}
		if err := os.WriteFile(filepath.Join(d, ".zshenv"), []byte(zshEnv), 0o600); err != nil {
			os.RemoveAll(d)
			return
		}
		zshDir = d
	})
	if zshDir == "" {
		return nil
	}
	env := []string{"ZDOTDIR=" + zshDir}
	if v, ok := os.LookupEnv("ZDOTDIR"); ok {
		env = append(env, "SHIJIMI_ZDOTDIR="+v)
	}
	return env
}

// removeShellIntegration deletes the files shellIntegration wrote.
func removeShellIntegration() {
	if zshDir != "" {
		os.RemoveAll(zshDir)
	}
}
