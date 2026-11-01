package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

func TestSignatureRender(t *testing.T) {
	s := &core.SignaturePopup{Help: &lsp.SignatureHelp{Label: "f(a int, b string)", ParamStart: 9, ParamEnd: 17}}
	got := drawSignature(s.Overlay(), 80)
	if len(got) != 1 || ansi.Strip(got[0]) != " f(a int, b string) " {
		t.Errorf("render = %q", got)
	}
	if w := ansi.StringWidth(drawSignature(s.Overlay(), 10)[0]); w != 10 {
		t.Errorf("narrow width = %d, want 10", w)
	}
	s.Help.ParamStart, s.Help.ParamEnd = -1, -1
	if !strings.Contains(ansi.Strip(drawSignature(s.Overlay(), 80)[0]), "f(a int") {
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
