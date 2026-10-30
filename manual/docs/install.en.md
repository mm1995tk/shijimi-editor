# Installation and startup

## Installation

Run one of the following.

```sh
nix run github:mm1995tk/shijimi-editor/release   # run it right away without installing
nix profile install github:mm1995tk/shijimi-editor/release
go install github.com/mm1995tk/shijimi-editor/cmd/shj@latest      # installs the shj command
go install github.com/mm1995tk/shijimi-editor/cmd/shijimi@latest  # installs the same thing as shijimi
npm install -g shijimi-editor                                     # installs both shj and shijimi (Linux and macOS)
```

All of them get the latest release: the Nix commands follow the `release` branch, `go install` takes the newest tag, and npm takes the binaries published at release time. Unreleased changes on `main` are not included.

To try the unreleased changes on `main`, install the nightly build. It installs as `shj-nightly` (and `shijimi-nightly`) next to `shj`, reads `~/.config/shijimi-editor-nightly/` ([Configuration files](config.md#nightly-build)), uses its own socket directory, and has the `[experimental]` settings (features being tried out; none yet) on by default. Nix builds it from the newest `main` at install time; npm and a GitHub pre-release are planned.

```sh
nix run github:mm1995tk/shijimi-editor#shj-nightly
nix profile install github:mm1995tk/shijimi-editor#shj-nightly
```

To install it with home-manager, see [Nix and home-manager](nix.md).

## Requirements

shijimi alone is enough for editing. The following features need other commands.

| Feature | Required command |
| --- | --- |
| Comparing with the base of a pull request in diff mode | [gh](https://cli.github.com/). Run `gh auth login` first |
| Git status in the file tree, diff mode | `git` |
| LSP | A language server for each language (`gopls`, `rust-analyzer`, and so on) |
| mermaid diagrams | [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) (`mmdc`) |
| Writing the configuration in Nix | `nix` |
| Copying to and pasting from the system clipboard (`Space y` / `Space p`) | `wl-copy` and `wl-paste` on Wayland, `xclip` or `xsel` on X11, `pbcopy` and `pbpaste` on macOS (included with macOS). If none is available, only shijimi's internal clipboard is used |
| Opening the URL under the cursor in an external browser (`Space o`) | `xdg-open` on Linux, `open` on macOS (included with macOS) |
| Icons in the file tree | A terminal configured with a Nerd Font |

## Starting

```sh
shj            # open the current directory
shj ./src      # open a directory
shj main.go    # edit only that file (single-file mode)
```

`shijimi` works the same as `shj` (installing with Nix gives you both).

When you give it a file, shijimi opens in single-file mode, which edits only that file. It is meant to be called from `$EDITOR`. It does not start the file tree, LSP, terminal, diff mode, suggestions panel or the Claude Code IDE integration, and it does not use tabs. When you close the file with `:q` or `:wq` (`Ctrl+W` or `Ctrl+Q` in VS Code mode), shijimi exits, so the caller can wait for it to finish and move on. A path that does not exist is opened as an empty buffer, and the file is created when you save. Giving a directory behaves as before.

## Quitting

Press `Ctrl+Q`. In Helix mode you can also quit with `:q`.

!!! note "Files with the same name as a subcommand"
    When the first argument is `diff` or `skill`, it is treated as a subcommand.
    To open a file with the same name, write it like `./diff`.
    The subcommands are described in [AI agent integration](agent.md).
