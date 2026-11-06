package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/ctl"
	tuicfg "github.com/mm1995tk/shijimi-editor/ui/tui/config"
)

type fakeGUI struct {
	ops  []string
	dead bool
}

func (f *fakeGUI) do(r ctl.PaneRequest) (ctl.PaneReply, error) {
	f.ops = append(f.ops, r.Op+" "+r.ID)
	switch r.Op {
	case "split":
		return ctl.PaneReply{OK: true, ID: "t9"}, nil
	case "alive":
		return ctl.PaneReply{OK: true, Alive: !f.dead}, nil
	case "tail":
		return ctl.PaneReply{OK: true, Lines: []string{"make: *** error"}}, nil
	}
	return ctl.PaneReply{OK: true}, nil
}

func (f *fakeGUI) count(op string) int {
	n := 0
	for _, o := range f.ops {
		if strings.HasPrefix(o, op+" ") {
			n++
		}
	}
	return n
}

func guiApp(t *testing.T, setting string) (*App, *fakeGUI) {
	t.Helper()
	a := newTestApp(t, "hello")
	tuicfg.Of(a.Cfg).Experimental.Multiplexer = true
	tuicfg.Of(a.Cfg).Terminal.Multiplexer = setting
	f := &fakeGUI{}
	a.SetGUIPane(f.do)
	return a, f
}

// The terminal commands use the GUI's panes in the primary shj, with "auto" even under another multiplexer.
func TestGUIPaneToggle(t *testing.T) {
	a, f := guiApp(t, "auto")
	a.mux.parents = fakeChain("shj", "zsh", "tmux: server", "systemd")
	ctrlBacktick := core.Key{Code: '`', Mod: core.ModCtrl}
	a.Focus = core.FocusEditor
	a.Update(ctrlBacktick)
	if a.showTerm || a.term.cur() != nil || a.mux.b == nil || a.mux.b.name() != "gui" {
		t.Fatalf("the GUI pane expected: %v", f.ops)
	}
	if f.count("split") != 1 || a.mux.id != "t9" || !a.mux.visible {
		t.Fatalf("not created: %v", f.ops)
	}
	a.Update(ctrlBacktick)
	if f.count("hide") != 1 || a.mux.visible {
		t.Fatalf("not hidden: %v", f.ops)
	}
	a.Update(ctrlBacktick)
	if f.count("show") != 1 || !a.mux.visible || f.count("split") != 1 {
		t.Fatalf("not shown again: %v", f.ops)
	}
	f.dead = true
	a.Update(ctrlBacktick)
	if f.count("split") != 2 {
		t.Fatalf("an ended pane is replaced: %v", f.ops)
	}
	a.Quit()
	if f.count("close") != 0 { // the pane is dead, so there is nothing to close
		t.Fatalf("closed an ended pane: %v", f.ops)
	}
	f.dead = false
	a.mux.id = "t9"
	a.muxClose()
	if f.count("close") != 1 {
		t.Fatalf("the pane stays after quitting: %v", f.ops)
	}
}

// Without the GUI (not the primary shj), the built-in terminal stays, also when "gui" is asked for.
func TestGUIPaneNeedsPrimary(t *testing.T) {
	for _, s := range []string{"auto", "gui"} {
		a := newTestApp(t, "hello")
		tuicfg.Of(a.Cfg).Experimental.Multiplexer = true
		tuicfg.Of(a.Cfg).Terminal.Multiplexer = s
		a.mux.parents = fakeChain("shj", "zsh", "alacritty", "init")
		if a.muxOn() {
			t.Errorf("%s: no GUI backend without SetGUIPane", s)
		}
	}
}

// "none" and a named multiplexer win over the GUI.
func TestGUIPaneSettings(t *testing.T) {
	a, _ := guiApp(t, "none")
	if a.muxOn() {
		t.Fatal("none uses the built-in terminal")
	}
	a, _ = guiApp(t, "tmux")
	t.Setenv("TMUX", "")
	if a.muxOn() {
		t.Fatal("tmux was asked for and is not usable")
	}
}

func TestGUIPaneErrorsAndTail(t *testing.T) {
	g := &guiBackend{do: func(ctl.PaneRequest) (ctl.PaneReply, error) { return ctl.PaneReply{}, fmt.Errorf("boom") }}
	if _, err := g.split("/", nil); err == nil {
		t.Fatal("split error")
	}
	if g.alive("t1") || g.tail("t1", 3) != nil {
		t.Fatal("errors mean not alive and no lines")
	}
	f := &fakeGUI{}
	g = &guiBackend{do: f.do}
	if l := g.tail("t1", 3); len(l) != 1 {
		t.Fatalf("tail: %v", l)
	}
}
