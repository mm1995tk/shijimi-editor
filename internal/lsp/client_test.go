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
