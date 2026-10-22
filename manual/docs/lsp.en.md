# LSP

shijimi uses language servers (LSP) to provide the following features.

| Feature | Helix mode | VS Code mode |
| --- | --- | --- |
| Diagnostics | `]d` / `[d` to move, `Space d` for a list | — |
| Completion | Automatic. `Ctrl+X` / `Ctrl+Space` for manual | Automatic. `Ctrl+Space` for manual |
| Hover | `K` / `Space k` | — |
| Signature help (parameter display) | Automatic. `:signature` for manual | Automatic. `:signature` for manual |
| Go to definition | `gd` | `F12` |
| References | `gr` | `Shift+F12` |
| Rename | `Space r` | `F2` |
| Code actions (quick fixes and refactorings) | `Space .` / `:action` | `:action` |
| Symbol list (in the file) | `Space s` / `:symbol` | `:symbol` |
| Symbol search (whole project) | `Space S` / `:symbol-all` | `:symbol-all` |
| Format | `:fmt` | `:fmt` |

Operations marked `—` can be run from the command palette (`F1`).

## Code actions

`Space .` (or `:action`) lists the code actions available at the cursor. Choosing one applies it.

- The fix for a diagnostic hint (for example, gopls's "Loop can be simplified using slices.Contains") appears in this list.
- When there is a selection, the code actions for the selected range are shown. When there is no selection and the cursor is outside a diagnostic's range, the diagnostics on the cursor's line are used.
- Changes are not saved. Review them, then save with `:w`.

## Symbol list

`Space s` (or `:symbol`) lists the functions, types, variables and so on in the open file. Choosing one jumps to its line.

- The list shows the kind (function, type, variable, and so on) and the line number. Nested symbols also show the names of their enclosing symbols.
- You can filter by typing.

`Space S` (or `:symbol-all`) searches the whole project for symbols by name.

- When you type a name, shijimi queries the language server and builds the list. The query is sent after you stop typing.
- Choosing one opens the file and jumps to the line of the definition.

## Hover with the mouse

If you rest the mouse on a word in the editor for 0.5 seconds, the hover for that position appears. The cursor does not move.

- The popup appears at the mouse position. It stays open while the mouse is over the popup, and you can scroll it with the wheel.
- It closes when the mouse leaves the word (moving to another character of the same word does not close it) and the popup, when you click, or when you press a key.
- It opens only while the editor has focus. The popup does not take keys; pressing a key closes it.
- Nothing is shown when the language server is not running, when there is no hover information, or on errors.
- You can turn it off with `mouse-hover = false` under `[editor]` ([Configuration files](config.md)). When turned off, mouse movement events without a button press are not received either.

## Built-in settings

The settings for the following language servers are built in. If the server is on your PATH, it works without any configuration.

| Language | Server |
| --- | --- |
| Go | `gopls` |
| Rust | `rust-analyzer` |
| TypeScript / JavaScript (including JSX and TSX) | `typescript-language-server` |
| Python | `pylsp` |
| C / C++ | `clangd` |
| Nix | `nil` |
| Haskell | `haskell-language-server-wrapper` |
| Lua | `lua-language-server` |
| Bash | `bash-language-server` |
| Zig | `zls` |
| TOML | `taplo` |
| YAML | `yaml-language-server` |
| JSON | `vscode-json-language-server` |

## Writing settings

Settings go in `languages.toml`, which is compatible with Helix.
If you are moving from Helix, copy your `languages.toml` to `~/.config/shijimi/`.
Where files are read from and in what order is described in [Configuration files](config.md).

```toml
# .shijimi/languages.toml
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

The supported keys are as follows.

| Section | Keys |
| --- | --- |
| `[language-server.*]` | `command` `args` `config` `environment` |
| `[[language]]` | `name` `language-id` `file-types` `roots` `language-servers` `indent.unit` `formatter` |

## Formatting with a formatter

`:fmt` normally asks the language server to format.
If you want to use a formatting tool that does not run as a language server (oxfmt, nixfmt, and so on), or you have two or more language servers and want to choose which one formats, write `formatter` in `[[language]]`.
For languages that have a `formatter`, it takes priority over the language server.

```toml
[[language]]
name = "typescript"
formatter = { command = "oxfmt", args = ["--stdin-filepath", "%{buffer_name}"] }

[[language]]
name = "nix"
formatter = { command = "nixfmt" }
```

- `%{buffer_name}` in `args` is replaced with the path of the file being formatted (relative to the project root).
- The command runs in the project root.
- The buffer content (including unsaved content) is passed on standard input, and standard output becomes the new content.
- Only the changed lines are replaced, so the cursor and scroll position do not move. Formatting can be undone with a single undo (`u` / `Ctrl+Z`).
- If the command exits with a non-zero status, is not found, or does not finish in 10 seconds, the buffer is left unchanged. The first line of standard error is shown in the status line.
- There is no format on save, because auto save would move the text you are typing every time. Format with `:fmt`.

## Restarting

| Command | Action |
| --- | --- |
| `:lsp-restart` | Restart the language server |
| `:config-reload` | Reload the configuration files |

## Signature help (parameter display)

When you type the `(` of a function or the `,` between arguments, the function's parameter list appears one line above the cursor. The current parameter is bold.

- It disappears with `)` or `Esc`. It also disappears when the cursor moves to another line.
- To show it manually, use `:signature` (it works outside insert mode too). `Ctrl+Shift+Space` cannot be used because terminals send it as the same signal as `Ctrl+Space`.
- The characters that trigger it automatically are the `triggerCharacters` reported by the language server. If it reports none, they are `(` and `,`.
