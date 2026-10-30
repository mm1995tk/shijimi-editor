# Terminal

There is a terminal panel at the bottom of the screen.

## Opening and closing

| Method | Action |
| --- | --- |
| ``Ctrl+` `` | Toggle the panel |
| `Ctrl+J` | Toggle the panel (only when the terminal does not have focus) |
| `Space t` (Helix mode) | Toggle the panel |
| `:sh <command>` | Open the terminal and run the command |

## What runs inside

- A login shell (`$SHELL -l`) runs on a PTY. It reads `.zshrc` and so on.
- Full-screen programs such as `vim`, `top` and `lazygit` work too.
- Scroll the mouse wheel to go back through past output.

## Keys

When the terminal has focus, most keys are passed to the shell.
The keys that shijimi handles are:

| Key | Action |
| --- | --- |
| `Ctrl+Q` | Quit shijimi |
| ``Ctrl+` `` | Close the terminal |
| `Alt+R` | Resize mode ([Understanding the screen](screen.md)) |
| `F1` | Command palette |
| `F2` | New terminal tab |
| `Ctrl+PgUp` / `Ctrl+PgDn` | Switch terminal tabs |

When the shell exits, its tab is closed.

## Copying

### Selecting with the mouse

Drag with the left mouse button inside the terminal to select the characters in that range.
When you release the button, the selected characters go to the clipboard.

- While a program that uses the mouse (`vim`, `htop`, tmux and so on) is running, drag while holding `Shift`.
- If you drag the pointer above or below the terminal, you can go back through past output while selecting.
- Pressing a key or clicking inside the terminal clears the selection.

### Copying from a program

When a program in the terminal (Claude Code, tmux and so on) copies with OSC 52, the text goes to shijimi's clipboard.

- When a copy happens, the status bar shows "Copied N characters from the terminal".
- It is also sent to the outer terminal with OSC 52. Even over SSH, it reaches the clipboard on your local machine (if the outer terminal supports OSC 52).
- Copies larger than about 1.5 MB are not accepted. An error appears in the status bar.

## Inside herdr

When shijimi runs in the [herdr](https://herdr.dev) terminal multiplexer (`HERDR_ENV=1`) and `[experimental]` `herdr` is `true` ([Configuration files](config.md#nightly-build)), the terminal commands use a pane of herdr instead of the terminal panel. herdr then detects the state of the agents run there, notifies you and restores them. Without `HERDR_ENV=1`, or with `herdr = false`, nothing changes.

| Method | Action |
| --- | --- |
| ``Ctrl+` `` / `Space t` / `:sh` | The first time, create a pane right below shijimi's pane and move focus there. After that, hide the pane, or show it again (and move focus there) |
| `Ctrl+J` | Show the pane and move focus there |
| `:sh <command>` | Show the pane and run the command in it |
| `Space a` / `Alt+A` | Paste the context into the pane (Enter is not sent) |
| `shj exec ":sh hide"` | Hide the pane. Run it in the pane; bind it to a key of herdr |

- The pane starts in the root of the workspace, with the same environment variables as the terminal panel, so `claude` started there connects to the IDE integration.
- To end the pane, type `exit` in its shell or use herdr's key (`Ctrl+B x`). After that, the next ``Ctrl+` `` creates a new pane. When shijimi quits, it closes the pane it created.
- To go back to shijimi from the pane, use herdr's key (`Ctrl+B` `k` by default). shijimi's own keys (`Ctrl+Q`, ``Ctrl+` ``, `Alt+R`, `F1`, `F2` and `Ctrl+PgUp` / `Ctrl+PgDn`) do not work in the pane.
- The screen, the scrolling, the copying with the mouse and OSC 52 in the pane are herdr's. The message "Copied N characters from the terminal" does not appear.
- The shell of the pane reports the exit code of each command with `shj command-end <code>`, so the suggestion to investigate a failed command still appears. This works for zsh and bash. For bash, the hook is set through `PROMPT_COMMAND`, so it does not work when a startup file replaces `PROMPT_COMMAND`.
- When a herdr command fails, the error is shown in the status bar.

## Environment variables

`SHIJIMI_SOCKET` is set in the shell of the terminal panel.
You can use it to control shijimi from the shell.
When it is not set, `shj` looks for the running shijimi by itself (see [AI agent integration](agent.md#controlling-shijimi-from-the-shell)).
`CLAUDE_CODE_SSE_PORT`, `ENABLE_IDE_INTEGRATION=true` and `FORCE_CODE_TERMINAL=true` are also set, for the Claude Code IDE integration.
For details, see [AI agent integration](agent.md).
