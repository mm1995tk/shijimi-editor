# Diff mode

Diff mode switches the whole editor into a view for reviewing changes.
How you open it decides what you compare against (the comparison base).

| Key | Comparison base | Requires |
| --- | --- | --- |
| `Space G`, `Ctrl+G`, `:gitdiff` | `HEAD` | `git` |
| `Space g`, `:pr`, `:pr <number>` | The base of a pull request | `git`, `gh` (after `gh auth login`) |

- Pressing the same key while diff mode is open closes it.
- Pressing the other key switches the comparison base.
- With either base, you can edit and save the right side (the working tree file) in place.

## Screen

From top to bottom:

| Area | Content |
| --- | --- |
| Header | The base and the target. For a pull request, it also shows `PR #number`, the title, the state and the author |
| Left sidebar | The file tree in the upper 70%, CHANGES (the list of changed files) in the lower 30% |
| Center | The diff. A change-position bar runs along the right edge |
| Help line | The keys available now |
| Status line | The mode and messages |

- Files with no changes open in the normal editor.
- Files with changes open as a whole-file **Split** (the base on the left, the current file on the right).
- A new file (including untracked files when the base is `HEAD`) opens in a single editor, whether the base is `HEAD` or a PR base. Lines are not colored. The bar on the right edge is all green, and the file name row above shows `+N` / `-N`. When the base is `HEAD`, once you commit the file it is displayed like the other changed files.
- When the base is a PR base, PR comments also appear on the correct lines of a new file opened in a single editor. You can add comments and replies too.
- The diff updates immediately when you edit.
- CHANGES updates when you save, when an external change is detected, and every few seconds.

### CHANGES

It is shown as a folder tree, like the file tree.
A folder with only one child is collapsed into one row, such as `internal/config`.

A file row shows a status letter, the file name, the number of added lines `+N` and the number of removed lines `-N`.

| Letter | Status |
| --- | --- |
| `M` | Modified |
| `A` | Added (including untracked) |
| `D` | Deleted |
| `R` | Renamed |

The right end of the heading shows the current base. `DIFF` is `HEAD`, and `PR` is the base of a pull request.

When you open a file from CHANGES, the cursor is placed on the first changed line of the file.
For a change that only removes lines, it is placed on the line after it.
A file already open in a tab keeps its cursor position.

### Split and Unified

Switch with any of these:

- `s` in CHANGES
- `:gitdiff split`, `:gitdiff unified`
- Click the file name row above the diff (for a new file opened in a single editor there is nothing to switch to, so `[split ⇄ unified]` at the right of the row is not shown and clicking does nothing)

You can edit in Unified too. Deleted lines are interleaved as read-only lines.

### Change-position bar

One column at the right edge of the diff shows where the changes are in the whole file.

| Color | Kind of change |
| --- | --- |
| Green | Additions only |
| Red | Deletions only |
| Blue | Rewrites (lines exist on both sides). Also blue when one cell mixes several kinds |

- The height of the bar corresponds to the whole file. When the file is longer than the screen, one cell groups several lines.
- The range currently displayed has a lighter bar background.
- Clicking the bar scrolls to that position.

### Intra-line diff

On a line where only part of it changed, the changed part is highlighted in a color stronger than the line background.
The left (base) uses the deletion color, and the right (current file) uses the addition color.

- The comparison is between the lines paired side by side in Split. Unified uses the same pairs.
- A pair with less than half of its characters in common is treated as unrelated, and only the whole-line color is used.
- Lines longer than 400 characters use only the whole-line color.
- When very many lines changed, the amount compared at once is limited. The remaining lines are compared little by little on each edit or refresh.

## Comparison base

| Base | Content |
| --- | --- |
| `HEAD` | Compares `HEAD` with the working tree. Includes untracked files |
| The base of a pull request | Compares `git merge-base <PR base> HEAD` with the working tree |

Comparing with the base of a pull request gives the diff shown on GitHub's PR page plus your uncommitted changes.
Even if you merge the base branch, the diff shows only your own changes.

The pull request opened with `Space g` is chosen as follows:

- If the current branch has exactly one pull request, it opens that one without showing a list.
- If it has several, you choose from a list of only those pull requests.
- If it has none, you choose from the repository's pull request list.
- `:pr <number>` opens the pull request with that number. `:pr` alone lets you choose from the repository's list.

### A pull request on a different branch

If you choose a pull request that differs from the current branch, shijimi runs `gh pr checkout <number>` and then opens it.

- If there are uncommitted changes, the status line asks "Stash the uncommitted changes and switch? (y/n)".
    - `y`: runs `git stash push -u -m "shijimi: changes before switching to PR #<number>"`, then checks out.
    - `n`: cancels the switch.
- If `gh pr checkout` fails and you stay on the original branch, the stashed changes are restored with `git stash pop`. If they cannot be restored, the status line tells you the changes are in the stash.
- If there are unsaved files, it does not switch. Save them first.
- If there are unsent comments for another pull request, it asks "PR #N has M unsent comments. Discard them and open PR #N? (y/n)". Merely switching the base to `HEAD` does not discard unsent comments.
- In other situations (switching the base, closing diff mode, quitting shijimi), it never stashes or commits automatically.

### PR and local divergence

When the local `HEAD` differs from the latest commit of the PR, the status line shows the following:

