// Command shj runs shijimi, a VS Code–style IDE for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/caret"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
	"github.com/mm1995tk/shijimi-editor/internal/kitty"
	"github.com/mm1995tk/shijimi-editor/internal/ui"
)

func main() {
	dir, file := ".", ""
	single := false
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-v" || arg == "--version" {
			fmt.Println("shijimi " + ui.VersionLabel())
			return
		}
		if arg == "-h" || arg == "--help" {
			i18n.SetLang(config.Load(dir).Editor.UILanguage)
			fmt.Println(i18n.T("help.usage"))
			fmt.Println(i18n.T("help.single1"))
			fmt.Println(i18n.T("help.single2"))
			fmt.Println(i18n.T("help.single3"))
			fmt.Println("       shj diff [path]")
			fmt.Println(i18n.T("help.exec"))
			fmt.Println(i18n.T("help.skill"))
			fmt.Println("       shj -v, --version")
			fmt.Println(i18n.T("help.sub1") + ctl.EnvSocket + i18n.T("help.sub2"))
			fmt.Println(i18n.T("help.samename"))
			return
		}
		if ctl.IsSubcommand(arg) {
			i18n.SetLang(config.Load(dir).Editor.UILanguage)
			os.Exit(ctl.Run(os.Args[1:],
				func(s string) { fmt.Println(s) },
				func(s string) { fmt.Fprintln(os.Stderr, s) }))
		}
		if st, err := os.Stat(arg); err == nil && st.IsDir() {
			dir = arg
		} else {
			file, single = arg, true
		}
	}
	var app *ui.App
	if single {
		var err error
		if app, err = ui.NewSingle(file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		app = ui.New(dir, file)
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
		if srv, err := ide.Start(abs, ui.IDETools(), ui.IDEHandler(p)); err == nil {
			closeIDE = srv.Close
			app.SetIDE(srv)
			os.Setenv(ide.EnvPort, strconv.Itoa(srv.Port()))
			os.Setenv(ide.EnvEnable, "true")
			os.Setenv(ide.EnvForceTerminal, "true")
		}
	}
	// The socket is best-effort: the editor still works without it.
	if path, err := ctl.SocketPath(); err == nil && !single {
		if srv, err := ctl.Listen(path, ui.ControlHandler(p)); err == nil {
			closeSock = srv.Close
			os.Setenv(ctl.EnvSocket, path)
		}
	}
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
