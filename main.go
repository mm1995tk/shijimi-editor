// Command gh-editor is a VS Code–style IDE for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/ctl"
	"github.com/mm1995tk/gh-editor/internal/ui"
)

func main() {
	dir, file := ".", ""
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-h" || arg == "--help" {
			fmt.Println("usage: gh-editor [ディレクトリ | ファイル]")
			fmt.Println("       gh-editor open <path>[:行] | diff [path] | status")
			fmt.Println("       gh-editor skill install [書き出し先]")
			fmt.Println("サブコマンドは実行中の gh-editor へ $" + ctl.EnvSocket + " 経由で送ります。")
			fmt.Println("同名のファイルを開くときは ./status のようにパスで書いてください。")
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
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
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
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
