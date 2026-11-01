package core

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// The core answers an Event with Effects and never touches bubbletea.
func TestWatchTickEffects(t *testing.T) {
	a := newTestApp(t, "one\n")
	effs := a.HandleEvent(WatchTick{})
	var timer *StartTimer
	for _, e := range effs {
		if st, ok := e.(StartTimer); ok {
			timer = &st
		}
	}
	if timer == nil || timer.After != watchInterval {
		t.Fatalf("WatchTick must schedule the next one: %#v", effs)
	}
	if _, ok := timer.Fire.(WatchTick); !ok {
		t.Fatalf("timer fires %T", timer.Fire)
	}
}

func TestStaleHoldScrollTickIsDropped(t *testing.T) {
	a := newTestApp(t, "one\n")
	if effs := a.HandleEvent(HoldScrollTick{seq: a.holdSeq + 1}); len(effs) != 0 {
		t.Fatalf("stale tick produced %#v", effs)
	}
}

// A server's own event is handled and the next one is asked for in the same
// answer, so none is missed while the core works.
func TestLSPServerEventAsksForTheNext(t *testing.T) {
	a := newTestApp(t, "one\n")
	effs := a.HandleEvent(LspEventMsg(lsp.Event{Server: "gopls", Message: "ready"}))
	var waits int
	for _, e := range effs {
		if w, ok := e.(WaitLSP); ok && w.From == a.lsp.events {
			waits++
		}
	}
	if waits != 1 {
		t.Fatalf("want one WaitLSP, got %#v", effs)
	}
}

// A signature reply that is not for the latest request is dropped.
func TestStaleSignatureReplyIsDropped(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.sigSeq = 5
	a.HandleEvent(signatureMsg{tab: a.CurText(), seq: 4, manual: true, help: &lsp.SignatureHelp{Label: "f()"}})
	if a.sig != nil {
		t.Fatal("stale reply opened the popup")
	}
	a.HandleEvent(signatureMsg{tab: a.CurText(), seq: 5, manual: true, help: &lsp.SignatureHelp{Label: "f()"}})
	if a.sig == nil {
		t.Fatal("current reply was dropped")
	}
}

// An agent Event is handled and the next one is asked for in the same answer;
// one from a client that was dropped (older gen) changes nothing.
func TestAgentEventAsksForTheNext(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.ag.gen = 2
	a.ag.armed = true
	effs := a.HandleEvent(agentUpdateMsg{gen: 1})
	if len(a.ag.log) != 0 {
		t.Fatalf("stale update was logged: %+v", a.ag.log)
	}
	if len(effs) != 1 {
		t.Fatalf("want one WaitAgent, got %#v", effs)
	}
	if w, ok := effs[0].(WaitAgent); !ok || w.From != a.ag.ch {
		t.Fatalf("want WaitAgent on the agent channel, got %#v", effs[0])
	}
}
