package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/channel"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
	"github.com/mm1995tk/shijimi-editor/internal/server"
)

// runServe is "shj serve": the headless server that GUI clients talk to over
// a Unix socket. It returns the process exit code.
func runServe(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, i18n.T("serve.usage"))
		return 2
	}
	sock, err := ctl.ServeSocketPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// The log goes to a file because the server runs detached from a terminal.
	var logw io.Writer = io.Discard
	if path, err := server.LogPath(); err == nil {
		f, err := server.OpenLog(path, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			defer f.Close()
			logw = f
		}
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	ver, commit := core.BuildVersion()
	chName := "stable"
	if channel.IsNightly() {
		chName = channel.Nightly
	}
	logger := slog.New(slog.NewTextHandler(logw, nil))
	var idle time.Duration
	if home, err := os.UserHomeDir(); err == nil {
		cfg := config.Load(home)
		idle = time.Duration(cfg.Serve.IdleStopDays) * 24 * time.Hour
		// The UI language is the process's: the home config sets it once, and
		// the workspaces' cores leave it alone (core.NewIn).
		i18n.SetLang(cfg.Editor.UILanguage)
	}
	srv, err := server.Listen(server.Options{
		SocketPath: sock,
		Shj:        server.Shj{Version: ver, Commit: commit, Channel: chName},
		Logger:     logger,
		IdleStop:   idle,
	})
	if err != nil {
		if errors.Is(err, server.ErrRunning) {
			fmt.Fprintf(os.Stderr, i18n.T("serve.running")+"\n", sock)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	// A core per workspace, on one set of process-wide parts. A core is made
	// when its workspace takes the focus and kept while it has files open.
	shared := core.NewShared()
	defer shared.Close()
	cores := core.NewWorkspaces(shared, nil)
	defer cores.CloseAll()
	srv.SetCores(serveCores{cores})
	srv.SetScreens(serveCores{cores})
	srv.AddBeforeStop(func() { writeBackups(cores, logger) })
	// Like the TUI, the control socket and the Claude Code connection are best
	// effort. The server has one of each for all workspaces; requests go to the
	// focused workspace. The socket is <pid>.sock as in the TUI, so shj finds it
	// by the same rules; the environment is set before any terminal is created,
	// so shells in the server's terminals reach this server directly.
	if path, err := ctl.SocketPath(); err == nil {
		if err := srv.StartControl(path); err == nil {
			os.Setenv(ctl.EnvSocket, path)
		} else {
			logger.Warn("control socket", "err", err)
		}
	}
	// The lock file names the home directory as the workspace folder: Claude
	// Code connects when its directory is inside one, and the server does not
	// know the workspaces' roots when it starts.
	if home, err := os.UserHomeDir(); err == nil {
		if is, err := srv.StartIDE(home, core.IDETools()); err == nil {
			os.Setenv(ide.EnvPort, strconv.Itoa(is.Port()))
			os.Setenv(ide.EnvEnable, "true")
			os.Setenv(ide.EnvForceTerminal, "true")
		} else {
			logger.Warn("ide", "err", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Serve(ctx); err != nil {
		logger.Error("serve", "err", err)
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
