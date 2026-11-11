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

When shijimi runs in a terminal multiplexer, `[experimental]` `multiplexer` is `true` ([Configuration files](config.md#nightly-build)) and the multiplexer is supported, the terminal commands use a pane of the multiplexer instead of the terminal panel. The multiplexer then detects the state of the agents run there, notifies you and restores them. Which multiplexer is used is set by `[terminal]` `multiplexer` (`"auto"` by default, which takes the innermost multiplexer among the parent processes of shijimi; `"none"` always uses the terminal panel). [herdr](https://herdr.dev) (`HERDR_ENV=1`) , tmux 3.1 or later (`TMUX` set) and zellij 0.44 or later (`ZELLIJ` set) and cmux on macOS (`CMUX_WORKSPACE_ID` and `CMUX_SURFACE_ID` set) are supported. Without a supported multiplexer, or with `multiplexer = false` or `"none"`, nothing changes. The rest of this section describes herdr, tmux, zellij and cmux; they work the same way except where noted.

| Method | Action |
| --- | --- |
| ``Ctrl+` `` / `Space t` / `:sh` | The first time, create a pane right below shijimi's pane and move focus there. After that, hide the pane, or show it again (and move focus there) |
| `Ctrl+J` | Show the pane and move focus there |
| `:sh <command>` | Show the pane and run the command in it |
| `Space a` / `Alt+A` | Paste the context into the pane (Enter is not sent) |
| `shj exec ":sh hide"` | Hide the pane. Run it in the pane; bind it to a key of the multiplexer |

- The pane is hidden by zooming shijimi's pane (in tmux, `resize-pane -Z`; in zellij, `toggle-fullscreen`). cmux has no zoom: hiding only moves the focus back to shijimi, and the pane stays on the screen. In cmux the command that starts the pane's shell is typed into it (`cd <root> && exec env ...`), so it is visible. The new pane takes 30% of the height; in zellij and cmux, which cannot size a tiled pane, half of it. zellij names the pane by `--pane-id` in every action, so nothing depends on the focus. In tmux, a pane that ended but is kept by `remain-on-exit` counts as ended, and a new one is created.
- The pane starts in the root of the workspace, with the same environment variables as the terminal panel, so `claude` started there connects to the IDE integration.
- To end the pane, type `exit` in its shell or use the multiplexer's key (herdr: `Ctrl+B x`; tmux: its prefix then `x`; zellij: `Ctrl+P` then `x`; cmux: close the surface with its own key). After that, the next ``Ctrl+` `` creates a new pane. When shijimi quits, it closes the pane it created.
- To go back to shijimi from the pane, use the multiplexer's key (herdr: `Ctrl+B` `k` by default; tmux: its prefix then an arrow key, or `select-pane -U`; zellij: `Alt+Up` by default; cmux: its keys for moving between panes). shijimi's own keys (`Ctrl+Q`, ``Ctrl+` ``, `Alt+R` and `F1`) do not work in the pane.
- The screen, the scrolling, the copying with the mouse and OSC 52 in the pane are the multiplexer's. The message "Copied N characters from the terminal" does not appear.
- The shell of the pane reports the exit code of each command with `shj command-end <code>`, so the suggestion to investigate a failed command still appears. This works for zsh and bash. For bash, the hook is set through `PROMPT_COMMAND`, so it does not work when a startup file replaces `PROMPT_COMMAND`.
- When a command of the multiplexer fails, the error is shown in the status bar.

### In kisui-terminal

In the editor that runs in the primary terminal of a kisui-terminal workspace (the one `kisui serve` accepts as the primary), kisui-terminal itself is the multiplexer. The terminal commands then open another terminal of the same workspace, which kisui-terminal shows beside the primary, and move the focus to it. The keys and commands are the same as above, with these differences:

- The second ``Ctrl+` `` hides every terminal but the primary (the same as `Ctrl+Shift+M` in kisui-terminal), and the next one shows them again and moves the focus to the pane. You can have up to 6 terminals in a workspace, the pane included.
- The pane starts in the root of the workspace, with the same environment variables as the other multiplexers.
- When shijimi-editor quits, the panes it opened are closed. If you type `exit` in the pane, kisui-terminal closes it and the next ``Ctrl+` `` opens a new one.
- The text that the commands send to the pane (`:sh <command>`, `Space a`) is written to the terminal as it is. The lines kept for the failed command are taken from the output kisui-terminal has kept, so they can differ a little from the screen.
- An editor in any other terminal of kisui-terminal (a split terminal) is not the primary: it keeps the built-in terminal panel. `"auto"` chooses kisui-terminal in the primary even when `kisui serve` was started inside tmux; set `[terminal]` `multiplexer = "none"` to keep the terminal panel there too.
- In the primary, `[terminal]` `multiplexer` other than `"none"` (for example `"herdr"` left over from using shijimi-editor in another terminal) is read as `"auto"`: kisui-terminal is used. When the primary keeps the terminal panel because of `"none"` or `[experimental]` `multiplexer = false`, the status bar says so.

### Videos, images and PDFs in kisui-terminal

A terminal cannot play a video, but kisui-terminal can. In the editor that runs in the primary terminal of a kisui-terminal workspace, opening a video file (pressing it in kisui-terminal's file tree, `:open <file>`, the command palette and so on) does not read it as text. kisui-terminal opens a video pane beside the terminals and plays it there. This works for a file on another machine too: the video travels over the same ssh connection as kisui-terminal.

- The extensions are `.mp4`, `.m4v`, `.mov`, `.webm`, `.ogv` and `.mkv`. Which of them play depends on the codecs of kisui-terminal's web view; a video that cannot be played says so in the pane.
- The pane is one of the terminals' cells (up to 6 cells in a workspace, the primary and the panes included). It is not a tab. A workspace has one video pane: opening another video replaces it. Opening a video moves the keys to the pane.
- The video is fetched in full before it plays, so a video of more than 512 MiB is not opened (the pane says so).
- Keys, while the keys are in the pane: Space or `k` plays and pauses, `←` and `→` seek 5 seconds, `j` and `l` 10 seconds, `m` mutes. kisui-terminal's own keys (`Ctrl+Shift+arrows`, `Ctrl+Shift+M`, `Cmd+W` on a Mac) work as in the terminals.
- Close the pane with the × in its title, or with `Cmd+W` on a Mac. It also closes when the editor in the primary terminal quits.
- Images and PDFs take the same road and open in a pane too (every way of opening a file does, as with a video: the file tree, `:open`, the command palette and so on). The image extensions are `.png`, `.jpg`, `.jpeg`, `.gif`, `.bmp`, `.webp` and `.avif`; the PDF extension is `.pdf`. A workspace has one pane of each kind: opening another file of the same kind replaces it. The size limits and the keys inside a pane are kisui-terminal's business. When a pane cannot be opened (a file over the limit, for example), the status line says why. `.tif` and `.tiff` images are still shown inside the editor.
- The editor outside kisui-terminal, or in any other terminal of it (a split terminal), is not the primary and uses no panes: images are shown inside the editor as before, and a PDF is opened like any other file.

## Environment variables

`SHIJIMI_SOCKET` is set in the shell of the terminal panel.
You can use it to control shijimi from the shell.
When it is not set, `shj` looks for the running shijimi by itself (see [AI agent integration](agent.md#controlling-shijimi-from-the-shell)).
`CLAUDE_CODE_SSE_PORT`, `ENABLE_IDE_INTEGRATION=true` and `FORCE_CODE_TERMINAL=true` are also set, for the Claude Code IDE integration.
For details, see [AI agent integration](agent.md).
