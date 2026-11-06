package screen

import (
	"encoding/json"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	in := Pane{
		ID: "p1", Kind: KindText, Viewport: Viewport{Rows: 10, Cols: 80},
		Lines:   []Line{{No: 0, Spans: []Span{{Text: "func", Style: StyleSyntaxKeyword}}}},
		Cursors: []Pos{{Line: 0, Col: 2}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Pane
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Lines[0].Spans[0].Style != StyleSyntaxKeyword || out.Cursors[0].Col != 2 {
		t.Fatalf("round trip lost data: %s", b)
	}
}
