package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/config"
)

// The examples the manual and the README show must keep loading, with the
// settings of every part (core, UI shared, TUI, GUI) read from one file.
func TestDocumentedExamplesLoad(t *testing.T) {
	root := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	userDir := filepath.Join(xdg, "shijimi-editor")
	os.MkdirAll(userDir, 0o755)
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(userDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.toml", `
theme = "github_dark"
keymap = "helix"
ui-language = "en"

[editor]
line-number = "relative"
file-icons = true
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"
watch-files = true
mouse-hover = true

[editor.indent-guides]
render = true

[editor.auto-save.after-delay]
enable = true
timeout = 1000

[search]
exclude = ["*.lock", "dist/**"]
include = []

[browser]
listen-host = "127.0.0.1"

[terminal]
multiplexer = "auto"

[experimental]
multiplexer = true
herdr = true

[agent]
command = "claude-agent-acp"
args = []
environment = { A = "1" }

[server]
idle_stop_days = 7

[keys.normal]
"C-s" = ":write"
`)
	write("languages.toml", `
[language-server.my-gopls]
command = "gopls"
args = ["serve"]
config = { gofumpt = true }
environment = { GOFLAGS = "-tags=integration" }

[[language]]
name = "go"
language-servers = ["my-gopls"]
roots = ["go.mod"]
indent = { tab-width = 4, unit = "\t" }
formatter = { command = "gofumpt", args = ["--path", "%{buffer_name}"] }

[[language]]
name = "mylang"
file-types = ["ml2", { glob = "*.special" }]
language-servers = [{ name = "my-gopls", except-features = ["format"] }]

[language.indent]
unit = "  "
`)
	if _, err := config.Load(root, configOptions()); err != nil {
		t.Fatal(err)
	}
}
