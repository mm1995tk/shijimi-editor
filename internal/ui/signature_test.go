package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/dageki/internal/lsp"
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
