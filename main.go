// Command gh-editor is a VS Code–style IDE for the terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/ui"
)

func main() {
	dir, file := ".", ""
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-h" || arg == "--help" {
			fmt.Println("usage: gh-editor [ディレクトリ | ファイル]")
			return
		}
		if st, err := os.Stat(arg); err == nil && st.IsDir() {
			dir = arg
		} else {
			dir, file = filepath.Dir(arg), arg
		}
	}
	p := tea.NewProgram(ui.New(dir, file), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
