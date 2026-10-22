# Configuration files

Configuration files are written in TOML. `languages.toml` is compatible with Helix. If you are moving from Helix, copy your `languages.toml` to `~/.config/shijimi/`.

## Load order

They are loaded in the following order. Later ones take precedence.

1. Built-in defaults
2. `~/.config/shijimi/languages.toml`, `~/.config/shijimi/config.toml`
3. The project's `.shijimi/languages.toml`, `.shijimi/config.toml`

- Write language servers in `languages.toml` ([LSP](lsp.md)).
- Write everything else in `config.toml` (this page).
- Run `:config-reload` while running to reload them.
- `[agent]` is read only from `~/.config/shijimi/config.toml` ([AI agent integration](agent.md)).

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

# Effective only in ~/.config/shijimi/config.toml
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
| `editor.line-number` | How line numbers are shown. `absolute` or `relative` | — |
| `editor.file-icons` | Icons in the file tree | [Files and search](files.md) |
| `editor.mermaid-command` | Command that converts mermaid to PNG | [Preview](preview.md) |
| `editor.watch-files` | Whether to reflect external changes | [Files and search](files.md) |
| `editor.indent-guides.render` | Whether to draw indent guides (default true) | — |
| `editor.mouse-hover` | Whether to show hover when the mouse rests on a word | [LSP](lsp.md) |
| `editor.auto-save.after-delay` | Auto save | [Files and search](files.md) |
| `formatter` in `[[language]]` | The formatter command that `:fmt` runs (languages.toml) | [LSP](lsp.md) |
| `search.exclude` / `search.include` | Exclude / include for project-wide search | [Files and search](files.md) |
| `[keys.normal]` `[keys.insert]` `[keys.select]` | Remap keys in Helix mode (only in `~/.config/shijimi/`) | [Editing (Helix mode)](editing-helix.md#remapping-keys) |
| `browser.listen-host` | Listen address for port forwarding | [External browser and port forwarding](browser.md) |
| `agent.command` / `agent.args` / `agent.environment` | The ACP agent used by the suggestions panel | [AI agent integration](agent.md) |

## Themes

There are three built-in themes.

| Name | Description |
| --- | --- |
| `dark-plus` | Default |
| `github-dark` | — |
| `light-plus` | — |

Helix theme names also work. `github_dark*` maps to `github-dark`, and `github_light*` and `light` map to `light-plus`.

There are three ways to switch.

- Write `theme = "<name>"` in `config.toml`.
- Type `:theme <name>` while running. Without a name, a list is shown.
- Choose "Settings: Switch theme" in the command palette.

## Remapping keys

With `[keys.normal]` `[keys.insert]` `[keys.select]` you can remap keys in Helix mode. The syntax and the list of command names are in [Editing (Helix mode)](editing-helix.md#remapping-keys). They are read only from `~/.config/shijimi/config.toml` (they have no effect in a project's `.shijimi/config.toml`).
