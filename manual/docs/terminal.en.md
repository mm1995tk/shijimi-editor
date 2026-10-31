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

There is one terminal. For several, use a multiplexer such as herdr or tmux. When the shell exits, the panel closes, and the next time you open it a new shell starts.

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

## Inside a terminal multiplexer

When shijimi runs in a terminal multiplexer, `[experimental]` `multiplexer` is `true` ([Configuration files](config.md#nightly-build)) and the multiplexer is supported, the terminal commands use a pane of the multiplexer instead of the terminal panel. The multiplexer then detects the state of the agents run there, notifies you and restores them. Which multiplexer is used is set by `[terminal]` `multiplexer` (`"auto"` by default, which takes the innermost multiplexer among the parent processes of shijimi; `"none"` always uses the terminal panel). [herdr](https://herdr.dev) (`HERDR_ENV=1`) and tmux 3.1 or later (`TMUX` set) are supported so far; zellij and cmux will follow. Without a supported multiplexer, or with `multiplexer = false` or `"none"`, nothing changes. The rest of this section describes herdr and tmux; they work the same way except where noted.

| Method | Action |
| --- | --- |
| ``Ctrl+` `` / `Space t` / `:sh` | The first time, create a pane right below shijimi's pane and move focus there. After that, hide the pane, or show it again (and move focus there) |
| `Ctrl+J` | Show the pane and move focus there |
| `:sh <command>` | Show the pane and run the command in it |
| `Space a` / `Alt+A` | Paste the context into the pane (Enter is not sent) |
| `shj exec ":sh hide"` | Hide the pane. Run it in the pane; bind it to a key of the multiplexer |

- The pane is hidden by zooming shijimi's pane (in tmux, `resize-pane -Z`). The new pane takes 30% of the height. In tmux, a pane that ended but is kept by `remain-on-exit` counts as ended, and a new one is created.
- The pane starts in the root of the workspace, with the same environment variables as the terminal panel, so `claude` started there connects to the IDE integration.
- To end the pane, type `exit` in its shell or use the multiplexer's key (herdr: `Ctrl+B x`; tmux: its prefix then `x`). After that, the next ``Ctrl+` `` creates a new pane. When shijimi quits, it closes the pane it created.
- To go back to shijimi from the pane, use the multiplexer's key (herdr: `Ctrl+B` `k` by default; tmux: its prefix then an arrow key, or `select-pane -U`). shijimi's own keys (`Ctrl+Q`, ``Ctrl+` ``, `Alt+R` and `F1`) do not work in the pane.
- The screen, the scrolling, the copying with the mouse and OSC 52 in the pane are the multiplexer's. The message "Copied N characters from the terminal" does not appear.
- The shell of the pane reports the exit code of each command with `shj command-end <code>`, so the suggestion to investigate a failed command still appears. This works for zsh and bash. For bash, the hook is set through `PROMPT_COMMAND`, so it does not work when a startup file replaces `PROMPT_COMMAND`.
- When a command of the multiplexer fails, the error is shown in the status bar.

## Environment variables

`SHIJIMI_SOCKET` is set in the shell of the terminal panel.
You can use it to control shijimi from the shell.
When it is not set, `shj` looks for the running shijimi by itself (see [AI agent integration](agent.md#controlling-shijimi-from-the-shell)).
`CLAUDE_CODE_SSE_PORT`, `ENABLE_IDE_INTEGRATION=true` and `FORCE_CODE_TERMINAL=true` are also set, for the Claude Code IDE integration.
For details, see [AI agent integration](agent.md).
