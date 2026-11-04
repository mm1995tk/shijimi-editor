package core

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

func TestConfirmOverlay(t *testing.T) {
	a := newTestApp(t, "x\n")
	if a.ConfirmOverlay() != nil {
		t.Fatal("no confirmation asked")
	}
	a.Confirm = &YesNo{Prompt: msg.S("close a.txt? (y/n)")}
	ov := a.ConfirmOverlay()
	if ov == nil || ov.Kind != screen.OverlayConfirm || ov.Pane != "" || ov.At != nil || len(ov.Items) != 2 || tt(ov.Title) != "close a.txt? (y/n)" {
		t.Fatalf("overlay = %+v", ov)
	}
	if !a.StatusOf().Msg.IsZero() {
		t.Error("the question is not a message")
	}
	a.Confirm = nil
	a.confirmQuit, a.Msg = true, msg.S("unsaved")
	ov = a.ConfirmOverlay()
	if ov == nil || len(ov.Items) != 0 || tt(ov.Title) != "unsaved" {
		t.Fatalf("overlay = %+v", ov)
	}
	if !a.StatusOf().Msg.IsZero() {
		t.Error("the confirmation is not also a message")
	}
}
