# Configuration files

Configuration files are written in TOML. `languages.toml` is compatible with Helix. If you are moving from Helix, copy your `languages.toml` to `~/.config/shijimi-editor/`.

## Load order

They are loaded in the following order. Later ones take precedence.

1. Built-in defaults
2. In `~/.config/shijimi-editor/`: `config.toml`, then `config.local.toml`; `languages.toml`, then `languages.local.toml`
3. In the project's `.shijimi-editor/`: `config.toml`, then `config.local.toml`; `languages.toml`, then `languages.local.toml`

`~/.config` means `$XDG_CONFIG_HOME` when it is set. It is read from there on every OS, macOS included (not from `~/Library/Application Support`).

- Write language servers in `languages.toml` ([LSP](lsp.md)).
- Write everything else in `config.toml` (this page).
- Run `:config-reload` while running to reload them.
- If a file has a mistake, shijimi-editor does not start. Keys it does not know are only a warning ([If a file has a mistake](#if-a-file-has-a-mistake)).
- `[agent]` is read only from the user's own `config.toml` and `config.local.toml` (in `~/.config/shijimi-editor/`, or the nightly build's directory below) ([AI agent integration](agent.md)).
- `config.nix` and `languages.nix` are not read. Write the settings in TOML (with home-manager, in `settings` and `languages`; [Nix and home-manager](nix.md)).

## Local files

`config.local.toml` and `languages.local.toml` are for what you change on your own machine only. They win over `config.toml` and `languages.toml` of the same folder:

- The `config.toml` and `languages.toml` that home-manager writes are read-only. Write what you want to change on one machine in `~/.config/shijimi-editor/config.local.toml`.
- A project's `.shijimi-editor/config.toml` is shared through git. Write your own settings for the project in `.shijimi-editor/config.local.toml`, and add the local files to the project's `.gitignore`:

```gitignore
.shijimi-editor/*.local.toml
```

Each file is laid over what was read before it, in the same way for a `.local` file and for the next folder:

- A table (`[editor]`, `[search]`, `[keys.normal]`, ...) is merged key by key: a key you do not write keeps its value. `environment` (of `[agent]` and of a language server) is the exception: it is replaced as a whole.
- Any other value, an array included (`search.exclude`, `args`, `file-types`, ...), is replaced as a whole.
- In `languages.toml`, a `[language-server.<name>]` and a `[[language]]` with the same `name` are merged key by key; a server's `config` is merged table by table.
- What is read only from the user's own folder (`[agent]`, `[keys]`) is also read from `~/.config/shijimi-editor/config.local.toml`, and is a mistake in the project's `.shijimi-editor/config.local.toml`.

## If a file has a mistake

If any file has a mistake, `shj` does not start. It prints the file, the line and the reason for every mistake, and exits with a non-zero status:

```
/home/me/.config/shijimi-editor/config.toml:3: editor.file-icons: must be true or false, but it is a string
/home/me/.config/shijimi-editor/config.toml:8: ui-language: is not valid, or this version of shijimi-editor cannot use it. It must be one of: en, ja
```

These are mistakes:

- TOML syntax errors.
- Values of the wrong type, such as a string where a number goes, and values that are not allowed (`ui-language`, `[terminal]` `multiplexer`, `timeout` of 0 or less). A setting that has a fixed list of values also stops on a value that a newer version added, because this version cannot use it: the message says "is not valid, or this version of shijimi-editor cannot use it".
- `[agent]` and `[keys]` in a project's `.shijimi-editor/config.toml` or `config.local.toml`, because they are read only from the user's own directory.

Older versions ignored these mistakes in silence. After an update, a file that used to start the editor may stop it; fix what the message points at.

While it is running, `:config-reload` with a mistake in a file does not stop the editor. It keeps the current settings and shows the first mistake in the status line.

## Unknown keys are ignored

A key that this version does not read is not a mistake. shijimi-editor ignores it and starts. This lets one file be used by versions of different ages (a file written for a newer version still starts an older one), and it includes keys that Helix has but shijimi-editor does not read, such as `comment-token` in `languages.toml`. Only the keys in this manual are read.

- `shj` shows the first ignored key (with the file and line) in the status line when it starts, and after `:config-reload`.
- kisui-terminal's settings (`[server]`, `[gui]`) are not read from these files: kisui-terminal has its own (see its README). Keys of those tables left in a file are ignored like any other key.

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
| `"auto"` (default) | The innermost multiplexer that shijimi runs in. In the primary terminal of kisui-terminal, its panes (every value except `"none"` means this there). Otherwise it follows the parent processes of shijimi and takes the first multiplexer it finds; with none, the terminal panel is used |
| `"herdr"` | herdr (needs `HERDR_ENV=1`) |
| `"tmux"` | tmux 3.1 or later (needs `TMUX`) |
| `"zellij"` | zellij 0.44 or later (needs `ZELLIJ`) |
| `"cmux"` | cmux on macOS (needs `CMUX_WORKSPACE_ID` and `CMUX_SURFACE_ID`). It cannot hide the pane |
| `"gui"` | The panes of kisui-terminal (only for the editor that runs in the primary terminal of a kisui-terminal workspace; elsewhere the terminal panel is used) |
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
```

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
