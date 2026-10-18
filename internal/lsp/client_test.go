package lsp

import (
	"encoding/json"
	"testing"
)

func TestHoverText(t *testing.T) {
	cases := []struct{ in, want string }{
		{`null`, ""},
		{`"plain"`, "plain"},
		{`{"kind":"markdown","value":"~~~go\nx\n~~~"}`, "~~~go\nx\n~~~"},
		{`{"language":"go","value":"x"}`, "x"},
		{`["a",{"kind":"plaintext","value":"b"}]`, "a\n\nb"},
	}
	for _, c := range cases {
		if got := hoverText(json.RawMessage(c.in)); got != c.want {
			t.Errorf("hoverText(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseCodeAction(t *testing.T) {
	uri := "file:///a.go"
	a, ok := parseCodeAction(json.RawMessage(`{"title":"Fix","kind":"quickfix","isPreferred":true,
		"edit":{"documentChanges":[{"textDocument":{"uri":"` + uri + `","version":1},"edits":[{"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":3}},"newText":"x"}]}]}}`))
	if !ok || a.Title != "Fix" || a.Kind != "quickfix" || !a.IsPreferred || len(a.Edit[uri]) != 1 || a.Command != nil {
		t.Errorf("action with edit: %+v %v", a, ok)
	}

	a, ok = parseCodeAction(json.RawMessage(`{"title":"Extract","kind":"refactor","command":{"title":"Extract","command":"x.extract","arguments":[1]}}`))
	if !ok || a.Edit != nil || a.Command == nil || a.Command.Command != "x.extract" || len(a.Command.Arguments) != 1 {
		t.Errorf("action with command: %+v %v", a, ok)
	}

	// A bare Command: "command" is the command's name.
	a, ok = parseCodeAction(json.RawMessage(`{"title":"Run","command":"x.run","arguments":["a"]}`))
	if !ok || a.Command == nil || a.Command.Command != "x.run" || a.Command.Title != "Run" {
		t.Errorf("bare command: %+v %v", a, ok)
	}

	if _, ok := parseCodeAction(json.RawMessage(`{"title":"No","disabled":{"reason":"r"}}`)); ok {
		t.Error("disabled action should be dropped")
	}
}

func TestParseWorkspaceEditChanges(t *testing.T) {
	we := parseWorkspaceEdit(json.RawMessage(`{"changes":{"file:///a.go":[{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}},"newText":"x"}]},
		"documentChanges":[{"kind":"create","uri":"file:///b.go"}]}`))
	if len(we) != 1 || len(we["file:///a.go"]) != 1 {
		t.Errorf("got %+v", we)
	}
}

func TestParseDocumentSymbolsNested(t *testing.T) {
	uri := "file:///a.go"
	got := parseDocumentSymbols(json.RawMessage(`[
		{"name":"Server","kind":23,"range":{"start":{"line":0,"character":0},"end":{"line":9,"character":1}},
		 "selectionRange":{"start":{"line":0,"character":5},"end":{"line":0,"character":11}},
		 "children":[{"name":"Start","kind":6,"range":{"start":{"line":2,"character":0},"end":{"line":4,"character":1}},
		   "selectionRange":{"start":{"line":2,"character":10},"end":{"line":2,"character":15}}}]}]`), uri)
	if len(got) != 2 || got[0].Name != "Server" || got[0].Range.Start.Character != 5 ||
		got[1].Name != "Start" || got[1].Container != "Server" || got[1].URI != uri {
		t.Errorf("got %+v", got)
	}
}

func TestParseDocumentSymbolsFlat(t *testing.T) {
	uri := "file:///a.go"
	got := parseDocumentSymbols(json.RawMessage(`[
		{"name":"b","kind":12,"containerName":"pkg","location":{"uri":"`+uri+`","range":{"start":{"line":5,"character":0},"end":{"line":6,"character":0}}}},
		{"name":"a","kind":13,"location":{"uri":"`+uri+`","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":3}}}}]`), uri)
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" || got[1].Container != "pkg" || got[1].Range.Start.Line != 5 {
		t.Errorf("got %+v", got)
	}
	if got := parseDocumentSymbols(json.RawMessage(`null`), uri); len(got) != 0 {
		t.Errorf("null: %+v", got)
	}
}

func TestParseWorkspaceSymbols(t *testing.T) {
	// A WorkspaceSymbol's location may carry only the uri.
	got := parseSymbolInformation(json.RawMessage(`[{"name":"X","kind":5,"location":{"uri":"file:///x.go"}}]`), "")
	if len(got) != 1 || got[0].URI != "file:///x.go" || SymbolKindName(got[0].Kind) != "クラス" {
		t.Errorf("got %+v", got)
	}
}
