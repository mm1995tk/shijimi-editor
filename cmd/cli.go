// Package cmd is the entry point shared by the shj and shijimi commands.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/ui/cli"
	"github.com/mm1995tk/shijimi-editor/ui/cli/ctlclient"
	"github.com/mm1995tk/shijimi-editor/ui/tui"
	"github.com/mm1995tk/shijimi-editor/ui/tui/caret"
	"github.com/mm1995tk/shijimi-editor/ui/tui/kitty"
)

// Main runs shijimi with the command-line arguments in os.Args.
func Main() {
	dir, file := ".", ""
	single := false
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-v" || arg == "--version" {
			cli.Version()
			return
		}
		if arg == "-h" || arg == "--help" {
			setLangFromConfig(dir)
			cli.Help()
			return
		}
		if ctlclient.IsSubcommand(arg) {
			setLangFromConfig(dir)
			os.Exit(ctlclient.Run(os.Args[1:],
				func(s string) { fmt.Println(s) },
				func(s string) { fmt.Fprintln(os.Stderr, s) }))
		}
		if st, err := os.Stat(arg); err == nil && st.IsDir() {
			dir = arg
		} else {
			file, single = arg, true
		}
	}
	var app *tui.App
	if single {
		var err error
		if app, err = tui.NewSingle(file, coreBuild()); err != nil {
			failConfig(err)
		}
	} else {
		var err error
		if app, err = tui.New(dir, file, coreBuild()); err != nil {
			failConfig(err)
		}
	}
	closeSock := func() {}
	// The terminal cursor follows the typing position so that an IME shows
	// the text being composed there, but stays hidden while shijimi runs.
	caretOut := caret.NewWriter(os.Stdout)
	app.SetCaret(caretOut)
	app.SetCellSize(func() (int, int) { return kitty.CellSize(os.Stdout) })
	p := tea.NewProgram(app, tea.WithOutput(caretOut))
	// The IDE integration is best-effort too: Claude Code started in the terminal
	// panel finds this server through the environment and the lock file.
	closeIDE := func() {}
	// Single-file mode starts neither the IDE integration nor the control socket.
	if abs, err := filepath.Abs(dir); err == nil && !single {
		if srv, err := core.StartIDE(abs, core.IDEHandler(func(m any) { p.Send(m) })); err == nil {
			closeIDE = srv.Close
			app.SetIDE(srv)
			core.SetIDEEnv(srv.Port())
		}
	}
	// The socket is best-effort: the editor still works without it.
	if path, err := ctl.SocketPath(cli.Suffix()); err == nil && !single {
		if srv, err := core.ListenCtl(path, tui.ControlHandler(p), cli.Text); err == nil {
			closeSock = srv.Close
			os.Setenv(ctl.EnvSocket, path)
			// The terminals this shj starts belong to this shj, not to the GUI workspace it may run in.
			os.Unsetenv(ctl.EnvGUIWorkspace)
			// In the primary terminal of a GUI workspace, kisui serve is told.
			// The GUI shows the file tree then, so this shj leaves its own out.
			endPrimary, primary, reportFile, pane, perr := ctlclient.AnnouncePrimary(cli.Suffix(), path)
			app.SetTreeHidden(primary)
			// A shj that kisui-terminal refuses for its version works like one in a pane that is not the primary.
			var ve *ctlclient.VersionError
			if errors.As(perr, &ve) {
				app.Info("gui.protocol", ctl.PrimaryProtocol, ve.Min, ve.Max)
			}
			// The GUI tree lights the row of the file this shj shows.
			if primary {
				app.SetFileReporter(reportFile)
				app.SetGUIPane(pane.Do)
			}
			prev := closeSock
			closeSock = func() { endPrimary(); prev() }
		}
	}
	// Whatever happened above, nothing started from here is a primary.
	os.Unsetenv(ctl.EnvGUIPrimary)
	_, err := p.Run()
	_ = caretOut.ShowCursor()
	app.Close()
	closeSock()
	closeIDE()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// setLangFromConfig sets the UI language from dir's settings, for the commands
// that need nothing else from them. A mistake in the files is not reported
// here: the commands that run the editor report it, and the others work
// without settings.
func setLangFromConfig(dir string) {
	cfg, _ := config.Load(dir, configOptions())
	cli.SetLangFrom(cfg)
}

// failConfig reports why the editor could not start and exits. A mistake in a
// configuration file is listed with every file, line and reason.
func failConfig(err error) {
	fmt.Fprintln(os.Stderr, configErrorText(err))
	os.Exit(1)
}

// configErrorText is err for a terminal: a *config.Error gets a heading with
// its count.
func configErrorText(err error) string {
	if ce, ok := errors.AsType[*config.Error](err); ok {
		return fmt.Sprintf(cli.T("config.err.head"), len(ce.Problems)) + "\n" + cli.ErrText(ce)
	}
	return cli.ErrText(err)
}
