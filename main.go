// Command gh-editor is a VS Code–style IDE for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/ctl"
	"github.com/mm1995tk/gh-editor/internal/ide"
	"github.com/mm1995tk/gh-editor/internal/kitty"
	"github.com/mm1995tk/gh-editor/internal/ui"
)

func main() {
	dir, file := ".", ""
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-h" || arg == "--help" {
			fmt.Println("usage: gh-editor [ディレクトリ | ファイル]")
			fmt.Println("       gh-editor diff [path]")
			fmt.Println("       gh-editor skill install [書き出し先]")
			fmt.Println("サブコマンドは実行中の gh-editor へ $" + ctl.EnvSocket + " 経由で送ります。")
			fmt.Println("同名のファイルを開くときは ./diff のようにパスで書いてください。")
			return
		}
		if ctl.IsSubcommand(arg) {
			os.Exit(ctl.Run(os.Args[1:],
				func(s string) { fmt.Println(s) },
				func(s string) { fmt.Fprintln(os.Stderr, s) }))
		}
		if st, err := os.Stat(arg); err == nil && st.IsDir() {
			dir = arg
		} else {
			dir, file = filepath.Dir(arg), arg
		}
	}
	app := ui.New(dir, file)
	closeSock := func() {}
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	// On a terminal with Kitty graphics, screen updates and image data share one
	// writer so they are never interleaved.
	kittyOut := kitty.NewWriter(os.Stdout)
	graphics := kitty.Supported(os.Getenv)
	if graphics {
		opts = append(opts, tea.WithOutput(kittyOut))
	}
	p := tea.NewProgram(app, opts...)
	if graphics {
		app.EnableKitty(kittyOut, p.Send)
	}
	// The IDE integration is best-effort too: Claude Code started in the terminal
	// panel finds this server through the environment and the lock file.
	closeIDE := func() {}
	if abs, err := filepath.Abs(dir); err == nil {
		if srv, err := ide.Start(abs, ui.IDETools(), ui.IDEHandler(p)); err == nil {
			closeIDE = srv.Close
			app.SetIDE(srv)
			os.Setenv(ide.EnvPort, strconv.Itoa(srv.Port()))
			os.Setenv(ide.EnvEnable, "true")
			os.Setenv(ide.EnvForceTerminal, "true")
		}
	}
	// The socket is best-effort: the editor still works without it.
	if path, err := ctl.SocketPath(); err == nil {
		if srv, err := ctl.Listen(path, ui.ControlHandler(p)); err == nil {
			closeSock = srv.Close
			os.Setenv(ctl.EnvSocket, path)
		}
	}
	_, err := p.Run()
	app.Close()
	closeSock()
	closeIDE()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
