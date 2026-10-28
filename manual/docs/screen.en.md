# Understanding the screen

The screen has the same layout as VS Code.

| Place | Contents | Toggle |
| --- | --- | --- |
| Left | File tree | `Ctrl+B` |
| Top | Tabs | — |
| Center | The contents of a tab: editor, preview, browser, and so on | — |
| Bottom | Terminal panel | ``Ctrl+` `` |
| Right | Suggestions panel ([AI agent integration](agent.md)) | `Alt+I` |
| Very bottom | Status bar | — |

While in [diff mode](review.md), a header is added at the top and a help line above the status bar.

## Moving focus

- `Ctrl+E`: move focus between the file tree and the editor.
- `Space e` (Helix mode): move to the file tree.
- `Space E` (Helix mode): in diff mode, move to CHANGES. Outside diff mode, move to the file tree.
- `Esc` in the file tree: go back to the editor.
- `Alt+I` (also `Space i` in Helix mode): move to the suggestions panel. Press it again to close.
- Clicking with the mouse also moves focus there.

## Status bar

On the left, from the left, it shows:

1. The mode (when the editor has focus in Helix mode): `NOR` (normal), `INS` (insert), `SEL` (select). Keys you have pressed partway, such as `Space`, are shown too.
2. The Git branch name (in the form `⎇ main`)
3. When focus is not on the editor, the name of that place ("File tree", "Terminal", "Suggestions")
4. The number of diagnostics (`● errors  ▲ warnings`)
5. When the suggestions panel is not shown, the number of suggestions (`Suggestions N (Alt+I)`)

On the right, it shows:

1. The cursor position (`line:column`). With multiple cursors, the number of cursors. During a selection, the number of selected characters
2. The indentation (`Spaces: 4` or `Tab`)
3. The language name and the LSP status

## Resizing panes

Press `Alt+R` to enter resize mode.

| Key | Action |
| --- | --- |
| `←` / `→` (`h` / `l`) | Move the left/right border of the focused pane 2 columns left / right |
| `↑` / `↓` (`k` / `j`) | Move the top/bottom border of the focused pane 2 rows up / down |
| `<` / `>` | Boundary between the editor and the preview |
| `=` | Reset to the initial values |
| `Esc` | Leave resize mode |

The focused pane decides which border the arrow keys move.

| Focused pane | `←` / `→` | `↑` / `↓` |
| --- | --- | --- |
| Explorer / CHANGES | Right edge of the sidebar | In diff mode, the boundary between the explorer and CHANGES. Otherwise the top of the terminal |
| Editor | In a split diff, the `│` between left and right. Otherwise the border with the preview or the suggestions panel. If there is neither, the right edge of the sidebar | Top of the terminal |
| Terminal | Right edge of the sidebar | Top of the terminal |
| Suggestions panel | Left edge of the suggestions panel | Top of the terminal |

You can also change them by dragging the border with the mouse.
In diff mode, dragging the boundary between the explorer and CHANGES (the CHANGES heading row) changes the height ratio of the top and bottom. Neither becomes smaller than 3 rows. `=` resets it to 7:3. The ratio is kept when you change the height of the terminal.
The sizes are kept until you quit shijimi.

## Switching tabs

| Key | Action |
| --- | --- |
| `Ctrl+PgUp` / `Ctrl+PgDn` | Previous / next tab |
| `Alt+1` to `Alt+9` | The 1st to 9th tab |
| `gn` / `gp` (Helix mode) | Next / previous tab |
| `Ctrl+W` | Close the tab |

## Command palette

Open the command palette with `F1` or `Ctrl+K` (also `Space ?` in Helix mode).
Type part of a name and choose the operation to run. You can run operations whose keys you do not remember from here.
