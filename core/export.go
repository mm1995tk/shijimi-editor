package core

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/core/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/core/internal/ctlserver"
	"github.com/mm1995tk/shijimi-editor/core/internal/diff"
	"github.com/mm1995tk/shijimi-editor/core/internal/ide"
	"github.com/mm1995tk/shijimi-editor/core/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/core/msg"
	"os"
)

// What the UIs and cmd may use of the parts under core/internal. A UI never
// imports buffer, diff, ide, ctlserver or lsp: it names them through this
// file, and core/deps_test.go keeps it that way.

// Text buffer: positions and the cell arithmetic the UIs draw with.
type (
	Buffer = buffer.Buffer
	Pos    = buffer.Pos
)

// NewBuffer returns an empty buffer.
func NewBuffer() *Buffer { return buffer.New() }

// VisualCol returns the cell column of  col in line.
func VisualCol(line []rune, col int) int { return buffer.VisualCol(line, col) }

// ColFromVisual is the inverse of VisualCol.
func ColFromVisual(line []rune, vis int) int { return buffer.ColFromVisual(line, vis) }

// RuneWidth returns the cells r takes at cell column vis.
func RuneWidth(r rune, vis int) int { return buffer.RuneWidth(r, vis) }

// WordRange returns the range of the word at col in line.
func WordRange(line []rune, col int) (int, int) { return buffer.WordRange(line, col) }

// MatchCols returns the  columns of the matches of q in line.
func MatchCols(line, q []rune) []int { return buffer.MatchCols(line, q) }

// Diff and language server values that appear in the state the UIs read.
type (
	DiffFile       = diff.File
	DiffSpan       = diff.Span
	Diagnostic     = lsp.Diagnostic
	CompletionItem = lsp.CompletionItem
	SignatureHelp  = lsp.SignatureHelp
)

// SymbolKindName is the name of an LSP SymbolKind, as a message.
func SymbolKindName(kind int) msg.Msg { return lsp.SymbolKindName(kind) }

// IDE integration (Claude Code connects to it) and control socket.
type (
	IDEServer  = ide.Server
	IDEContent = ide.Content
	IDETool    = ide.Tool
	CtlServer  = ctlserver.Server
)

// IDEAuthHeader carries the token from the lock file.
const IDEAuthHeader = ide.AuthHeader

// IDEText returns a text block of an IDE tool result.
func IDEText(s string) IDEContent { return ide.Text(s) }

// IDEEnvKeys are the environment variables that point a Claude Code at the
// IDE server; a terminal started for a shell clears them.
func IDEEnvKeys() []string {
	return []string{ide.EnvPort, ide.EnvEnable, ide.EnvForceTerminal}
}

// StartIDE starts the IDE server for the workspace folder root with the tools
// of IDETools, answering through h.
func StartIDE(root string, h func(ctx context.Context, tool string, args json.RawMessage) ([]IDEContent, error)) (*IDEServer, error) {
	return ide.Start(root, IDETools(), h)
}

// SetIDEEnv makes a Claude Code started from this process connect to the IDE
// server on port.
func SetIDEEnv(port int) {
	os.Setenv(ide.EnvPort, strconv.Itoa(port))
	os.Setenv(ide.EnvEnable, "true")
	os.Setenv(ide.EnvForceTerminal, "true")
}

// ListenCtl opens the control socket at path, answering through h. text turns
// a message of the server's own into the UI's language.
func ListenCtl(path string, h ctl.Handler, text func(msg.Msg) string) (*CtlServer, error) {
	return ctlserver.Listen(path, h, text)
}
