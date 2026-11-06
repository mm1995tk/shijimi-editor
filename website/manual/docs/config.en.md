# Configuration files

Configuration files are written in TOML. `languages.toml` is compatible with Helix. If you are moving from Helix, copy your `languages.toml` to `~/.config/shijimi-editor/`.

## Load order

They are loaded in the following order. Later ones take precedence.

1. Built-in defaults
2. `~/.config/shijimi-editor/languages.toml`, `~/.config/shijimi-editor/config.toml`
3. The project's `.shijimi-editor/languages.toml`, `.shijimi-editor/config.toml`

`~/.config` means `$XDG_CONFIG_HOME` when it is set. It is read from there on every OS, macOS included (not from `~/Library/Application Support`).

- Write language servers in `languages.toml` ([LSP](lsp.md)).
- Write everything else in `config.toml` (this page).
- Run `:config-reload` while running to reload them.
- If a file has a mistake, shijimi-editor does not start. Keys it does not know are only a warning ([If a file has a mistake](#if-a-file-has-a-mistake)).
- `[agent]` is read only from the user's own `config.toml` (`~/.config/shijimi-editor/config.toml`, or the nightly build's directory below) ([AI agent integration](agent.md)).

## If a file has a mistake

If any file has a mistake, `shj` and `kisui serve` do not start. They print the file, the line and the reason for every mistake, and exit with a non-zero status:

```
/home/me/.config/shijimi-editor/config.toml:3: editor.file-icons: must be true or false, but it is a string
/home/me/.config/shijimi-editor/config.toml:8: ui-language: is not valid, or this version of shijimi-editor cannot use it. It must be one of: en, ja
```

These are mistakes:

- TOML syntax errors, and Nix files (`config.nix` / `languages.nix`) that do not evaluate (Nix has no lines, so the setting is shown instead).
- Values of the wrong type, such as a string where a number goes, and values that are not allowed (`ui-language`, `[terminal]` `multiplexer`, a negative `idle_stop_days`, `timeout` of 0 or less, a `start_dir` that is not a folder). A setting that has a fixed list of values also stops on a value that a newer version added, because this version cannot use it: the message says "is not valid, or this version of shijimi-editor cannot use it".
- `[agent]`, `[server]`, `[gui]` and `[keys]` in a project's `.shijimi-editor/config.toml`, because they are read only from the user's own directory.

Older versions ignored these mistakes in silence. After an update, a file that used to start the editor may stop it; fix what the message points at.

While it is running, `:config-reload` with a mistake in a file does not stop the editor. It keeps the current settings and shows the first mistake in the status line. A mistake in the user's own files stops `kisui serve` itself, and `kisui attach` passes the reason to the GUI as the connection error.

## Unknown keys are ignored

A key that this version does not read is not a mistake. shijimi-editor ignores it and starts. This lets one file be used by versions of different ages (a file written for a newer version still starts an older one), and it includes keys that Helix has but shijimi-editor does not read, such as `comment-token` in `languages.toml`. Only the keys in this manual are read.

- `shj` shows the first ignored key (with the file and line) in the status line when it starts, and after `:config-reload`.
- `kisui serve` prints every ignored key to the standard error and writes it to its log, then goes on.
- The same goes for unknown keys inside `[gui]`.

```
/home/me/.config/shijimi-editor/config.toml:4: editor.bogus: unknown key, ignored (it may be for a newer shijimi-editor; this version does not read it)
```

## Nightly build

`shj-nightly` ([Installation](install.md)) reads `~/.config/shijimi-editor-nightly/` instead of `~/.config/shijimi-editor/`, so the two builds can be set up differently. While `~/.config/shijimi-editor-nightly/` does not exist, it reads `~/.config/shijimi-editor/` instead; create the directory when you want the nightly to have its own settings. The project's `.shijimi-editor/` is the same for both.

`[experimental]` holds features that are being tried out before they reach a release. Its keys default to `true` in the nightly build and to `false` in the stable build. The keys are:

| Key | Meaning |
| --- | --- |
| `multiplexer` | Inside a terminal multiplexer, use a pane of it instead of the terminal panel; `[terminal]` `multiplexer` chooses which ([Terminal](terminal.md#inside-a-terminal-multiplexer)). The old name `herdr` is read as the same key |

`[terminal]` `multiplexer` chooses the multiplexer whose pane is used while `[experimental]` `multiplexer` is on:

| Value | Meaning |
| --- | --- |
| `"auto"` (default) | The innermost multiplexer that shijimi runs in. In the primary terminal of the GUI, the GUI. Otherwise it follows the parent processes of shijimi and takes the first multiplexer it finds; with none, the terminal panel is used |
| `"herdr"` | herdr (needs `HERDR_ENV=1`) |
| `"tmux"` | tmux 3.1 or later (needs `TMUX`) |
| `"zellij"` | zellij 0.44 or later (needs `ZELLIJ`) |
| `"cmux"` | cmux on macOS (needs `CMUX_WORKSPACE_ID` and `CMUX_SURFACE_ID`). It cannot hide the pane |
| `"gui"` | The panes of the shijimi-editor GUI (only for the editor that runs in the primary terminal of a GUI workspace; elsewhere the terminal panel is used) |
| `"none"` | Always the terminal panel, even inside a multiplexer |

## Example config.toml

```toml
theme = "github_dark"
keymap = "helix"          # "vscode" for VS Code-style operation
ui-language = "en"        # language of the UI text: "en" (default) or "ja"

[editor]
line-number = "relative"  # how line numbers are shown
file-icons = true         # Nerd Font icons in the file tree; false shows ▸/▾
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"  # empty disables it
watch-files = true        # reflect file changes made by other processes
mouse-hover = true        # show hover when the mouse rests on a word

# Indent guides (a faint vertical line for each indent). false disables them
[editor.indent-guides]
render = true

# Auto save (enabled by default. Saves when editing pauses for 1 second. Unnamed files are excluded)
[editor.auto-save.after-delay]
enable = true
timeout = 1000            # milliseconds

[search]
exclude = ["*.lock", "dist/**"]
include = []

[browser]
listen-host = "127.0.0.1"

[terminal]
multiplexer = "auto"      # "auto" / "herdr" / "tmux" / "zellij" / "cmux" / "gui" / "none"


# Effective only in ~/.config/shijimi-editor/config.toml
[agent]
command = "claude-agent-acp"
args = []

[server]
idle_stop_days = 7        # 0 = never stop
start_dir = "~/work"      # where a new workspace's shell starts (default ~)
```

## Machines in the GUI

The GUI (not `shj`) reads `[[gui.machines]]` in the config.toml on the machine where the GUI runs, and shows each machine in the sidebar after "Local". Use it when you always use the same machines, so you do not have to add them again after reinstalling the GUI or on another Mac.

```toml
[[gui.machines]]
ssh = "dev-vm"   # where to ssh to (required; a host from ~/.ssh/config, for example)
name = "dev"     # the name in the sidebar (optional; without it, the ssh target)
```

- The GUI reads the file when it starts. The nightly GUI reads `shijimi-editor-nightly/` the same way as `shj-nightly` ("Nightly build" on this page). Only `config.toml` is read; a `config.nix` is not.
- Machines written here cannot be removed or renamed in the GUI (the right-click menu is disabled and says so). To change them, edit the file and start the GUI again. Machines added in the GUI are saved inside the GUI as before.
- If a machine added in the GUI has the same ssh target as one in the file, the one in the file is shown. A machine added in the GUI is not shown while its name is the same as a machine in the file.
- If the file has a mistake (TOML syntax, a value of the wrong type, an ssh target that cannot be used, the same ssh target or name twice, `name` empty or "Local"), the GUI does not start. It shows the file, line and reason on a plain screen and does nothing else. Fix the file and start the GUI again. `shj` starts with a file that has `[gui]`.
- With home-manager, write it in `settings` ([Nix and home-manager](nix.md)).

## Options

| Option | Description | Details |
| --- | --- | --- |
| `theme` | Theme | "Themes" on this page |
| `keymap` | `helix` (default) or `vscode` | [Editing (VS Code mode)](editing-vscode.md) |
| `ui-language` | Language of the UI text (status line, command palette, suggestions panel, errors, `shj --help`). `en` (English, default) or `ja` (Japanese). It is not chosen from environment variables such as `LANG`. Other values are ignored and English is used. `:config-reload` switches it | — |
| `editor.line-number` | How line numbers are shown. `absolute` or `relative`. With the Helix keymap, `\` shows the other way | — |
| `editor.file-icons` | Icons in the file tree | [Files and search](files.md) |
| `editor.mermaid-command` | Command that converts mermaid to PNG | [Preview](preview.md) |
| `editor.watch-files` | Whether to reflect external changes | [Files and search](files.md) |
| `editor.indent-guides.render` | Whether to draw indent guides (default true) | — |
| `editor.mouse-hover` | Whether to show hover when the mouse rests on a word | [LSP](lsp.md) |
| `editor.auto-save.after-delay` | Auto save | [Files and search](files.md) |
| `editor.auto-save.on-switch` | No longer used. It was what `kisui serve` did with a workspace's open files when the workspace switched folders, and `kisui serve` no longer edits files. It is still accepted (`save` or `discard`) so that a file that has it keeps loading, and it is not required when auto save is off | — |
| `formatter` in `[[language]]` | The formatter command that `:fmt` runs (languages.toml) | [LSP](lsp.md) |
| `search.exclude` / `search.include` | Exclude / include for project-wide search | [Files and search](files.md) |
| `[keys.normal]` `[keys.insert]` `[keys.select]` | Remap keys in Helix mode (only in `~/.config/shijimi-editor/`) | [Editing (Helix mode)](editing-helix.md#remapping-keys) |
| `browser.listen-host` | Listen address for port forwarding | [External browser and port forwarding](browser.md) |
| `agent.command` / `agent.args` / `agent.environment` | The ACP agent used by the suggestions panel | [AI agent integration](agent.md) |
| `server.start_dir` | The folder where the primary terminal of a new workspace starts (an absolute path, or one that starts with `~`). Default `~`. A folder that does not exist is a mistake in the configuration. If it is inside a project, the workspace's folder is decided at once | — |
| `server.idle_stop_days` | Days `kisui serve` stays up with no GUI connected before it stops itself (default 7, 0 = never). Running shells end with it. The list of workspaces (their folders, and the ones with no folder yet) is saved in `workspaces.json` next to `serve.log`, and the next `kisui serve` makes them again, each with a new primary shell in its folder (in `start_dir` if the folder is gone or not decided). Split terminals do not come back | — |

## Themes

There are three built-in themes.

| Name | Description |
| --- | --- |
| `dark-plus` | — |
| `github-dark` | Default |
| `light-plus` | — |

Helix theme names also work. `github_dark*` maps to `github-dark`, and `github_light*` and `light` map to `light-plus`.

There are three ways to switch.

- Write `theme = "<name>"` in `config.toml`.
- Type `:theme <name>` while running. Without a name, a list is shown.
- Choose "Settings: Switch theme" in the command palette.

## Remapping keys

With `[keys.normal]` `[keys.insert]` `[keys.select]` you can remap keys in Helix mode. The syntax and the list of command names are in [Editing (Helix mode)](editing-helix.md#remapping-keys). They are read only from `~/.config/shijimi-editor/config.toml` (they have no effect in a project's `.shijimi-editor/config.toml`).
