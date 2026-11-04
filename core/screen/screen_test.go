package screen

import (
	"encoding/json"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	in := Screen{
		Rev: 3, AckSeq: 2, Focus: RegionFiles,
		Files: &Files{Panes: []Pane{{
			ID: "p1", Kind: KindText, Viewport: Viewport{Rows: 10, Cols: 80},
			Lines:   []Line{{No: 0, Spans: []Span{{Text: "func", Style: StyleSyntaxKeyword}}}},
			Cursors: []Pos{{Line: 0, Col: 2}},
		}}},
		Overlay: []Overlay{{Kind: OverlayHover, Lines: []Line{{Spans: []Span{{Text: "x"}}}}}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Screen
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Rev != 3 || out.Files.Panes[0].Lines[0].Spans[0].Style != StyleSyntaxKeyword || out.Files.Panes[0].Cursors[0].Col != 2 {
		t.Fatalf("round trip lost data: %s", b)
	}
}
