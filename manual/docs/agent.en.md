# AI agent integration

There are two ways to connect shijimi with an AI agent.

- **Suggestion panel and ACP**: The panel on the right lists actions that fit the state of your work. When you choose one, it asks the agent. The agent's reply and changes come back to the panel and to diff mode.
- **Terminal panel**: Exchange information with an agent running in the terminal panel (such as Claude Code) through the IDE integration or by pasting. This works even with agents that do not support ACP.

## Suggestion panel

The right panel lists "recommended actions" that fit the current state.
Suggestions are decided by rules. No AI is used, so they appear instantly and cost nothing.

### Open and close

| Key | Action |
| --- | --- |
| `Alt+I` / `Space i` / `:agent` | Move to the panel (opening it if closed). Closes it when you are already in the panel |
| `Esc` / `Tab` | Return to the editor (the panel stays open) |

- It is closed at startup. Open it with `Alt+I`.
- When the panel is not shown and there are suggestions, the status line shows "Suggestions N (Alt+I)".
- Dragging the border between the panel and the editor with the mouse changes the width.

### Keys in the panel

| Key | Action |
| --- | --- |
| `j` / `k` | Select a suggestion |
| `Enter` | Run the selected suggestion |
| `x` | Dismiss the suggestion (it is not shown again while the same condition continues) |
| `y` / `n` | Answer the agent's permission request |
| `c` | Cancel the agent's work |
| `Ctrl+U` / `Ctrl+D` | Scroll the reply |

### Copying the reply text

