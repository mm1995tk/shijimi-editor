package tui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestConfirmOverlay(t *testing.T) {
	a := newTestApp(t, "x\n")
	if a.confirmOverlay() != nil {
		t.Fatal("no confirmation asked")
	}
	a.confirm = &yesNo{prompt: "close a.txt? (y/n)"}
	ov := a.confirmOverlay()
	if ov == nil || ov.Kind != screen.OverlayConfirm || ov.Pane != "" || ov.At != nil || len(ov.Items) != 2 || ov.Lines[0].Spans[0].Text != "close a.txt? (y/n)" {
		t.Fatalf("overlay = %+v", ov)
	}
	if a.statusOf().Msg != "" {
		t.Error("the question is not a message")
	}
	a.confirm = nil
	a.confirmQuit, a.msg = true, "unsaved"
	ov = a.confirmOverlay()
	if ov == nil || len(ov.Items) != 0 || ov.Lines[0].Spans[0].Text != "unsaved" {
		t.Fatalf("overlay = %+v", ov)
	}
	if a.statusOf().Msg != "" {
		t.Error("the confirmation is not also a message")
	}
}