| State | Display | `r` in CHANGES |
| --- | --- | --- |
| The PR is ahead | "The PR was updated. Press r to pull" | Runs `git pull --ff-only`, then reloads |
| The local branch is ahead | "There are N unpushed commits" | Reloads only |
| Both are ahead | "The local branch and the PR have diverged" | Reloads only (does not pull) |

To compare, it runs `git fetch` for the latest commit of the PR and the base branch.

## Review

When the base is a pull request base, `Space v` opens the review menu.
The right side is an editor, so review operations are gathered in this menu to avoid clashing with editing keys.

| Key | Action |
| --- | --- |
| `Space v c` | Comment on the cursor line or the selected range |
| `Space v C` | Comment on the whole current file |
| `Space v t` | Open the thread on the cursor line as a card. If there is none on the line, choose from the file's comment list |
| `Space v v` | Mark as viewed / unmark |
| `Space v A` | Approve and submit |
| `Space v R` | Request changes and submit |
| `Space v S` | Submit as comments only |

- The range is the editor selection (made with `x` and so on) as is.
- With the VS Code keymap, the same operations are available in the command palette (`F1`) under "Review:".
- When CHANGES has focus, `Space v C`, `t` and `v` act on the selected file.

### Files you can comment on

GitHub comments attach to line numbers of the PR's latest commit.
If you edit or commit locally, the working tree's line numbers drift from them.
So you can comment only on **files whose working tree content is the same as the PR's latest commit**. The same applies to applying suggestions.

Files whose content differs are displayed as follows:

- A `✎` appears next to the file name in CHANGES and in the file name row above the diff.
- Existing comments are not shown beside lines. The file name row summarizes them as "● 3 comments (not shown on lines; list with Space v t)".
- If you try to comment, the status line says "This file has local changes, so you cannot comment on its lines".

This is re-evaluated on every save and every reload. If you revert the local changes, you can comment again.
You cannot comment on files with unsaved changes either.

GitHub accepts comments only around the diff (the changed lines and 3 lines before and after). On other lines, the status line says so.

### Showing comments

| Place | Display |
| --- | --- |
| Left of the line number | `●` on lines with comments |
| File name row above the diff | "● 2 file comments" if there are comments on the whole file |
| CHANGES row | `✓` (viewed), `●N` (number of comments) |
| CHANGES heading | `viewed 2/5` (`✓2/5` when narrow) |
| Right of the status line | "3 unsent comments" |

### Cards

Comments are shown in a card, similar to the LSP hover.
Open it by clicking `●` or with `Space v t`.

It also opens when you hover the mouse over `●` (when `mouse-hover` is enabled). This card is read-only and takes no keys. Pressing a key closes it and passes that key to the editor. It also closes when the pointer leaves the card.

- Comments are rendered as Markdown.
- Mermaid code blocks are rendered as diagrams (using the same component as the Markdown preview; without `mermaid-command`, the source is shown).

While a card is open, it receives the following keys:

| Key | Action |
| --- | --- |
| `r` | Reply (edit, for an unsent comment) |
| `x` | Resolve / mark unresolved (delete, for an unsent comment) |
| `s` | Apply the suggestion to the local file and save |
| `Enter` | Move the cursor to that line |
| `j` / `k`, `Ctrl+D` / `Ctrl+U` | Scroll |
| `Esc` | Close the card |

Any other key closes the card and is passed to the editor.

### Writing comments

The body of a comment, reply or review is written in an input box. Below the input box is a preview that looks like a card.

| Key | Action |
| --- | --- |
| `Enter` | Submit (a comment goes into the unsent list) |
| `Shift+Enter` / `Alt+Enter` / `Ctrl+J` | New line |
| `Ctrl+S` | Insert a ```` ```suggestion ```` block containing the original lines of the selected range |
| `Esc` | Cancel |

- Comments on lines, ranges and files are not sent immediately. You send them together as a review with `Space v A` / `R` / `S`.
- Replies and resolving are sent immediately.
- When sending fails, the partially built review on GitHub is deleted and the unsent comments are kept.
- If your own draft review (pending) remains on GitHub, shijimi does not send. Submit or discard it on GitHub's PR page.
- For comments on the left (base) side of a file and for outdated comments, `Enter` only opens the file and does not move the cursor.

## CHANGES keys

| Key | Action |
| --- | --- |
| `j` / `k` | Move (stops on folder rows too; skips the inside of closed folders) |
| `g` / `G` | Top / bottom |
| `h` | Close the folder. On a closed folder or a file row, moves to the parent folder |
| `Enter` | Open the file. On a folder row, toggle open/closed |
| `l` | Open the file. On a folder row, open it |
| `s` | Toggle split / unified |
| `r` | Refresh the list (for a pull request, it may pull as in the table above) |
| `Space G` / `Space g` | Switch the comparison base, or close |
| `Space v` | Review menu |
| `Tab` | To the file tree |
| `Esc` | To the editor |

## Mouse

- Clicking a folder row in CHANGES toggles it (`▾` when open, `▸` when closed). The open/closed state is kept across list refreshes and base switches.
- Dragging the border between the sidebar and the diff changes the sidebar width.
- Dragging the border between the file tree and CHANGES (the CHANGES heading row) changes the ratio of their heights. Pressing `=` in the `Alt+R` resize mode restores 7:3.
- In split view, dragging the `│` between left and right changes the widths of the base and the working tree. The widths are kept until you quit the editor, and pressing `=` in the `Alt+R` resize mode restores an even split. It has no effect in unified view.