Drag with the left mouse button in the lower part of the panel (the agent's reply and log) to select text in that range.
When you release the button, the selected text goes to the clipboard, and the status bar shows "Copied N characters from the agent panel".
The selected text is displayed inverted, as in the terminal panel.

- Lines wrapped to fit the width are copied back as the original single line (the wrapping line breaks are not included).
- The selection continues even if the pointer leaves the panel during the drag. If it goes above or below the panel, the log scrolls.
- Scrolling with the wheel keeps the selected text as is.
- Pressing a key or clicking elsewhere in the panel clears the selection.
- A click without dragging does not copy. As before, focus moves to the panel.

### List of suggestions

| Condition | Suggestion | What happens when you run it |
| --- | --- | --- |
| An investigation of a failure produced a proposed fix | Apply the fix? | Asks for the fix, then switches to diff mode when done |
| There are files with conflict markers (`<<<<<<<`) | Resolve the conflicts? | Asks for the resolution, then switches to diff mode when done |
| There are commits and the branch has no PR | Create a PR? | Asks for the PR to be created, then switches to diff mode when done |
| The branch has a PR | Switch to review mode? | Compares against the PR base in diff mode (does not use the agent) |
| A command in the terminal panel failed | Investigate the cause of the failure? | Sends the terminal output and shows the cause and a proposed fix in the panel |
| LSP errors remain | Fix N errors? | Sends the list of errors and asks for a fix, then switches to diff mode when done |
| Viewing a PR in diff mode | Have the AI review this PR? | Sends the diff and places the proposed comments as **unsent comments** |

- When the condition goes away, the suggestion goes away.
- "Conflicts" means files that git reports as unmerged, and files containing both a line starting with `<<<<<<<` and a line starting with `>>>>>>>`.
- "Create a PR?" appears when there are commits ahead of the default branch (`origin/HEAD`, or `main` / `master` if absent). Whether a PR exists is checked with `gh pr list`. Without `gh`, suggestions related to PRs are not shown.
- "A command failed" means either of the following:
    - The exit code is non-zero (except 130, from stopping with `Ctrl+C`). The exit code is available in zsh and bash (see "Shell integration" below).
    - A test failure line (`FAIL`, `--- FAIL:`, `FAILED`, `test result: FAILED` and so on) appeared. This works in any shell.
- When the next command in the same tab succeeds, that tab's failure suggestion goes away.
- If you dismiss it with `x`, failure suggestions are not shown for that tab until a command there succeeds.

### AI review

When you run "Have the AI review this PR?", the PR's diff is given to the agent, which returns its findings as JSON.
The findings are placed as **unsent comments**, the same as comments you write yourself (the author is "AI").
Nothing is sent to GitHub. Read them, delete, fix or add as needed, and then send with `Space v A` / `R` / `S`.

- Findings on lines outside the diff become comments on the file. The line number is put at the start of the body.
- Findings on files that are not in the PR, and on files with local changes, are not placed (the count is shown in the status line).

## ACP (Agent Client Protocol)

Running a suggestion is handed to the agent through [ACP](https://agentclientprotocol.com).
The agent starts the first time you run a suggestion, and the same session is used until you quit shijimi.

### Configuration

Write the command that starts the agent in `~/.config/shijimi-editor/config.toml`.

```toml
[agent]
command = "claude-agent-acp"  # Example: Claude Code (@agentclientprotocol/claude-agent-acp on npm)
args = []
environment = { }             # Environment variables passed to the agent
```

- For example, for Gemini CLI, use `command = "gemini"` and `args = ["--experimental-acp"]`.
- `[agent]` is read only from `~/.config/shijimi-editor/`. It is not used if written in a project's `.shijimi-editor/`. This prevents a cloned repository from deciding which program is launched.
- If it is not configured, running a suggestion shows the location of the setting in the status line.

### What the agent does

- **Reading files**: For open files, the buffer content is passed (including unsaved edits). Files that are not open are read from disk.
- **Writing files**: Writes to disk. Open buffers are reloaded as external changes. You can check them in diff mode. It does not write to files with unsaved edits (it returns an error to the agent).
- During "Investigate the cause of the failure?" and the AI review, write requests are refused. This keeps the agent from writing even if the terminal output or the PR diff contains text urging it to rewrite files.
- **Asking for permission**: A card appears in the panel. `y` allows and `n` denies. If the panel is closed, it opens.
- **Progress and replies**: Streamed into the panel. Tool calls are shown as `○` (waiting), `◐` (running), `●` (done) and `✗` (failed).

| Command | Action |
| --- | --- |
| `:agent` | Move to / close the panel |
| `:agent cancel` | Cancel the work (same as `c` in the panel) |
| `:agent stop` | Stop the agent. Running a suggestion next time starts it anew |

### Shell integration

When the shell in the terminal panel is zsh or bash, shijimi automatically adds setup that reports the exit code of each command to shijimi (OSC 133;D).

- zsh: A temporary `.zshenv` is loaded as `ZDOTDIR`. It restores the original `ZDOTDIR`, loads the user's `.zshenv`, and then adds `precmd` / `preexec` hooks. `.zshrc` and the rest are loaded from the user's files as usual.
- bash: A temporary file is loaded with `--rcfile`. It reads the same files as a login shell (`/etc/profile`, and the first of `~/.bash_profile`, `~/.bash_login` and `~/.profile` that exists; `~/.bashrc` if none exist), and adds a hook at the start of `PROMPT_COMMAND`.
- Other shells (such as fish) can be used if you set them up to emit OSC 133;D yourself (for example, run `printf '\e]133;D;%s\a' $?` before the prompt).

## Claude Code IDE integration

shijimi supports Claude Code's IDE integration protocol, the same protocol used by the VS Code extension and claudecode.nvim.

- On startup, it starts a WebSocket MCP server on a free port of `127.0.0.1`.
- It writes the connection target and an authentication token to the lock file `~/.claude/ide/<port>.lock` (permission 0600). If `CLAUDE_CONFIG_DIR` is set, it writes under `ide/` there. The file is removed on exit.
- When you start `claude` in shijimi's terminal panel, it connects automatically.
- Claude Code running in an outside terminal can be selected with `/ide` if its working directory is under shijimi's root.
- Connections without the authentication token, and connections from a browser (with an `Origin` header), are rejected.

### Tools available to the agent

| Tool | Content |
| --- | --- |
| `getCurrentSelection` / `getLatestSelection` | The selection (or the cursor position if none) and the file |
| `getOpenEditors` / `getWorkspaceFolders` | Open files / roots |
| `getDiagnostics` | LSP diagnostics. All files if the URI is omitted |
| `openFile` | Open a file. Select a range with `startText` / `endText` |
| `openDiff` | Show a change as a diff. **Enter approves, Esc rejects** |
| `closeAllDiffTabs` / `close_tab` | Reject and close pending diffs |
| `checkDocumentDirty` / `saveDocument` | Check whether there are unsaved changes / save |

- Even if you approve in `openDiff`, shijimi does not write the file. Claude Code writes after the approval.
- Whenever the selection or the cursor moves, shijimi sends `selection_changed`.

## Sending the current context to the agent

Run it with any of these:

- `Space a` (Helix mode)
- `Alt+A`
- The command palette

The selection (or the cursor line if nothing is selected) is formatted as follows:

1. `path:start-line-end-line`
2. The selected text (a code block)
3. Diagnostics (errors, warnings and so on) that touch those lines

When connected through the IDE integration, the formatted text is sent to Claude Code as `at_mentioned`.
When not connected, it appears in a preview.

| Key | Action |
| --- | --- |
| `Enter` | Paste into the terminal panel (does not send `Enter`) |
| `Esc` / `q` | Cancel |
| `j` / `k` | Scroll the preview |

- Available only for saved files.
- Parts beyond 200 lines are omitted.

## Controlling shijimi from the shell

On startup, shijimi creates a Unix socket (permission 0600).
Its location is placed in the environment variable `SHIJIMI_SOCKET` of the shell in the terminal panel.
The following command is available in that shell. Only operations that the IDE integration lacks are kept.

When `SHIJIMI_SOCKET` is not set, `shj` looks for a running shijimi by itself.
The socket is `$XDG_RUNTIME_DIR/shijimi-<uid>/<pid>.sock` (`shijimi-nightly-<uid>` for the nightly build; under the temporary directory when `XDG_RUNTIME_DIR` is not set).
shijimi started inside the herdr terminal multiplexer (`HERDR_ENV=1`) also links it as `herdr-<HERDR_WORKSPACE_ID>/<HERDR_PANE_ID>.sock` in that directory.
Inside a herdr workspace (`HERDR_WORKSPACE_ID` set), `shj` uses the link of its pane `HERDR_PANE_ID`, or the only shijimi running in the workspace.
Outside herdr, it uses the only shijimi running for your user.
Sockets that no longer accept a connection are skipped, and shijimi removes its own files when it quits.
When none is found, or more than one, `shj` writes an error and exits with code 1. Set `SHIJIMI_SOCKET` to choose one.
This lets a shell command bound to a key of herdr run `shj exec ":sh hide"`, for example.

| Command | Action |
| --- | --- |
| `shj diff [path]` | Switch to diff mode (base `HEAD`). With a path, opens that file in diff view |

!!! note "Removed commands"
    `shj open` and `shj status` were removed. They were replaced by the IDE integration's `openFile` and `getOpenEditors` / `getCurrentSelection`.

## Installing the skill

```sh
shj skill install [destination]
```

This writes a skill (`SKILL.md`) that describes how to use `shj diff`.
If you omit the destination, it writes to `~/.claude/skills/shijimi/`.
Claude Code reads this skill and becomes able to open diffs in shijimi.
