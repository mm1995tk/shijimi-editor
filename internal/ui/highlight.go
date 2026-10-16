package ui

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

const maxHighlightLines = 20000

// chromaStyle approximates VS Code's Dark+ token colors.
var chromaStyle = chroma.MustNewStyle("dark-plus", chroma.StyleEntries{
	chroma.Text:                "#d4d4d4",
	chroma.Comment:             "#6a9955",
	chroma.CommentPreproc:      "#c586c0",
	chroma.Keyword:             "#569cd6",
	chroma.KeywordNamespace:    "#c586c0",
	chroma.KeywordType:         "#4ec9b0",
	chroma.Name:                "#9cdcfe",
	chroma.NameBuiltin:         "#4ec9b0",
	chroma.NameFunction:        "#dcdcaa",
	chroma.NameClass:           "#4ec9b0",
	chroma.NameConstant:        "#4fc1ff",
	chroma.NameTag:             "#569cd6",
	chroma.NameAttribute:       "#9cdcfe",
	chroma.NameDecorator:       "#dcdcaa",
	chroma.LiteralString:       "#ce9178",
	chroma.LiteralStringEscape: "#d7ba7d",
	chroma.LiteralNumber:       "#b5cea8",
	chroma.Operator:            "#d4d4d4",
	chroma.Punctuation:         "#d4d4d4",
	chroma.GenericHeading:      "bold #569cd6",
	chroma.GenericSubheading:   "bold #569cd6",
	chroma.GenericInserted:     "#b5cea8",
	chroma.GenericDeleted:      "#ce9178",
	chroma.GenericEmph:         "italic",
	chroma.GenericStrong:       "bold",
})

// highlighter caches per-rune foreground colors for one buffer.
type highlighter struct {
	lexer   chroma.Lexer
	version int
	colors  [][]string
	cache   map[chroma.TokenType]string
}

func newHighlighter(path string) *highlighter {
	h := &highlighter{version: -1, cache: map[chroma.TokenType]string{}}
	if path != "" {
		h.lexer = lexers.Match(filepath.Base(path))
	}
	if h.lexer != nil {
		h.lexer = chroma.Coalesce(h.lexer)
	}
	return h
}

// Language is a human-readable name of the detected language.
func (h *highlighter) Language() string {
	if h.lexer == nil {
		return "Plain Text"
	}
	return h.lexer.Config().Name
}

func (h *highlighter) color(tt chroma.TokenType) string {
	if c, ok := h.cache[tt]; ok {
		return c
	}
	c := ""
	if e := chromaStyle.Get(tt); e.Colour.IsSet() {
		c = e.Colour.String()
	}
	h.cache[tt] = c
	return c
}

// Colors returns per-rune colors ("" = default) for the buffer, recomputing
// only when the buffer changed.
func (h *highlighter) Colors(b *buffer.Buffer) [][]string {
	if h.lexer == nil || len(b.Lines) > maxHighlightLines {
		return nil
	}
	if h.version == b.Version && len(h.colors) == len(b.Lines) {
		return h.colors
	}
	colors := make([][]string, len(b.Lines))
	for i, l := range b.Lines {
		colors[i] = make([]string, len(l))
	}
	it, err := h.lexer.Tokenise(nil, b.Text())
	if err != nil {
		return nil
	}
	row, col := 0, 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		c := h.color(tok.Type)
		if !strings.ContainsRune(tok.Value, '\n') && c == "" {
			col += len([]rune(tok.Value))
			continue
		}
		for _, r := range tok.Value {
			if r == '\n' {
				row++
				col = 0
				continue
			}
			if row < len(colors) && col < len(colors[row]) {
				colors[row][col] = c
			}
			col++
		}
	}
	h.colors = colors
	h.version = b.Version
	return colors
}

// commentPrefix returns the line comment token for a file, or "".
func commentPrefix(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".c", ".h", ".cpp", ".hpp", ".cc",
		".java", ".rs", ".swift", ".kt", ".scala", ".cs", ".dart", ".zig", ".php", ".jsonc":
		return "//"
	case ".py", ".sh", ".bash", ".zsh", ".rb", ".yaml", ".yml", ".toml", ".nix",
		".pl", ".r", ".conf", ".ini", ".mk", ".dockerfile", ".fish", ".ps1":
		return "#"
	case ".lua", ".sql", ".hs", ".elm":
		return "--"
	case ".vim":
		return "\""
	case ".lisp", ".el", ".clj", ".scm":
		return ";"
	}
	switch filepath.Base(path) {
	case "Makefile", "makefile", "Dockerfile", ".gitignore", ".env":
		return "#"
	}
	return ""
}
