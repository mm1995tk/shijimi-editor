package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/core/msg"
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

// New lays the app out (and so looks for a multiplexer) before cli.Main has
// announced the primary; SetGUIPane after that still gives the GUI's panes.
func TestGUIPaneAfterLayout(t *testing.T) {
	a := newTestApp(t, "hello")
	tuicfg.Of(a.Cfg).Experimental.Multiplexer = true
	a.mux.parents = fakeChain("shj", "zsh", "kisui", "init")
	if a.muxOn() {
		t.Fatal("no multiplexer before the primary is announced")
	}
	f := &fakeGUI{}
	a.SetGUIPane(f.do)
	if !a.muxOn() || a.mux.b.name() != "gui" {
		t.Fatal("the primary keeps the built-in terminal")
	}
}

// What the first look said about a named multiplexer (the built-in terminal)
// does not stay on the status line once the primary uses the GUI.
func TestGUIPaneAfterLayoutClearsReason(t *testing.T) {
	t.Setenv(envHerdr, "")
	for _, s := range []string{"herdr", "screen"} {
		a := newTestApp(t, "hello")
		tuicfg.Of(a.Cfg).Experimental.Multiplexer = true
		tuicfg.Of(a.Cfg).Terminal.Multiplexer = s
		if a.muxOn() || (a.Msg.Key != "mux.unavailable" && a.Msg.Key != "mux.unsupported") {
			t.Fatalf("%s: expected a reason before the primary is announced: %v", s, a.Msg)
		}
		a.SetGUIPane((&fakeGUI{}).do)
		if !a.muxOn() || a.mux.b.name() != "gui" || a.Msg.Key != "" {
			t.Fatalf("%s: the GUI with no stale reason expected: %v", s, a.Msg)
		}
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

// "none" wins over the GUI and says why on the status line; a named multiplexer does not.
func TestGUIPaneSettings(t *testing.T) {
	a, _ := guiApp(t, "none")
	if a.muxOn() {
		t.Fatal("none uses the built-in terminal")
	}
	if a.Msg.Key != "mux.guioff.none" {
		t.Fatalf("no reason on the status line: %v", a.Msg)
	}
	for _, s := range []string{"tmux", "herdr", "zellij", "cmux"} {
		a, _ = guiApp(t, s)
		t.Setenv("TMUX", "")
		if !a.muxOn() || a.mux.b.name() != "gui" {
			t.Fatalf("%s: the primary uses the GUI", s)
		}
	}
}

// With [experimental] multiplexer off, the primary says so once.
func TestGUIPaneExperimentalOff(t *testing.T) {
	a, _ := guiApp(t, "auto")
	tuicfg.Of(a.Cfg).Experimental.Multiplexer = false
	if a.muxOn() || a.Msg.Key != "mux.guioff.experimental" {
		t.Fatalf("expected a reason: %v", a.Msg)
	}
	a.Msg = msg.Msg{}
	if a.muxOn() || a.Msg.Key != "" {
		t.Fatal("told again")
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

// A video, an image and a PDF are not opened as text or a tab: the primary shj asks the GUI for a pane, and a failure is on the status line.
func TestGUIOpenMedia(t *testing.T) {
	a := newTestApp(t, "hello")
	dir := t.TempDir()
	mp4 := dir + "/a.mp4"
	if err := os.WriteFile(mp4, []byte("not really a video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.OpenMedia("video", mp4); ok {
		t.Fatal("a shj without the GUI has no video view")
	}
	var got []ctl.PaneRequest
	var fail error
	a.SetGUIPane(func(r ctl.PaneRequest) (ctl.PaneReply, error) {
		got = append(got, r)
		return ctl.PaneReply{OK: fail == nil, ID: "x1"}, fail
	})
	n := len(a.Tabs)
	if !a.OpenFile(mp4) || len(a.Tabs) != n {
		t.Fatalf("a video opens in the GUI, not as a tab: %d tabs", len(a.Tabs))
	}
	if len(got) != 1 || got[0].Op != "open" || got[0].Kind != "video" || got[0].Path != mp4 {
		t.Fatalf("request: %+v", got)
	}
	fail = errors.New("too many panes")
	if a.OpenFile(mp4) || !a.MsgErr {
		t.Fatalf("a refusal is an error on the status line: %q", a.Msg)
	}
	fail = nil
	got = nil
	for kind, name := range map[string]string{"image": "/a.png", "pdf": "/a.pdf"} {
		p := dir + name
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !a.OpenFile(p) || len(a.Tabs) != n {
			t.Fatalf("a %s opens in the GUI, not as a tab: %d tabs", kind, len(a.Tabs))
		}
		if last := got[len(got)-1]; last.Op != "open" || last.Kind != kind || last.Path != p {
			t.Fatalf("request: %+v", got)
		}
	}
}

// :open-url in the primary shj asks the GUI for a browser pane, even over SSH; the other shj uses the opener.
func TestGUIOpenURL(t *testing.T) {
	a := newTestApp(t, "hello")
	oldSSH := core.IsRemoteSession
	core.IsRemoteSession = func() bool { return true }
	defer func() { core.IsRemoteSession = oldSSH }()
	var got []ctl.PaneRequest
	var fail error
	a.SetGUIPane(func(r ctl.PaneRequest) (ctl.PaneReply, error) {
		got = append(got, r)
		return ctl.PaneReply{OK: fail == nil, ID: "x1"}, fail
	})
	old := core.OpenExternal
	core.OpenExternal = func(string) error { t.Fatal("the opener must not run in the GUI"); return nil }
	defer func() { core.OpenExternal = old }()

	runCmd(a, a.effectCmd(a.ExecCommand("open-url http://localhost:3000/x")...))
	if len(got) != 1 || got[0].Op != "open" || got[0].Kind != "browser" || got[0].URL != "http://localhost:3000/x" || a.MsgErr {
		t.Fatalf("request: %+v err %v %q", got, a.MsgErr, a.Msg)
	}
	fail = errors.New("too many panes")
	runCmd(a, a.effectCmd(a.ExecCommand("open-url https://example.com")...))
	if !a.MsgErr {
		t.Fatalf("a refusal is an error on the status line: %q", a.Msg)
	}
}
