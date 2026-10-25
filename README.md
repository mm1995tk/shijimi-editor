# <img src="manual/docs/assets/icon.png" alt="" width="32" align="top"> shijimi

English | [日本語](README.ja.md)

An integrated development environment for the terminal (TUI). The screen is laid out like VS Code. For editing you can choose between Helix-style modal editing and modeless VS Code-style keys. The command is `shj`.

Manual (English, with a Japanese version): <https://shijimi-manual.pages.dev/> (source in [`manual/`](manual/))

The UI is in English by default. To switch it to Japanese, set `ui-language = "ja"` in `config.toml` (see [UI language](#ui-language)).

## Features

- **Modal editing and VS Code-style keys**: choose Helix-style modal editing (normal / insert / select, `:` commands) or modeless VS Code-style keys
- **Multiple cursors** (Helix-style): `C` / `Alt+C` copy the cursor to the line below / above, `s` splits a selection by a regex, `,` goes back to one cursor. Inserting, deleting, `c` and so on apply to every cursor
- **File tree** (shows Git status as M / U / A)
- **LSP**: diagnostics, completion, hover, signature help, go to definition, references, rename, code actions, symbol list, formatting
- **LSP configuration** is written in a `languages.toml` that is compatible with Helix's (`.shijimi-editor/languages.toml` or `~/.config/shijimi-editor/languages.toml`). If you are coming from Helix, copy your `languages.toml` to `~/.config/shijimi-editor/`
- **Diff mode** (`Space G` / `Ctrl+G` compares with `HEAD`; `Space g` / `:pr` compares with the base of a pull request): switches the whole editor to diff mode. Changed files are listed in a tree (CHANGES), and a changed file opens as the **whole file** in a split (or unified) diff. The right side is editable. The bar on the right edge shows where the changes are, and the parts that changed inside a line are highlighted (see below)
- **Image preview** (PNG / JPEG / GIF / BMP / WebP / TIFF)
- **Markdown preview** (`Space m` / `:preview`): renders headings, lists, code and tables, and shows them beside the buffer you are editing. Press it again to show only the preview at full width. Mermaid code blocks are shown as diagrams (see below)
- **Send the current context to an agent** (`Space a` / `Alt+A` / command palette): formats the selection (or the cursor line) as `path:line-line`, the text, and the diagnostics on those lines, and after you check it in a preview, pastes it into the terminal panel (it does not press Enter)
- **Suggestions panel and ACP** (`Alt+I` / `Space i`): lists suggestions such as "Resolve the conflicts?", "Create a PR?" and "Investigate the cause of the failure?" in the right panel, based on the state of git, LSP and the terminal. When you pick one, it asks an agent over ACP (Agent Client Protocol), shows the progress and the reply in the panel, and shows changes in diff mode. Comments proposed by an AI review are placed as unsent comments
- **Claude Code IDE integration**: Claude Code running in the terminal panel can get the selection, the open files and the diagnostics, and you approve its changes as a diff (Claude Code writes the file after you approve; see "Agent integration" below)
- Tabs, syntax highlighting, Quick Open, project-wide search, a terminal panel, mouse support

## Installation

```sh
nix run github:mm1995tk/shijimi-editor/release   # run it right away
nix profile install github:mm1995tk/shijimi-editor/release
go install github.com/mm1995tk/shijimi-editor/cmd/shj@latest      # installs the shj command
go install github.com/mm1995tk/shijimi-editor/cmd/shijimi@latest  # installs the same thing as shijimi
```

Both get the latest release: the Nix commands follow the `release` branch, and `go install` takes the newest tag. Unreleased changes on `main` are not included.

To compare with the base of a pull request you need [gh](https://cli.github.com/) and `gh auth login`.

## Usage

```sh
shj            # open the current directory
shj ./src      # open a directory
shj main.go    # edit just one file (single-file mode, for $EDITOR)
```

`shijimi` works the same as `shj` (installing with Nix gives you both).

Given a file, shj starts in single-file mode. It does not start the file tree, LSP, terminal, diff mode or agent, and exits when you close it with `:q` or `:wq` (see [Installation and startup](manual/docs/install.en.md) in the manual).

## Settings

Files are read in the following order, and later ones win (`languages.toml` is compatible with Helix's. If you are coming from Helix, copy your `languages.toml` to `~/.config/shijimi-editor/`).

1. Built-in defaults (gopls / rust-analyzer / typescript-language-server and others)
2. `~/.config/shijimi-editor/languages.toml`, `~/.config/shijimi-editor/config.toml`
3. The project's `.shijimi-editor/languages.toml`, `.shijimi-editor/config.toml`

### UI language

The language of the UI text (status line, command palette, suggestions panel, error messages, `shj --help`) is English by default. Set `ui-language` in `config.toml` to switch it.

```toml
ui-language = "ja"   # "en" (default) or "ja"
```

shijimi does not choose the language from `LANG` or other environment variables. Any value other than `en` and `ja` is ignored. `:config-reload` applies a change without restarting.

### Theme

Switch with `theme` in `config.toml`, or with `:theme <name>` while running (without a name it lists the themes). You can also choose it from the command palette ("Settings: Switch theme").

- Built in: `dark-plus` (default), `github-dark`, `light-plus`
- Helix's `github_dark*` maps to `github-dark`, and `github_light*` and `light` map to `light-plus`

```toml
theme = "github_dark"
```

### Listen address of port forwarding

Sets the address that `:browser forward` listens on.

```toml
[browser]
listen-host = "127.0.0.1"
```

### Agent (suggestions panel)

Write the command of the ACP agent that runs the suggestions in `~/.config/shijimi-editor/config.toml` (it is not read from a project's settings).

```toml
[agent]
command = "claude-agent-acp"
args = []
```

### Example LSP settings (`.shijimi-editor/languages.toml`)

```toml
[language-server.my-gopls]
command = "gopls"
args = ["serve"]
config = { gofumpt = true }          # sent as initializationOptions
environment = { GOFLAGS = "-tags=integration" }

[[language]]
name = "go"
language-servers = ["my-gopls"]
roots = ["go.mod"]
indent = { tab-width = 4, unit = "\t" }
```

Supported keys: `command`, `args`, `config` and `environment` of `[language-server.*]`; `name`, `language-id`, `file-types`, `roots`, `language-servers` and `indent.unit` of `[[language]]`.

### Editor settings (`config.toml`)

```toml
keymap = "helix"          # "vscode" for VS Code-style keys

[editor]
line-number = "relative"  # how line numbers are shown
file-icons = true         # Nerd Font icons in the file tree; false shows ▸/▾
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"  # external command that turns mermaid into a PNG; empty disables it
watch-files = true        # detect and reflect file changes by other processes (such as AI agents). A file with unsaved edits waits for :reload
mouse-hover = true        # show the LSP hover when the pointer rests on a word in the editor for 0.5 seconds. false disables it (and stops receiving motion-only mouse events)

# Indent guides (a faint vertical line at each indent level). false disables them
[editor.indent-guides]
render = true

# Auto save (on by default; saves when editing pauses for 1 second; unnamed files are not saved)
[editor.auto-save.after-delay]
enable = true
timeout = 1000            # milliseconds
```

### Writing settings in Nix

If you put `config.nix` / `languages.nix` next to the `config.toml` / `languages.toml` of a directory, it is evaluated with `nix eval --json --file` and read as the same structure as TOML (in the same directory it is applied after the TOML file). A file is ignored if `nix` is missing or the evaluation fails.

```nix
# ~/.config/shijimi-editor/languages.nix
{
  language-server.nil.command = "nil";
  language = [
    { name = "nix"; language-servers = [ "nil" ]; }
  ];
}
```

### home-manager

Import the flake's `homeManagerModules.default`. `settings` is written to `~/.config/shijimi-editor/` as `config.toml`, and `languages` as `languages.toml`.

```nix
{
  # inputs.shijimi.url = "github:mm1995tk/shijimi-editor/release";
  imports = [ inputs.shijimi.homeManagerModules.default ];
  programs.shijimi-editor = {
    enable = true;
    settings = { keymap = "vscode"; editor.line-number = "relative"; };
    languages = {
      language-server.gopls.command = "gopls";
      language = [ { name = "go"; language-servers = [ "gopls" ]; } ];
    };
  };
}
```

### Markdown preview and mermaid

In a `.md` file, `Space m` (or `:preview`, or the command palette) opens a preview to the right of the buffer. It follows your edits and scrolls with the cursor. Press it again to show only the preview at full width, and a third time to close it. At full width, scroll with `j` / `k` or the mouse wheel.

Mermaid diagrams are turned into PNGs with [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) (`mmdc`) and shown. If `mmdc` is not on the PATH, or rendering fails, the source and the error are shown instead of the diagram. `mermaid-command` replaces it with another command (`{in}` is the input, `{out}` the output PNG). Rendered images are kept in `~/.cache/shijimi/mermaid`.

## Keys (Helix mode)

### Normal mode

| Key | Action |
| --- | --- |
| `h j k l` | Move (a number before it gives a count) |
| `w` `b` `e` | Move while selecting a word |
| `x` | Select a line (press again to extend) |
| `%` / `;` | Select all / collapse the selection |
| `v` | Select mode |
| `d` `c` `y` | Delete / change / yank |
| `p` `P` | Paste after / before |
| `i` `a` `I` `A` `o` `O` | Go to insert mode |
| `u` `U` | Undo / redo |
| `f` `t` `F` `T` + a character | Find a character in the line |
| `r` + a character | Replace |
| `>` `<` | Indent |
| `J` | Join lines |
| `mm` | Go to the matching bracket |
| `gg` `ge` `gh` `gl` `gs` / `<number>g` | Jump |
| `gd` `gr` | Go to definition / references |
| `Ctrl+O` `Ctrl+I` | Go back / forward to where you jumped from (VS Code mode: `Alt+←` `Alt+→`) |
| `]d` `[d` | Next / previous diagnostic |
| `gn` `gp` | Next / previous tab |
| `/` `?` `n` `N` `*` | Search |
| `Ctrl+C` | Toggle a line comment |
| `:` | Commands (table below) |

### Space menu

When you press `Space`, a popup at the bottom right of the editor area lists the keys you can press next. Close it with `Esc` or any key with no assignment.

| Key | Action |
| --- | --- |
| `Space f` | Open a file |
| `Space b` | Open buffers |
| `Space e` | Go to the file tree |
| `Space k` | Hover. While it is shown, scroll with `Ctrl+D` / `Ctrl+U` (`PgDn` / `PgUp`, wheel); copy the Markdown source with `y` or by clicking `[Copy]` at the top right of the frame |
| `Space o` | Open the URL under the cursor in the external browser (Linux: `xdg-open` / macOS: `open`) |
| `Space r` | Rename |
| `Space .` | Code actions (apply a diagnostic hint) |
| `Space s` / `Space S` | Symbol list (in the file / in the whole project) |
| `Space d` | Diagnostics list |
| `Space /` | Search and replace in the whole project (floating window; see "Search in files") |
| `Space g` | Open / close diff mode (compared with the base of the pull request) |
| `Space m` | Toggle the Markdown preview (side by side → full width → closed) |
| `Space G` | Open / close diff mode (compared with `HEAD`) |
| `Space v` | Review menu (when comparing with the base of a pull request) |
| `Space y` `Space p` | Copy to / paste from the system clipboard |
| `Space ?` | Command palette |

### Insert mode

`Esc` goes back to normal mode. Completion appears automatically when you type `.` or similar, or after 2 characters (`Ctrl+X` to trigger it by hand). Confirm with `Tab` / `Enter`. A snippet completion is inserted with its placeholder selected, and `Tab` / `Shift+Tab` move between placeholders.

### `:` commands

`:w` `:w <path>` `:wa` `:q` `:q!` `:wq` `:wqa` `:o <path>` `:n` `:bc` `:bc!` `:fmt` `:reload` `:<line number>` `:sh <command>` `:grep [-x exclude-glob] [-i include-glob] <text>` `:replace [-x exclude-glob] [-i include-glob] <text> => <replacement>` `:pr [number]` `:gitdiff [on|off|split|unified]` `:preview` `:lsp-restart` `:config-reload` `:browser forward|list|stop` `:keymap [helix|vscode]` `:open-url [URL]` (without an argument, the URL under the cursor)

### Global

| Key | Action |
| --- | --- |
| `Ctrl+P` | Open a file |
| `F1` / `Ctrl+K` | Command palette |
| `Ctrl+B` | Toggle the file tree |
| `Ctrl+E` | Switch focus between the file tree and the editor |
| `Ctrl+G` | Open / close diff mode (compared with `HEAD`) |
| ``Ctrl+` `` / `Ctrl+J` | Toggle the terminal (inside the terminal, only ``Ctrl+` ``) |
| `Alt+R` | Pane resize mode (`←→` sidebar width, `↑↓` terminal height, `<` `>` boundary between the editor and the preview, `=` defaults, `Esc` to finish). You can also drag the borders with the mouse. The sizes are kept until you quit the editor |
| `Ctrl+S` | Save |
| `Ctrl+W` | Close the tab |
| `Ctrl+PgUp` / `Ctrl+PgDn` / `Alt+1`-`9` | Switch tabs |
| `Ctrl+Q` | Quit |

### File tree

`j/k` move, `Enter` open, `h/l` collapse / expand, `a` new file, `A` new folder, `r` rename, `d` delete, `y` copy the relative path, `Y` copy the absolute path, `R` refresh, `Esc` back to the editor.

### Diff mode

How you open it decides what you compare with (the base). Pressing the same key while it is open closes it, and pressing a different key switches the base.

| Key | Base |
| --- | --- |
| `Space G` / `Ctrl+G` / `:gitdiff` | `HEAD` (includes untracked files) |
| `Space g` / `:pr [number]` | The base of the pull request (`git merge-base <PR base> HEAD`). Opens the PR of the current branch. For a PR on another branch, run `gh pr checkout` first (needs `gh`) |

- From the top, the screen is the header (the base and what it is compared with), the sidebar and the diff, the help line (the keys you can use now), and the status line.
- The sidebar is split 7:3 vertically, and **CHANGES** (a tree of the changed files) appears in the lower 3. A folder with a single child is collapsed like `internal/config`. `Tab` moves between it and the file tree.
- A file without changes opens in the normal editor. A changed file opens as the whole file in **Split** (the base on the left, the current file on the right). A new file (including an untracked one) opens in a single editor. Lines are not colored, the bar on the right edge turns all green, and the `+N` / `-N` in the file name row at the top remain.
- The right side (the current file) can be edited and saved as usual. The diff updates right away when you edit.
- You can also switch to **Unified** (`s`, `:gitdiff unified` / `split`, or a click on the file name row).
- The bar on the right edge of the diff shows where the changes are by color (green: added, red: deleted, blue: modified). The range now shown is brighter, and clicking scrolls to that position.
- In a line where only a part changed, the changed part is highlighted in a darker color.
- While comparing with the base of a pull request, you can review with `Space v` (comments on lines, ranges and files, replies, resolving, applying suggestions, viewed, and submitting an approval and so on). You can comment only on files whose working tree content is the same as the PR's latest commit. See "Diff mode" in the manual for details.

| Key (with CHANGES focused) | Action |
| --- | --- |
| `j/k` | Move |
| `Enter` / `l` | Open the file |
| `s` | Toggle split / unified |
| `r` | Refresh the list |
| `Tab` | Go to the file tree |
| `Esc` | Go to the editor |

## Limitations

- The terminal panel is a login shell (`$SHELL -l`) on a PTY. `.zshrc` and the like are read, and `vim` / `top` / `lazygit` work. While it has focus, almost every key goes to the shell (the global ones are only `Ctrl+Q` / ``Ctrl+` `` / `F1` / `Alt+R`). `Ctrl+J` goes to the shell, so you can enter a newline in Claude Code even on a terminal that sends Shift+Enter as LF. `F2` opens a new tab, `Ctrl+PageUp/PageDown` switches tabs, and the tab closes when the shell exits. You can scroll back through the output with the mouse wheel.
- Images and mermaid diagrams are drawn with the Kitty graphics protocol. You need a terminal that supports it (Kitty / Ghostty / WezTerm and others).

## Development

Use the Nix devShell (Go, gopls, gomod2nix, git). With direnv, `.envrc` enters it automatically.

```sh
nix develop            # or: direnv allow
go test ./...
go build ./cmd/shj
nix build              # build the package (also runs the tests)
nix fmt                # format with nixfmt / gofmt / taplo
nix flake check        # check the formatting
```

After changing `go.mod`, run `gomod2nix` to update `gomod2nix.toml`.

User-visible strings live in one table per language, [`internal/i18n/en.go`](internal/i18n/en.go) and [`internal/i18n/ja.go`](internal/i18n/ja.go). Code asks for a string with `i18n.T("key")` and does not contain UI text. When you add a string, add the key to both tables (a test checks that they match).

### Search in files

`Space /`, "Search: Search / replace in the project" in the command palette, `:grep` and `:replace` all open the same floating window (almost the full screen, leaving one column or row on each side). At the top are the conditions (four fields, Search, Replace, Include and Exclude, in bordered boxes in two columns, where the border color of the selected field changes; the toggles are below them), the matched lines (Matched) are at the lower left, and the lines around the selected one (View) at the lower right. When you change a condition, it searches again automatically after a short wait. When the Replace field has text, View also shows the `-` / `+` diff.

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Next / previous field |
| `↑` `↓` / `Ctrl+P` `Ctrl+N` / `PgUp` `PgDn` | Select a matched line |
| `Enter` | Open the selected line in its file |
| `Ctrl+X` (`Alt+X`) | Exclude the selected line from the replacement / put it back (an excluded line is marked `x`) |
| `Alt+C` / `Alt+W` / `Alt+R` / `Alt+P` | Match case / whole word / regex / preserve case when replacing |
| `Alt+1` | Replace only the selected line (the panel stays open and moves to the next match) |
| `Alt+Enter` | Replace every target line |
| `Esc` | Close |

- Replace All opens the target files as buffers and replaces in them. The disk does not change until you save, and `u` / `Ctrl+Z` undoes it.
- When regex is on, you can use captures such as `$1` in the Replace field. When it is off, `$` is just a character.
- When there are more than 5000 results, Replace All is unavailable until you narrow the conditions.
- The search covers text files under Git (including untracked files that are not ignored). Outside Git it searches everything except build directories. It also searches the unsaved content of open buffers.
- The arguments of `:grep -x '*.lock,dist/**' -i '*.go' foo` and `:replace foo => bar` open the window with those values in the fields. Typing just `:grep` / `:replace` opens it with the previous conditions.

### Excluding and including files in search

In the Include and Exclude fields, write comma-separated globs (for example `*.lock,dist/**`). With `:grep` / `:replace` you can also give them with `-x` / `--exclude=` (exclude) and `-i` / `--include=` (include). The initial values come from the `[search]` setting. Put anything you always want applied in `~/.config/shijimi-editor/config.toml` or `.shijimi-editor/config.toml`.

```toml
[search]
exclude = ["*.lock", "dist/**"]
include = []
```

### Port forwarding (`:browser forward`)

`:browser forward <target port> [listen port]` makes shijimi listen on `127.0.0.1` and forward over TCP to `localhost:<target port>` (both HTTP and WebSocket work). If you omit the listen port, a free port is chosen. `:browser list` lists the forwards, and `:browser stop <listen port>` stops one. Quitting shijimi stops them all.

When shijimi runs on a remote VM, running the following on your own machine lets your local browser see the development server at `http://localhost:8080`.

```sh
ssh -L 8080:127.0.0.1:<listen port> vm
```

Compared with running `ssh -L` directly, the advantages are that the listen port can be fixed, and that a server that listens only on `0.0.0.0` or IPv6 can still be reached through `127.0.0.1`. If you do not need those, use `ssh -L` directly.

The listen address can be changed with `listen-host` in `[browser]` (see "Listen address of port forwarding" under Settings). By default it does not bind to `0.0.0.0`. Change it only on a trusted network.

## Agent integration

### Claude Code IDE integration

shijimi supports Claude Code's IDE integration protocol (the same one as the VS Code extension and claudecode.nvim). On startup it starts a WebSocket MCP server on a free port of 127.0.0.1, and writes the destination and an authentication token to the lock file `~/.claude/ide/<port>.lock` (0600; under `ide/` of `CLAUDE_CONFIG_DIR` if it is set). It removes the file on exit.

When you start `claude` in shijimi's terminal panel, it connects automatically through the environment variables `CLAUDE_CODE_SSE_PORT`, `ENABLE_IDE_INTEGRATION=true` and `FORCE_CODE_TERMINAL=true`. A Claude Code running in another terminal can be chosen with `/ide` if its working directory is under shijimi's root.

Tools the agent can use:

| Tool | What it does |
|------|------|
| `getCurrentSelection` / `getLatestSelection` | The selection (or the cursor position) and its file |
| `getOpenEditors` / `getWorkspaceFolders` | The open files / the roots |
| `getDiagnostics` | LSP diagnostics (all files if the URI is omitted) |
| `openFile` | Open a file (select a range with `startText` / `endText`) |
| `openDiff` | Show a change as a diff. **Enter approves and Esc rejects.** Claude Code writes the file after you approve |
| `closeAllDiffTabs` / `close_tab` | Reject and close the diffs that are waiting |
| `checkDocumentDirty` / `saveDocument` | Check for unsaved changes / save |

- Whenever the selection or the cursor moves, it sends `selection_changed`.
- While connected, `Space a` / `Alt+A` (send the current context to the agent) sends `at_mentioned` instead of pasting. When not connected, it shows the preview and pastes into the terminal as before.
- Connections without an authentication token, and connections from a browser (with an `Origin` header), are refused.

### Control socket

A running shijimi creates a Unix socket (0600) and tells its location through the environment variable `SHIJIMI_SOCKET`. From a shell in the terminal panel you can run the following. Only operations that the IDE integration lacks are kept.

- `shj diff [path]`: switch to diff mode (base `HEAD`). With `path`, opens that file as a diff
- `shj exec ":<command>"`: run a `:` command as it is (for example `shj exec ":preview"`). On success it writes the status-line text to standard output; on failure it writes the error text to standard error and exits with code 1. It returns an error without running commands that quit or discard (`:q` `:q!` `:wq` `:bc!` and so on), commands that open a picker when given no argument (`:o` `:theme` `:grep` `:replace` `:symbol` `:action`, and `:pr` with a non-numeric argument), and `:reload` and `:bc` on a file with unsaved changes

When the first argument is `diff` / `exec` / `skill`, it is treated as a subcommand. Write a file with the same name as `./diff`.

`shj skill install [destination]` writes a skill (`SKILL.md`) that describes how to use this command. Without a destination it writes to `~/.claude/skills/shijimi/`.

> `shj open` and `shj status` were removed and replaced by the IDE integration (`openFile` / `getOpenEditors` / `getCurrentSelection`).

## Editing the manual

The manual is built from `manual/` with [MkDocs Material](https://squidfunk.github.io/mkdocs-material/). When you push to `main`, GitHub Actions publishes it to Cloudflare Pages (the repository Secrets need `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`). To preview it locally, run:

```sh
pip install -r manual/requirements.txt
mkdocs serve --config-file manual/mkdocs.yml
```

The manual has an English page (`name.en.md`, the default) and a Japanese page (`name.ja.md`) for each page. When you change one, change the other too. `manual/check-translations.sh` checks that every page has both languages and the same structure, and CI runs it on pull requests (it also fails when only one language of a page changed; add `[skip-i18n-check]` to the pull request title to skip that part, for a typo fix for example).

## License

[MIT](LICENSE)
