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
