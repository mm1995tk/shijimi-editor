# Preview

## Markdown

In a `.md` file, run one of the following to open a preview to the right of the buffer.

- `Space m` (Helix mode)
- `:preview`
- The command palette

Headings, lists, code and tables are formatted.
Metadata enclosed by `---` at the top of the file (front matter) is shown as a YAML code block.
Long lines in code blocks wrap at the width of the preview.
The preview follows your edits and scrolls to match the cursor position.
Run it again to show only the preview at full width. A third time closes it.
At full width, scroll with `j` / `k` or the mouse wheel.

## mermaid

mermaid code blocks are shown as diagrams.

- They are converted to PNG with [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) (`mmdc`).
- If `mmdc` is not on the PATH, or rendering fails, the source and the error are shown instead of the diagram.
- Rendered results are stored in `~/.cache/shijimi/mermaid`.

You can replace the command with `mermaid-command` in `[editor]`. `{in}` is the input file and `{out}` is the output PNG. Setting it to empty disables it.

```toml
[editor]
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"
```

## Images

Opening an image file in one of the following formats shows it as an image.

PNG / JPEG / GIF / BMP / WebP / TIFF

Images and mermaid diagrams are drawn with the Kitty graphics protocol. You need a terminal that supports it, such as Kitty, Ghostty or WezTerm.
