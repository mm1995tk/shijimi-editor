package vt

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/parser"
)

func TestTitleWithUTF8DoesNotSpillOntoScreen(t *testing.T) {
	term, _ := newT(40, 3)
	// ✳ is E2 9C B3 and 検 is E6 A4 9C: both hold 0x9C, the 8-bit ST.
	term.Write([]byte("\x1b[2;1H\x1b]0;✳ ファイル内検索機能\x07"))
	s := snap(term)
	if s.Title != "✳ ファイル内検索機能" {
		t.Fatalf("title was cut: %q", s.Title)
	}
	if got := rowText(s, 1); got != "" {
		t.Fatalf("title spilled onto the screen: %q", got)
	}
	term.Write([]byte("\x1b]2;検索\x1b\\"))
	if got := snap(term).Title; got != "検索" {
		t.Fatalf("title ended by ESC \\ = %q", got)
	}
}

func TestAPCSwallowsC1Bytes(t *testing.T) {
	term, _ := newT(20, 3)
	// 0x9B inside an APC string does not start a CSI, and 検索 is not printed.
	term.Write([]byte("\x1b_\x9b31m検索\x1b\\A"))
	if got := rowText(snap(term), 0); got != "A" {
		t.Fatalf("row = %q", got)
	}
}

func TestEightBitSTNoLongerTerminatesOSC(t *testing.T) {
	term, _ := newT(20, 3)
	term.Write([]byte("\x1b]0;t\x9cHELLO"))
	s := snap(term)
	if s.Title != "" || rowText(s, 0) != "" {
		t.Fatalf("0x9C should not end the OSC: title=%q row=%q", s.Title, rowText(s, 0))
	}
	term.Write([]byte("\x18WORLD")) // CAN cancels the string
	if got := rowText(snap(term), 0); got != "WORLD" {
		t.Fatalf("row = %q", got)
	}
}

func TestTableFixAppliesToStringWidth(t *testing.T) {
	// The init in this package changes the process-wide parser.Table, so the
	// UI's width calculations are fixed too.
	if got := ansi.StringWidth("\x1b]0;✳ 検索\x07TAIL"); got != 4 {
		t.Fatalf("width = %d, want 4", got)
	}
}

func TestUpstreamStillNeedsTheWorkaround(t *testing.T) {
	fresh := parser.GenerateTransitionTable()
	if _, action := fresh.Transition(parser.OscStringState, 0x9C); action != parser.DispatchAction {
		t.Fatal("x/ansi changed how 0x9C ends an OSC string; check whether the init in term.go is still needed")
	}
}
