# Editing (Helix mode)

The default is Helix-style modal editing.
To switch to VS Code-style editing, see [Editing (VS Code mode)](editing-vscode.md).

## Modes

| Mode | Status bar | How to enter | How to leave |
| --- | --- | --- | --- |
| Normal | `NOR` | At startup. `Esc` | — |
| Insert | `INS` | `i` `a` `I` `A` `o` `O`, `c` | `Esc` |
| Select | `SEL` | `v` | `v` or `Esc` |

As in Helix, you edit by selecting first and then acting.
For example, select a word with `w`, then delete it with `d`.

## Normal mode

### Movement

| Key | Action |
| --- | --- |
| `h` `j` `k` `l` (arrow keys also work) | Move left / down / up / right. A number before it repeats (for example `5j`) |
| `w` `b` `e` (`W` `B` `E`) | Move while selecting a word |
| `Home` / `End` | Start / end of line |
| `PgUp` `Ctrl+B` / `PgDn` `Ctrl+F` | Page up / down |
| `Ctrl+U` / `Ctrl+D` | Move up / down. In terminals that report key release (those supporting the kitty keyboard protocol), it scrolls one line at a time while the key is held. Otherwise it moves half a screen and puts the cursor line in the middle of the screen |
| `zh` / `zl` (`Alt+PgUp` / `Alt+PgDn`) | Scroll the view half a screen left / right without moving the cursor. It stops at the left edge and where the end of the longest line is visible. The horizontal mouse wheel and `Shift` + wheel do the same |
| `G` | Last line |
| `f` `t` `F` `T` + character | Find a character in the line (`f` `t` forward, `F` `T` backward) |
| `mm` | Go to the matching bracket |

### Jumps starting with `g`

| Key | Action |
| --- | --- |
| `gg` / `ge` | Start / end of the file |
| `gh` / `gl` | Start / end of the line |
| `gs` | First non-blank character of the line |
| `gt` / `gc` / `gb` | Top / middle / bottom line of the screen |
| `<number>g` | The given line (for example `42g`) |
| `gd` / `gr` | Go to definition / list references |
| `Ctrl+O` / `Ctrl+I` | Go back / forward to where you jumped from (up to 100 entries. `gd` `gg` `ge` `<number>g` `:<line number>` `/` `n` `N` and the pickers add to the history) |
| `gn` / `gp` | Next / previous tab |

### Selection

| Key | Action |
| --- | --- |
| `x` (`X`) | Select a line. Pressing it again extends downward |
| `%` | Select all |
| `;` | Clear the selection |
| `Alt+;` | Flip the selection direction (swap the cursor and the anchor) |
| `v` | Toggle select mode |

### Editing

| Key | Action |
| --- | --- |
| `d` | Delete and yank the selection (or the character under the cursor if none) |
| `Alt+D` | Delete without yanking |
| `c` | Delete and enter insert mode. `Alt+c` (lowercase) does not yank |
| `y` | Yank (copy) |
| `p` / `P` | Paste after / before the cursor |
| `R` | Replace the selection with the yanked content |
| `r` + character | Replace the characters in the selection with the given character |
| `u` / `U` | Undo / redo |
| `>` / `<` | Increase / decrease indentation |
| `J` | Join lines |
| `~` | Swap upper and lower case |
| `` ` `` / `` Alt+` `` | Convert to lowercase / uppercase |
| `Ctrl+C` | Toggle line comment |

### Search

| Key | Action |
| --- | --- |
| `/` / `?` | Search forward / backward |
| `n` / `N` | Next / previous match |
| `*` | Use the selection (within one line) as the search term |

### Others

| Key | Action |
| --- | --- |
| `K` | Hover (LSP) |
| `]d` / `[d` | Next / previous diagnostic |
| `:` | Enter a command ([:Commands](commands.md)) |

## Multiple cursors

| Key | Action |
| --- | --- |
| `C` | Copy the cursor to the line below |
| `Alt+C` (uppercase) | Copy the cursor to the line above |
| `s` | Split the selection by a regular expression and place a cursor at each match |
| `,` | Reduce to a single cursor |

Edits such as insertion, deletion and `c` apply to all cursors.
The number of cursors is shown at the right of the status bar.

## Space menu

Press `Space`, then one of the following keys.

| Key | Action |
| --- | --- |
| `Space f` | Open a file (Quick Open) |
| `Space b` | List of open buffers |
| `Space e` | Move to the file tree |
| `Space E` | Move to CHANGES (in diff mode; outside diff mode, move to the file tree) |
| `Space k` | Hover |
| `Space r` | Rename (LSP) |
| `Space .` | Code actions (LSP. Apply diagnostic hints) |
| `Space s` / `Space S` | Symbol list (LSP. In the file / whole project) |
| `Space d` | List of diagnostics |
| `Space /` | Search / replace in the whole project (floating window. [Files and search](files.md)) |
| `Space c` | Toggle line comment |
| `Space t` | Toggle the terminal |
| `Space g` | Open / close diff mode (compared with the base of the pull request) |
| `Space G` | Open / close diff mode (compared with `HEAD`) |
| `Space v` | Review menu (when the comparison is with the base of a pull request. [Diff mode](review.md)) |
| `Space m` | Toggle the Markdown preview (side by side → full width → closed) |
| `Space a` | Send the current context to the agent |
| `Space i` | Move to / close the suggestions panel |
| `Space o` | Open the URL under the cursor in an external browser |
| `Space y` / `Space p` | Copy to / paste from the system clipboard |
| `Space ?` | Command palette |

## Insert mode

Type text. `Esc` returns to normal mode.

