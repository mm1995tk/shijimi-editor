package core

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

const maxHighlightLines = 20000

// chromaStyle holds the current theme's token colors; set by applyTheme. It
// is replaced, never modified, so a lexing command may keep the one it read.
var chromaStyle *chroma.Style

// highlighter colors one buffer. Lexing the whole buffer takes milliseconds,
// and bubbletea renders after every message, so lexing runs as a command
// (lexCmd) outside the render. Colors only returns what the last lexing
// produced, mapped onto the buffer's current lines.
type highlighter struct {
	lexer   chroma.Lexer
	running bool // a lexCmd is under way

	// The last lexing.
	version int        // buffer version lexed; -1 before the first
	gen     int        // themeGen it was colored with
	lines   []string   // the lines lexed
	colors  [][]string // per-rune colors of lines ("" = default)

	// colors mapped onto the buffer at viewVersion.
	viewVersion int
	view        [][]string

	sem semanticState
}

// highlightMsg carries the result of a lexCmd.
type highlightMsg struct {
	h       *highlighter
	version int
	gen     int
	lines   []string
	colors  [][]string
}

func newHighlighter(path string) *highlighter {
	h := &highlighter{version: -1, viewVersion: -1}
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

// lexCmd returns a command that lexes b's current text, or nil when the
// colors are current or a lexing is under way. Edits made meanwhile are
// lexed once that one is applied, so a burst of edits costs two lexings.
func (h *highlighter) lexCmd(b *buffer.Buffer) []Effect {
	if h.lexer == nil || h.running || len(b.Lines) > maxHighlightLines ||
		h.version == b.Version && h.gen == ThemeGen {
		return nil
	}
	h.running = true
	lines := make([]string, len(b.Lines))
	for i, l := range b.Lines {
		lines[i] = string(l)
	}
	msg := highlightMsg{h: h, version: b.Version, gen: ThemeGen, lines: lines}
	lexer, style := h.lexer, chromaStyle
	return async(func() Event {
		msg.colors = lexColors(lexer, style, lines)
		return msg
	})
}

// apply stores the result of a lexCmd.
func (h *highlighter) apply(m highlightMsg) {
	h.running = false
	if m.colors == nil {
		h.version, h.gen = m.version, m.gen // lexing failed; do not retry
		return
	}
	h.version, h.gen, h.lines, h.colors = m.version, m.gen, m.lines, m.colors
	h.viewVersion = -1
	h.sem.stamp++
}

// Colors returns per-rune colors ("" = default) for b, or nil before the first
// lexing. Until the latest edit is lexed, lines left unchanged keep their
// colors and edited lines have none.
func (h *highlighter) Colors(b *buffer.Buffer) [][]string {
	base := h.chromaColors(b)
	if h.sem.roles == nil {
		return base
	}
	return h.sem.overlay(b, base)
}

// chromaColors is Colors without the semantic tokens.
func (h *highlighter) chromaColors(b *buffer.Buffer) [][]string {
	if h.colors == nil || h.version == b.Version {
		return h.colors
	}
	if h.viewVersion != b.Version {
		h.view = alignColors(h.lines, h.colors, b.Lines)
		h.viewVersion = b.Version
	}
	return h.view
}

// alignColors maps colors of the lexed lines onto lines: the lines both
// start and end with keep their colors, the ones between have none.
func alignColors(lexed []string, colors [][]string, lines [][]rune) [][]string {
	out := make([][]string, len(lines))
	pre := 0
	for pre < len(lexed) && pre < len(lines) && lexed[pre] == string(lines[pre]) {
		out[pre] = colors[pre]
		pre++
	}
	for i, j := len(lexed)-1, len(lines)-1; i >= pre && j >= pre && lexed[i] == string(lines[j]); i, j = i-1, j-1 {
		out[j] = colors[i]
	}
	return out
}

// highlightNow lexes b right away, for a file shown once without a tab.
func highlightNow(path string, b *buffer.Buffer) [][]string {
	h := newHighlighter(path)
	if h.lexer == nil || len(b.Lines) > maxHighlightLines {
		return nil
	}
	lines := make([]string, len(b.Lines))
	for i, l := range b.Lines {
		lines[i] = string(l)
	}
	return lexColors(h.lexer, chromaStyle, lines)
}

// lexColors returns per-rune colors of lines, or nil if lexing fails.
func lexColors(lexer chroma.Lexer, style *chroma.Style, lines []string) [][]string {
	it, err := lexer.Tokenise(nil, strings.Join(lines, "\n"))
	if err != nil {
		return nil
	}
	cache := map[chroma.TokenType]string{}
	color := func(tt chroma.TokenType) string {
		c, ok := cache[tt]
		if !ok {
			if e := style.Get(tt); e.Colour.IsSet() {
				c = e.Colour.String()
			}
			cache[tt] = c
		}
		return c
	}
	colors := make([][]string, len(lines))
	for i, l := range lines {
		colors[i] = make([]string, len([]rune(l)))
	}
	row, col := 0, 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		c := color(tok.Type)
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

// lexTabs starts lexing every text tab whose colors are behind its text.
func (a *Core) LexTabs() []Effect {
	var cmds [][]Effect
	for _, t := range a.tabs {
		if t.Kind == TabText {
			cmds = append(cmds, t.hl.lexCmd(t.Buf))
		}
	}
	return Batch(cmds...)
}
