package lsp

import (
	"context"
)

// SemanticToken is one token from textDocument/semanticTokens/full, resolved
// to an absolute position and to names from the server's legend.
//
// Line is 0-based. StartChar and Length are in UTF-16 code units, the unit the
// client declares in positionEncodings, so they match Position.Character.
type SemanticToken struct {
	Line      int
	StartChar int
	Length    int
	Type      string   // e.g. "function", "type"; "" if the index is out of the legend
	Modifiers []string // e.g. "declaration", "readonly"
}

// Standard token types and modifiers of the LSP specification. They are
// declared in initialize so that the server may use any of them.
var (
	semanticTokenTypes = []string{
		"namespace", "type", "class", "enum", "interface", "struct", "typeParameter",
		"parameter", "variable", "property", "enumMember", "event", "function",
		"method", "macro", "keyword", "modifier", "comment", "string", "number",
		"regexp", "operator", "decorator",
	}
	semanticTokenModifiers = []string{
		"declaration", "definition", "readonly", "static", "deprecated", "abstract",
		"async", "modification", "documentation", "defaultLibrary",
	}
)

// SemanticTokens returns the semantic tokens of the file in document order. It
// returns nil, nil when the server has no semanticTokensProvider (or does not
// support full requests), so callers need not check Supports first.
func (c *Client) SemanticTokens(ctx context.Context, path string) ([]SemanticToken, error) {
	if !c.Supports("semanticTokensProvider") {
		return nil, nil
	}
	provider, _ := c.caps["semanticTokensProvider"].(map[string]any)
	if full, ok := provider["full"]; ok && full == false {
		return nil, nil
	}
	types, mods := semanticLegend(provider)
	var res struct {
		Data []int `json:"data"`
	}
	p := map[string]any{"textDocument": map[string]any{"uri": URI(path)}}
	if err := c.Call(ctx, "textDocument/semanticTokens/full", p, &res); err != nil {
		return nil, err
	}
	return decodeSemanticTokens(res.Data, types, mods), nil
}

// semanticLegend reads legend.tokenTypes and legend.tokenModifiers.
func semanticLegend(provider map[string]any) (types, mods []string) {
	legend, _ := provider["legend"].(map[string]any)
	strs := func(v any) []string {
		var out []string
		list, _ := v.([]any)
		for _, x := range list {
			s, _ := x.(string)
			out = append(out, s)
		}
		return out
	}
	return strs(legend["tokenTypes"]), strs(legend["tokenModifiers"])
}

// decodeSemanticTokens turns the relative encoding (five integers per token:
// deltaLine, deltaStart, length, type index, modifier bit set) into absolute
// tokens. deltaStart is relative to the previous token only on the same line.
func decodeSemanticTokens(data []int, types, mods []string) []SemanticToken {
	var out []SemanticToken
	line, start := 0, 0
	for i := 0; i+5 <= len(data); i += 5 {
		dl, ds := data[i], data[i+1]
		if dl > 0 {
			line += dl
			start = ds
		} else {
			start += ds
		}
		t := SemanticToken{Line: line, StartChar: start, Length: data[i+2]}
		if ti := data[i+3]; ti >= 0 && ti < len(types) {
			t.Type = types[ti]
		}
		for b := 0; b < len(mods); b++ {
			if data[i+4]&(1<<b) != 0 {
				t.Modifiers = append(t.Modifiers, mods[b])
			}
		}
		out = append(out, t)
	}
	return out
}