| Key | Action |
| --- | --- |
| `Ctrl+X` / `Ctrl+Space` | Trigger completion manually |
| `Tab` / `Enter` | Accept the completion |
| `Ctrl+W` / `Alt+Backspace` | Delete the previous word |
| `Ctrl+U` | Delete to the start of the line |

Completion appears automatically after typing `.` and similar characters, or after typing 2 characters.

When you accept a completion with placeholders (a snippet), such as function arguments, the first placeholder is selected. Type to replace it. `Tab` moves to the next placeholder and `Shift+Tab` to the previous one. It ends when you reach the last placeholder (or `$0`) or press `Esc`. Placeholders with the same number change together. It also ends if you move the cursor out of the placeholders.

Other keys (arrow keys, `Ctrl+Z`, and so on) behave the same as in [VS Code mode](editing-vscode.md).

## Remapping keys

You can remap keys with `[keys.normal]` `[keys.insert]` `[keys.select]` in `~/.config/shijimi-editor/config.toml`. The syntax is the same as Helix's `config.toml`.

```toml
[keys.normal]
"C-s" = ":write"           # : commands can be used too
"H" = "goto_line_start"
"A-j" = "move_visual_line_down"
"d" = "delete_selection_noyank"
"q" = "no_op"              # disable the key

[keys.select]
"j" = "extend_line_below"  # remap in select mode only

[keys.insert]
"A-j" = "normal_mode"
```

- Write keys like `C-s` (Ctrl), `A-x` (Alt), `S-tab` (Shift). They can be combined, as in `C-A-x`. Special keys use the same names as Helix (`ret` `space` `tab` `esc` `backspace` `del` `ins` `left` `right` `up` `down` `home` `end` `pageup` `pagedown` `F1` to `F12`). Write `-` as `minus`, `<` as `lt`, `>` as `gt`, and `+` as `plus`.
- A value is a command name from the tables below, or a command starting with `:` (such as `":write"`; see [:Commands](commands.md)).
- `[keys.select]` is layered on top of `[keys.normal]`.
- Keys without a name (sequences starting with `g`, `/` `n`, the `Space` menu, and so on) keep working as they are. Sequences of consecutive keys such as `[keys.normal.g]` cannot be written yet.
- Keys for operations that work in every mode, such as `Ctrl+S` `Ctrl+Q` `Ctrl+P`, cannot be remapped.
- If you write an unknown command name or key name, the status line reports it at startup (and on `:config-reload`). That key keeps its default behavior.
- For safety, `[keys.*]` is read only from `~/.config/shijimi-editor/config.toml` (and `config.nix`). It is ignored in a project's `.shijimi-editor/config.toml` (so that just opening a repository that binds `:sh` or similar cannot run a program).
- In `[keys.insert]` you can write the same command names or `:` commands. That key no longer types a character.

### Available command names

"Default key" is the key when not remapped.

Movement:

| Command name | Default key |
| --- | --- |
| `move_char_left` / `move_char_right` | `h` `←` / `l` `→` |
| `move_visual_line_down` / `move_visual_line_up` | `j` `↓` / `k` `↑` |
| `move_line_down` / `move_line_up` | Same movement as the two above (so you can write the same names as Helix) |
| `move_next_word_start` / `move_next_long_word_start` | `w` / `W` |
| `move_prev_word_start` / `move_prev_long_word_start` | `b` / `B` |
| `move_next_word_end` / `move_next_long_word_end` | `e` / `E` |
| `goto_line_start` / `goto_line_end` | `Home` / `End` |
| `page_up` / `page_down` | `PgUp` `Ctrl+B` / `PgDn` `Ctrl+F` |
| `jump_backward` / `jump_forward` | `Ctrl+O` / `Tab` `Ctrl+I` |
| `goto_last_line` | `G` |

`move_*` extends the selection only in select mode. `extend_*` always extends the selection (`extend_char_left` `extend_char_right` `extend_visual_line_down` `extend_visual_line_up` `extend_line_down` `extend_line_up` `extend_next_word_start` `extend_next_long_word_start` `extend_prev_word_start` `extend_prev_long_word_start` `extend_next_word_end` `extend_next_long_word_end` `extend_to_line_start` `extend_to_line_end`).

Selection:

| Command name | Default key |
| --- | --- |
| `extend_line_below` | `x` |
| `extend_to_line_bounds` | `X` |
| `select_all` | `%` |
| `collapse_selection` | `;` |
| `flip_selections` | `Alt+;` |
| `select_mode` | `v` (toggle select mode) |
| `normal_mode` | `Esc` |
| `no_op` | None (disables the key) |

Editing:

| Command name | Default key |
| --- | --- |
| `delete_selection` / `delete_selection_noyank` | `d` / `Alt+D` |
| `change_selection` / `change_selection_noyank` | `c` / `Alt+c` |
| `yank` | `y` |
| `paste_after` / `paste_before` | `p` / `P` |
| `replace_with_yanked` | `R` |
| `undo` / `redo` | `u` / `U` |
| `insert_mode` / `append_mode` | `i` / `a` |
| `insert_at_line_start` / `insert_at_line_end` | `I` / `A` |
| `open_below` / `open_above` | `o` / `O` |
| `indent` / `unindent` | `>` / `<` |
| `join_selections` | `J` |
| `switch_case` | `~` |
| `switch_to_lowercase` / `switch_to_uppercase` | `` ` `` / `` Alt+` `` |
| `toggle_comments` | `Ctrl+C` |

A number before the key (a repeat count) and application to multiple cursors work the same way for remapped keys.
