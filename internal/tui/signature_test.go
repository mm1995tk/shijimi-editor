package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestSignatureRender(t *testing.T) {
	s := &signaturePopup{help: &lsp.SignatureHelp{Label: "f(a int, b string)", ParamStart: 9, ParamEnd: 17}}
	got := s.render(80)
	if len(got) != 1 || ansi.Strip(got[0]) != " f(a int, b string) " {
		t.Errorf("render = %q", got)
	}
	if w := ansi.StringWidth(s.render(10)[0]); w != 10 {
		t.Errorf("narrow width = %d, want 10", w)
	}
	s.help.ParamStart, s.help.ParamEnd = -1, -1
	if !strings.Contains(ansi.Strip(s.render(80)[0]), "f(a int") {
		t.Errorf("no active parameter should still render")
	}
}

func TestSignaturePos(t *testing.T) {
	l := layout{rightX: 10, editY: 2, editH: 20}
	box := []string{"abc"}
	if x, y := signaturePos(l, 80, 30, 8, box); x != 30 || y != 7 {
		t.Errorf("above cursor: %d,%d", x, y)
	}
	if _, y := signaturePos(l, 80, 30, 2, box); y != 3 {
		t.Errorf("top row should go below: y=%d", y)
	}
}

func TestSignatureOverlay(t *testing.T) {
	a := newTestApp(t, "f(a, b)\n")
	a.curText().buf.Cur.Col = 2
	s := &signaturePopup{tab: a.curText(), row: 0, help: &lsp.SignatureHelp{Label: "f(a int, b string)", ParamStart: 9, ParamEnd: 17}}
	ov := s.overlay()
	if ov.Kind != screen.OverlaySignature || ov.Pane != a.curText().path() || ov.At == nil || *ov.At != (screen.Pos{Line: 0, Col: 2}) || len(ov.Lines) != 1 {
		t.Fatalf("overlay = %+v", ov)
	}
	sp := ov.Lines[0].Spans
	if len(sp) != 3 || sp[0].Text != "f(a int, " || sp[1].Text != "b string" || sp[1].Style != screen.StylePaletteHi || sp[2].Text != ")" {
		t.Fatalf("spans = %+v", sp)
	}
}
