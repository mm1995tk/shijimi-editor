# Understanding the screen

The screen has the same layout as VS Code.

| Place | Contents | Toggle |
| --- | --- | --- |
| Left | File tree | `Ctrl+B` |
| Top | Title line (the path of the open file, and ● when unsaved) | — |
| Center | The contents of the open file: editor, preview, browser, and so on | — |
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

## Switching files

There is no tab bar. The files you have opened are kept in a list, and only the current one is shown, with its path on the title line. To see two files at once, [split the editor](#split-editor).

| Key | Action |
| --- | --- |
| `Space b` (Helix mode) / Quick Open | Pick from the open files / any file |
| `ga` (Helix mode) / `Ctrl+Tab` | Go back and forth between the last file and this one |
| `Ctrl+O` / `Ctrl+I` | Go back / forward to where you jumped from |
| `Ctrl+W` | Close the current file (remove it from the list) |

`Ctrl+Tab` only reaches shijimi on terminals that support the kitty keyboard protocol. Elsewhere use `ga` or `Space b`.

## Split editor

You can split the editor into two panes side by side, to read two files together (an implementation and its test, for example). Each pane has its own title line. There is no up-and-down split and no third pane. Most of the time there is one pane.

| Helix mode | VS Code mode | Action |
| --- | --- | --- |
| `Space w v` | `Ctrl+\` | Split. The current file stays on the left, and the right pane, which takes the focus, shows the last file (`ga`). If there is none, the file picker opens |
| `Space w h` / `Space w l` / `Space w w` | `Ctrl+1` / `Ctrl+2` | Move the focus to the left pane / the right pane / the other pane (also: click or scroll in a pane) |
| `Space w q` / `Space w o` | `Ctrl+\` | Close the split. `q` keeps the other pane's file and `o` (and `Ctrl+\`) keeps the focused pane's file. The closed file stays in the list of open files |

`:vsplit` (`:vs`) and `:only` do the same from the command line, and the command palette has them too. (`Ctrl+W` is "close the file", so the Helix `Ctrl+W` window keys are under `Space w` instead.)

- `Space b`, `ga` / `Ctrl+Tab` and `Ctrl+O` / `Ctrl+I` act on the focused pane. Each pane keeps its own last file for `ga`. The other pane is not touched; a file the other pane shows is swapped into this pane.
- The same file is never in both panes. Opening a file that the other pane shows (from the file tree, for example) moves the focus to that pane.
- `Ctrl+W` closes the focused pane's file. The split closes too, and the other file fills the area.
- The two panes are the same width (the mouse cannot resize them). The split needs an editor area at least 41 columns wide.
- In diff mode the editor is one pane, and the split comes back when you leave diff mode. The Markdown preview and images work inside a pane.

## Command palette

Open the command palette with `F1` or `Ctrl+K` (also `Space ?` in Helix mode).
Type part of a name and choose the operation to run. You can run operations whose keys you do not remember from here.
