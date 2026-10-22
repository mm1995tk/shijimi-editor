# shijimi

shijimi is a terminal-based integrated development environment (TUI). The command is `shj`.

- The screen layout follows **VS Code**.
- For editing, you can choose between Helix-style modal editing and VS Code-style modeless editing.

## Features

| Feature | Description | Page |
| --- | --- | --- |
| Modal editing | Helix-style normal / insert / select modes. Multiple cursors | [Editing (Helix mode)](editing-helix.md) |
| File tree | Shows the Git status as M / U / A | [Files and search](files.md) |
| Search and replace | Search and replace across the whole project (floating window) | [Files and search](files.md) |
| LSP | Diagnostics, completion, hover, signature help, go to definition, references, rename, code actions, symbol list, formatting | [LSP](lsp.md) |
| Diff mode | Edit while viewing whole files as diffs. The base is `HEAD` or the base of a pull request | [Diff mode](review.md) |
| Preview | Markdown (including mermaid) and images | [Preview](preview.md) |
| Terminal | A terminal panel at the bottom of the screen. You can open several in tabs | [Terminal](terminal.md) |
| Browser | Open a URL in an external browser. Port forwarding | [External browser and port forwarding](browser.md) |
| Agent integration | Claude Code IDE integration. Pass the selection to an AI agent | [AI agent integration](agent.md) |

## Where to start

1. [Installation and startup](install.md)
2. [Understanding the screen](screen.md)
3. [Editing (Helix mode)](editing-helix.md). If you prefer VS Code-style operation, see [Editing (VS Code mode)](editing-vscode.md)
