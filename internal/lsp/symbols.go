package lsp

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Symbol is one entry of a document or workspace symbol list. Range is the
// part to jump to (the name, for a document symbol).
type Symbol struct {
	Name      string
	Kind      int
	Container string
	URI       string
	Range     Range
}

// DocumentSymbols returns the symbols defined in the file, in document order.
func (c *Client) DocumentSymbols(ctx context.Context, path string) ([]Symbol, error) {
	var raw json.RawMessage
	p := map[string]any{"textDocument": map[string]any{"uri": URI(path)}}
	if err := c.Call(ctx, "textDocument/documentSymbol", p, &raw); err != nil {
		return nil, err
	}
	return parseDocumentSymbols(raw, URI(path)), nil
}

// WorkspaceSymbols returns the project's symbols whose names match query.
func (c *Client) WorkspaceSymbols(ctx context.Context, query string) ([]Symbol, error) {
	var raw json.RawMessage
	if err := c.Call(ctx, "workspace/symbol", map[string]any{"query": query}, &raw); err != nil {
		return nil, err
	}
	return parseSymbolInformation(raw, ""), nil
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []documentSymbol `json:"children"`
	Location       *Location        `json:"location"`
}

// parseDocumentSymbols accepts both DocumentSymbol[] (nested) and
// SymbolInformation[] (flat) and returns a flat list.
func parseDocumentSymbols(raw json.RawMessage, uri string) []Symbol {
	var list []documentSymbol
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	if len(list) > 0 && list[0].Location != nil {
		out := parseSymbolInformation(raw, uri)
		sort.SliceStable(out, func(i, j int) bool {
			a, b := out[i].Range.Start, out[j].Range.Start
			return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
		})
		return out
	}
	var out []Symbol
	var walk func(list []documentSymbol, container string)
	walk = func(list []documentSymbol, container string) {
		for _, d := range list {
			out = append(out, Symbol{Name: d.Name, Kind: d.Kind, Container: container, URI: uri, Range: d.SelectionRange})
			if container != "" {
				walk(d.Children, container+"."+d.Name)
			} else {
				walk(d.Children, d.Name)
			}
		}
	}
	walk(list, "")
	return out
}

// parseSymbolInformation reads SymbolInformation[] and WorkspaceSymbol[]
// (whose location may lack a range). uri is used when an entry has none.
func parseSymbolInformation(raw json.RawMessage, uri string) []Symbol {
	var list []struct {
		Name          string `json:"name"`
		Kind          int    `json:"kind"`
		ContainerName string `json:"containerName"`
		Location      struct {
			URI   string `json:"uri"`
			Range Range  `json:"range"`
		} `json:"location"`
	}
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	out := make([]Symbol, 0, len(list))
	for _, s := range list {
		u := s.Location.URI
		if u == "" {
			u = uri
		}
		out = append(out, Symbol{Name: s.Name, Kind: s.Kind, Container: s.ContainerName, URI: u, Range: s.Location.Range})
	}
	return out
}

var symbolKinds = [...]string{
	1: "sym.file", 2: "sym.module", 3: "sym.namespace", 4: "sym.package", 5: "sym.class",
	6: "sym.method", 7: "sym.property", 8: "sym.field", 9: "sym.constructor", 10: "sym.enum",
	11: "sym.interface", 12: "sym.function", 13: "sym.variable", 14: "sym.constant", 15: "sym.string",
	16: "sym.number", 17: "sym.boolean", 18: "sym.array", 19: "sym.object", 20: "sym.key",
	21: "sym.null", 22: "sym.enummember", 23: "sym.struct", 24: "sym.event", 25: "sym.operator", 26: "sym.typeparam",
}

// SymbolKindName is the localized name of an LSP SymbolKind.
func SymbolKindName(kind int) string {
	if kind > 0 && kind < len(symbolKinds) {
		return i18n.T(symbolKinds[kind])
	}
	return i18n.T("sym.other")
}
