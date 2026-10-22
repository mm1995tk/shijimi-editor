# Editing (VS Code mode)

Modeless, VS Code-style operation.

## Switching

Switch in one of the following ways.

- Type `:keymap vscode` while running. To go back to Helix mode, use `:keymap helix`.
- Write `keymap = "vscode"` in `config.toml` ([Configuration files](config.md)).

## Keys

| Key | Action |
| --- | --- |
| Arrow keys | Move |
| `Shift` + arrow keys | Move while selecting |
| `Ctrl+←` / `Ctrl+→` (`Alt+B` / `Alt+F`) | Move by word. Add `Shift` to select |
| `Home` / `End` | Start / end of line. Add `Shift` to select |
| `Ctrl+Home` / `Ctrl+End` | Start / end of the file. Add `Shift` to select |
| `PgUp` / `PgDn` | One screen up / down. Add `Shift` to select |
| `Alt+PgUp` / `Alt+PgDn` | Scroll the view left / right by half a screen without moving the cursor. It stops at the left edge and where the end of the longest line is visible. The horizontal mouse wheel and `Shift` + wheel do the same |
| `Alt+↑` / `Alt+↓` | Move the line up / down |
| `Tab` / `Shift+Tab` | Increase / decrease the indentation |
| `Ctrl+Z` | Undo |
| `Ctrl+Y` / `Ctrl+Shift+Z` | Redo |
| `Ctrl+A` | Select all |
| `Ctrl+C` / `Ctrl+X` | Copy / cut. With no selection, the whole line |
| `Ctrl+V` | Paste |
| `Ctrl+F` | Search in the file |
| `Ctrl+/` | Toggle a line comment |
| `Ctrl+Space` | Trigger completion manually (it is not used to toggle the terminal) |
| `Tab` / `Shift+Tab` | Move to the next / previous placeholder of a completion snippet (only while filling in; `Esc` ends it) |
| `F12` | Go to definition |
| `Shift+F12` | List references |
| `Alt+←` / `Alt+→` | Go back to / forward from the place before a jump |
| `F2` | Rename |
| `Esc` | Clear the selection and extra cursors |

The global keys (`Ctrl+P`, `Ctrl+S` and so on) are described in [Understanding the screen](screen.md) and [Files and search](files.md).
