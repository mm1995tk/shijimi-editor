# Files and search

## Opening files

| Method | Action |
| --- | --- |
| `Ctrl+P` (also `Space f` in Helix mode) | Quick Open. Type part of a file name and pick it |
| `Space b` (Helix mode) | Pick from the list of open buffers |
| `:o <path>` | Open a file by path |
| `Ctrl+N` / `:n` | New unnamed file |
| `Enter` in the file tree | Open the selected file |

## Saving

| Method | Action |
| --- | --- |
| `Ctrl+S` / `:w` | Save |
| `:w <path>` | Save to another path |
| `:wa` | Save all |

**Auto save** is enabled by default. It saves when editing pauses for 1 second. Unnamed files are not saved.
To turn it off, see [Configuration files](config.md).

## File tree

`Ctrl+B` toggles it. `Ctrl+E` moves focus.

The sort order is: folders starting with a dot, other folders, files starting with a dot, other files. Within each group, names are sorted by letters, ignoring case.

Files and folders matched by `.gitignore` are shown dimmed. They get no Git status letter (M / U / A).

| Key | Action |
| --- | --- |
| `j` / `k` | Move down / up |
| `Ctrl+D` / `Ctrl+U` | Page down / up |
| `g` / `G` | First / last |
| `Enter` / `o` | Open the file and move to the editor. Toggle a folder |
| `l` | Open the file as a preview (focus stays in the tree). Expand a folder |
| `h` | Collapse the folder. If already collapsed, go to the parent folder |
| `a` / `n` | New file |
| `A` / `N` | New folder |
| `r` | Rename |
| `d` / `Delete` | Delete |
| `y` / `Y` | Copy the relative / absolute path of the selected item ("File: Copy relative path" and "File: Copy absolute path" in the command palette use the file of the current tab) |
| `R` | Refresh the tree |
| `Esc` / `q` / `Tab` | Back to the editor |

### Opening as a preview

Clicking a file or pressing `l` opens it as a **preview**. The tab of a preview has an italic name.

- There is only one preview tab. Opening another file as a preview replaces the tab in the same position.
- A preview tab becomes a normal tab when you do any of the following:
  - Edit the buffer
  - Double-click the tab name
  - Double-click the file in the file tree, or press `Enter` / `o`
- A file that is already open does not become a preview; shijimi just switches to its tab.
- Files opened from anywhere other than the file tree, such as Quick Open, search results, or CHANGES in diff mode, open in normal tabs.

### Git status markers

| Character | Meaning |
| --- | --- |
| `M` | Modified |
| `U` | Untracked |
| `A` | Added (staged) |
| `D` | Deleted |
| `R` | Renamed |
| `•` | Folder that contains changed files |

## Searching in a file

| Method | Action |
| --- | --- |
| `/` / `?` (Helix mode) | Search forward / backward. `n` / `N` for next / previous |
| `Ctrl+F` (VS Code mode) | Open the search field. The selected text (within one line) becomes the initial value |

## Searching and replacing in the project

Any of the following opens the same floating window.

- `Space /` (Helix mode)
- "Search: Search / replace in the project" in the command palette
- `:grep`
- `:replace`

The window is laid out as follows.

- Top: the conditions. Four fields (search, replace, include, exclude) and toggles.
- Bottom left: the matching lines (Matched).
- Bottom right: the surroundings of the selected line (View). When the replace field has text, a `-` / `+` diff is also shown.

When you change the conditions, it searches again automatically after a short delay.

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Next / previous field |
| `↑` `↓` / `Ctrl+P` `Ctrl+N` / `PgUp` `PgDn` | Select a matching line |
| `Enter` | Open the selected line in its file |
| `Ctrl+X` (`Alt+X`) | Exclude / include the selected line from the replacement. Excluded lines are marked with `x` |
| `Alt+C` | Match case |
| `Alt+W` | Match whole words |
| `Alt+R` | Regular expression |
| `Alt+P` | Preserve case when replacing |
| `Alt+1` | Replace only the selected line (the panel stays open and moves to the next match) |
| `Alt+Enter` | Replace all target lines |
| `Esc` | Close |

### What is searched

- Text files under Git control are searched. Untracked files that are not ignored are included.
- Outside Git, everything is searched except build directories.
- Unsaved content of open buffers is also searched.

### Include and exclude

In the "include" and "exclude" fields, write comma-separated globs (for example: `*.lock,dist/**`).
The initial values come from the `[search]` setting.

With `:grep` and `:replace`, you can also give them as arguments. `-x` can also be written as `--exclude=`, and `-i` as `--include=`.

```text
:grep -x '*.lock,dist/**' -i '*.go' foo
:replace foo => bar
```

- With arguments, the window opens with the values already in the fields.
- If you type just `:grep` / `:replace`, it opens with the previous conditions.

Put settings that should always apply in `[search]`.

```toml
[search]
exclude = ["*.lock", "dist/**"]
include = []
```

### Replacing

Type text in the replace field and press `Alt+Enter` to run.

- Replaced files are opened as buffers. The disk does not change until you save. You can undo with `u` / `Ctrl+Z`.
- When regular expressions are enabled, you can use captures such as `$1` in the replace field. When disabled, `$` is a literal character.
- If there are more than 5000 results, you cannot replace all until you narrow the conditions.

## Reflecting external changes

When another process, such as an AI agent, changes a file, shijimi reloads it automatically.

- Files with unsaved edits are not reloaded. Run `:reload` to reload them.
- To turn this off, set `watch-files = false` under `[editor]` ([Configuration files](config.md)).
