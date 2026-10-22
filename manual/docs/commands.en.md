# :Command reference

Press `:` in normal mode in Helix mode to enter a command.
In VS Code mode, you can do the same from the command palette (`F1`).

## Files and buffers

| Command | Aliases | Action |
| --- | --- | --- |
| `:w [path]` | `:write` | Save. With a path, save to that path |
| `:wa` | `:write-all` | Save all |
| `:q` | `:quit` `:qa` | Quit. Stops if there are unsaved files |
| `:q!` | `:quit!` `:qa!` | Quit without saving |
| `:wq` | `:x` | Save and quit |
| `:wqa` | `:xa` | Save all and quit |
| `:o <path>` | `:open` `:e` `:edit` | Open a file |
| `:n` | `:new` | New file |
| `:bc` | `:bclose` `:buffer-close` | Close the buffer |
| `:bc!` | `:bclose!` `:buffer-close!` | Close the buffer without saving |
| `:reload` | `:rl` | Reload from disk |
| `:<line number>` | — | Go to that line |

## Editing and LSP

| Command | Aliases | Action |
| --- | --- | --- |
| `:fmt` | `:format` | Format |
| `:lsp-restart` | — | Restart the language server |

## Search and replace

| Command | Aliases | Action |
| --- | --- | --- |
| `:grep [-x exclude] [-i include] <text>` | `:rg` | Open the search / replace window (puts the text in the search field) |
| `:replace [-x exclude] [-i include] <text> => <replacement>` | `:rep` | Open the search / replace window (puts the values in the search and replace fields) |

## View

| Command | Aliases | Action |
| --- | --- | --- |
| `:preview` | `:md` | Cycle the Markdown preview (side by side, full width, closed) |
| `:tree` | `:explorer` | Move to the file tree |
| `:sh [command]` | `:run` `:!` | Run a command in the terminal. Without an argument, toggle the terminal |

## Diff mode

| Command | Action |
| --- | --- |
| `:gitdiff [on\|off\|split\|unified]` (`:gd` `:review`) | Toggle diff mode (base `HEAD`), toggle split / unified |
| `:pr [number]` | Compare with the base of a pull request in diff mode. Without a number, choose from a list |

## Browser

| Command | Aliases | Action |
| --- | --- | --- |
| `:browser forward <target port> [listen port]` | — | Start port forwarding |
| `:browser list` | — | List port forwards |
| `:browser stop <listen port>` | — | Stop port forwarding |
| `:open-url [URL]` | `:browse` | Open in an external browser. Without an argument, the URL under the cursor |

## Agent

| Command | Action |
| --- | --- |
| `:agent` | Go to / close the suggestions panel |
| `:agent cancel` | Cancel the agent's work |
| `:agent stop` | Stop the agent |

## Settings

| Command | Action |
| --- | --- |
| `:theme [name]` | Switch the theme. Without a name, show the list |
| `:keymap [helix\|vscode]` | Switch the editing style |
| `:config-reload` | Reload the configuration files |
