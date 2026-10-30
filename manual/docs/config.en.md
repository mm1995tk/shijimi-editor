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
- `[agent]` is read only from the user's own `config.toml` (`~/.config/shijimi-editor/config.toml`, or the nightly build's directory below) ([AI agent integration](agent.md)).

## Nightly build

`shj-nightly` ([Installation](install.md)) reads `~/.config/shijimi-editor-nightly/` instead of `~/.config/shijimi-editor/`, so the two builds can be set up differently. While `~/.config/shijimi-editor-nightly/` does not exist, it reads `~/.config/shijimi-editor/` instead; create the directory when you want the nightly to have its own settings. The project's `.shijimi-editor/` is the same for both.

`[experimental]` holds features that are being tried out before they reach a release. Its keys default to `true` in the nightly build and to `false` in the stable build. There are no keys yet.

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
