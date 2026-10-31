package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/mm1995tk/shijimi-editor/internal/attach"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// runAttach is "shj attach": the bridge between stdin/stdout and the socket of
// "shj serve", which it starts (detached) when it is not running. It returns
// the process exit code.
func runAttach(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, i18n.T("attach.usage"))
		return 2
	}
	sock, err := ctl.ServeSocketPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	lock, err := ctl.ServeLockPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	err = attach.Run(attach.Options{
		SocketPath: sock,
		LockPath:   lock,
		Start:      startServe,
		In:         os.Stdin,
		Out:        os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// startServe runs "shj serve" from this same executable, in its own session,
// with stdin/stdout/stderr closed (/dev/null). serve writes its own log file.
func startServe() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd := exec.Command(exe, "serve")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
