package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mm1995tk/shijimi-editor/internal/channel"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/server"
	"github.com/mm1995tk/shijimi-editor/internal/ui"
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
		if f, err := server.OpenLog(path, 0); err == nil {
			defer f.Close()
			logw = f
		}
	}
	ver, commit := ui.BuildVersion()
	chName := "stable"
	if channel.IsNightly() {
		chName = channel.Nightly
	}
	srv, err := server.Listen(server.Options{
		SocketPath: sock,
		Shj:        server.Shj{Version: ver, Commit: commit, Channel: chName},
		Logger:     slog.New(slog.NewTextHandler(logw, nil)),
	})
	if err != nil {
		if errors.Is(err, server.ErrRunning) {
			fmt.Fprintf(os.Stderr, i18n.T("serve.running")+"\n", sock)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Serve(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
