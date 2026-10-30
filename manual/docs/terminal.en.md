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

## Environment variables

`SHIJIMI_SOCKET` is set in the shell of the terminal panel.
You can use it to control shijimi from the shell.
When it is not set, `shj` looks for the running shijimi by itself (see [AI agent integration](agent.md#controlling-shijimi-from-the-shell)).
`CLAUDE_CODE_SSE_PORT`, `ENABLE_IDE_INTEGRATION=true` and `FORCE_CODE_TERMINAL=true` are also set, for the Claude Code IDE integration.
For details, see [AI agent integration](agent.md).
