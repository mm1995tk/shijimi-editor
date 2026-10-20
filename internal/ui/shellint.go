package ui

import (
	"os"
	"path/filepath"
	"sync"
)

// Shell integration: the shells of the terminal panel report each command's
// exit code with OSC 133;D, so a failed command can be suggested for
// analysis (suggest.go). zsh and bash are set up; other shells start as
// before and can print OSC 133;D themselves.
//
// zsh: ZDOTDIR points at a directory whose .zshenv puts the user's ZDOTDIR
// back, reads their .zshenv and adds the hooks; zsh then reads the rest of
// the user's files as usual.
//
// bash: an rcfile reads what a login shell reads (/etc/profile, then the
// first of ~/.bash_profile, ~/.bash_login and ~/.profile, or ~/.bashrc when
// none exists) and puts the hook first in PROMPT_COMMAND.

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

const bashRC = `# shijimi: report each command's exit code to the terminal panel (OSC 133;D)
[ -f /etc/profile ] && . /etc/profile
if [ -f ~/.bash_profile ]; then . ~/.bash_profile
elif [ -f ~/.bash_login ]; then . ~/.bash_login
elif [ -f ~/.profile ]; then . ~/.profile
elif [ -f ~/.bashrc ]; then . ~/.bashrc
fi
# The command number (\# of PS1) moves when a command ran, repeated or not,
# and stays on an empty line, which then reports nothing. Bash before 4.4
# cannot expand it and reports on every prompt.
__shijimi_num='\#'
__shijimi_prompt() {
  local s=$? n=
  if (( BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 4) )); then
    n=${__shijimi_num@P}
  fi
  if [[ -z $n || $n != "${__shijimi_last:-$n}" ]]; then
    printf '\e]133;D;%s\a' "$s"
  fi
  __shijimi_last=$n
  return $s
}
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
  PROMPT_COMMAND=(__shijimi_prompt "${PROMPT_COMMAND[@]}")
else
  PROMPT_COMMAND="__shijimi_prompt${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
`

var (
	shellDirOnce sync.Once
	shellDir     string
)

// integrationDir writes the startup files once and returns their directory,
// or "" when that failed.
func integrationDir() string {
	shellDirOnce.Do(func() {
		d, err := os.MkdirTemp("", "shijimi-shell-")
		if err != nil {
			return
		}
		if os.WriteFile(filepath.Join(d, ".zshenv"), []byte(zshEnv), 0o600) != nil ||
			os.WriteFile(filepath.Join(d, "bashrc"), []byte(bashRC), 0o600) != nil {
			os.RemoveAll(d)
			return
		}
		shellDir = d
	})
	return shellDir
}

// shellIntegration returns the arguments and extra environment that start
// shell sh as a login shell reporting exit codes. Unsupported shells, or a
// failed setup, get a plain login shell.
func shellIntegration(sh string) (args, env []string) {
	plain := []string{"-l"}
	switch filepath.Base(sh) {
	case "zsh":
		d := integrationDir()
		if d == "" {
			return plain, nil
		}
		env = []string{"ZDOTDIR=" + d}
		if v, ok := os.LookupEnv("ZDOTDIR"); ok {
			env = append(env, "SHIJIMI_ZDOTDIR="+v)
		}
		return plain, env
	case "bash":
		d := integrationDir()
		if d == "" {
			return plain, nil
		}
		return []string{"--rcfile", filepath.Join(d, "bashrc"), "-i"}, nil
	}
	return plain, nil
}

// removeShellIntegration deletes the files shellIntegration wrote.
func removeShellIntegration() {
	if shellDir != "" {
		os.RemoveAll(shellDir)
	}
}
