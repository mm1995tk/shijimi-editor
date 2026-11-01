package tui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestCompletionOverlay(t *testing.T) {
	items := make([]lsp.CompletionItem, 15)
	for i := range items {
		items[i] = lsp.CompletionItem{Label: string(rune('a' + i)), Kind: 3}
	}
	tb := &core.Tab{Buf: buffer.New()}
	c := &core.CompletionPopup{Tab: tb, All: items, Items: items, Sel: 12}
	ov := c.Overlay(20)
	if ov.Kind != screen.OverlayCompletion || ov.At == nil || len(ov.Items) != 10 || ov.Active != 9 || ov.Items[ov.Active].Label != "m" || ov.Items[0].Kind != "func" {
		t.Errorf("overlay = %+v", ov)
	}
	if got := len(drawCompletion(ov)); got != 10 {
		t.Errorf("drawn %d rows", got)
	}
}
