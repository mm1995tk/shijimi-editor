package core

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestSignatureOverlay(t *testing.T) {
	a := newTestApp(t, "f(a, b)\n")
	a.CurText().Buf.Cur.Col = 2
	s := &signaturePopup{tab: a.CurText(), row: 0, help: &lsp.SignatureHelp{Label: "f(a int, b string)", ParamStart: 9, ParamEnd: 17}}
	ov := s.overlay()
	if ov.Kind != screen.OverlaySignature || ov.Pane != a.CurText().Path() || ov.At == nil || *ov.At != (screen.Pos{Line: 0, Col: 2}) || len(ov.Lines) != 1 {
		t.Fatalf("overlay = %+v", ov)
	}
	sp := ov.Lines[0].Spans
	if len(sp) != 3 || sp[0].Text != "f(a int, " || sp[1].Text != "b string" || sp[1].Style != screen.StylePaletteHi || sp[2].Text != ")" {
		t.Fatalf("spans = %+v", sp)
	}
}
