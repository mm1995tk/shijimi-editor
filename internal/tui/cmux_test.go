package tui

import (
	"fmt"
	"strings"
	"testing"
)

// fakeCmux plays the cmux app: it records the calls and keeps the surfaces.
type fakeCmux struct {
	calls []string
	gone  bool // the new surface has been closed
	fail  string
}

func (f *fakeCmux) call(args ...string) ([]byte, error) {
	line := strings.Join(args, " ")
	f.calls = append(f.calls, line)
	if f.fail != "" && strings.Contains(line, f.fail) {
		return nil, fmt.Errorf("boom")
	}
	switch {
	case strings.Contains(line, " new-split "):
		f.gone = false
		return []byte(`{"surface_id":"S2","workspace_id":"W1"}`), nil
	case strings.Contains(line, " list-panels"):
		out := `{"panels":[{"id":"S1","focused":false}`
		if !f.gone {
			out += `,{"id":"S2","focused":true}`
		}
		return []byte(out + "]}"), nil
	case args[0] == "close-surface":
		f.gone = true
	case args[0] == "read-screen":
		return []byte("old\nmake: *** error\n\n"), nil
	}
	return nil, nil
}

func (f *fakeCmux) has(s string) bool { return f.count(s) > 0 }

func (f *fakeCmux) count(s string) int {
	n := 0
	for _, c := range f.calls {
		if strings.Contains(c, s) {
			n++
		}
	}
	return n
}

func cmuxApp(t *testing.T) (*App, *fakeCmux) {
	t.Helper()
	t.Setenv("SHELL", "/bin/zsh")
	a := newTestApp(t, "hello")
	a.cfg.Experimental.Multiplexer = true
	f := &fakeCmux{}
	a.mux.newBackend = func(kind string) muxBackend {
		if kind == "cmux" {
			return &cmuxBackend{editorPane: "S1", workspace: "W1", call: f.call}
		}
		return nil
	}
	a.cfg.Terminal.Multiplexer = "cmux"
	return a, f
}

// cmuxEnv makes cmux usable: inside cmux on macOS.
func cmuxEnv(t *testing.T) {
	t.Helper()
	old := cmuxGOOS
	cmuxGOOS = "darwin"
	t.Cleanup(func() { cmuxGOOS = old })
	t.Setenv(envCmuxWorkspace, "W1")
	t.Setenv(envCmuxSurface, "S1")
}

func TestCmuxToggle(t *testing.T) {
	cmuxEnv(t)
	a, f := cmuxApp(t)
	ctrlBacktick := Key{Code: '`', Mod: ModCtrl}
	a.focus = focusEditor
	a.Update(ctrlBacktick)
	if a.showTerm || a.term.cur() != nil {
		t.Fatal("the built-in terminal must stay closed")
	}
	if !f.has("--json --id-format uuids new-split down --focus true --surface S1 --command cd '"+a.root+"' && exec env ") ||
		a.mux.id != "S2" || !a.mux.visible {
		t.Fatalf("not created: %v id=%q", f.calls, a.mux.id)
	}
	a.Update(ctrlBacktick)
	if !f.has("focus-panel --panel S1") || a.mux.visible {
		t.Fatalf("not hidden: %v", f.calls)
	}
	a.Update(ctrlBacktick)
	if !f.has("focus-panel --panel S2") || !a.mux.visible {
		t.Fatalf("not shown again: %v", f.calls)
	}
	if n := f.count(" new-split "); n != 1 {
		t.Fatalf("created %d times", n)
	}
	// A closed pane is replaced.
	f.gone = true
	a.Update(ctrlBacktick)
	if n := f.count(" new-split "); n != 2 {
		t.Fatalf("closed pane: created %d times, want 2", n)
	}
	a.quit()
	if !f.has("close-surface --surface S2 --force --workspace W1") {
		t.Fatalf("pane not closed: %v", f.calls)
	}
}

func TestCmuxCommands(t *testing.T) {
	cmuxEnv(t)
	a, f := cmuxApp(t)
	a.execCommand("sh make test")
	if !f.has("paste --surface S2 -- make test") || !f.has("send-key --surface S2 enter") {
		t.Fatalf("calls: %v", f.calls)
	}
	f.calls = nil
	a.ctx = &ctxPreview{text: "ctx"}
	a.handleContextKey(Key{Code: KeyEnter})
	if !f.has("paste --surface S2 --force -- ctx") || f.has("send-key") {
		t.Fatalf("context must be pasted without Enter: %v", f.calls)
	}
	f.fail = "focus-panel"
	a.msg = ""
	a.execCommand("sh hide")
	if !a.msgErr || !strings.Contains(a.msg, "boom") {
		t.Fatalf("error not shown: %q", a.msg)
	}
}

func TestCmuxSplitCommand(t *testing.T) {
	got := cmuxCommand("/r s", []string{"SHIJIMI_SOCKET=/s", "A=b'c"}, "/bin/zsh")
	want := `cd '/r s' && exec env 'SHIJIMI_SOCKET=/s' 'A=b'\''c' '/bin/zsh'`
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestCmuxSplitFallsBackToRef(t *testing.T) {
	b := &cmuxBackend{call: func(args ...string) ([]byte, error) { return []byte(`{"surface_ref":"surface:7"}`), nil }}
	if id, err := b.split("/r", nil); err != nil || id != "surface:7" {
		t.Fatalf("%q %v", id, err)
	}
}

func TestCmuxBadReply(t *testing.T) {
	b := &cmuxBackend{call: func(args ...string) ([]byte, error) { return []byte("OK\n"), nil }}
	if _, err := b.split("/r", nil); err == nil {
		t.Fatal("a reply without a surface id must fail")
	}
	if b.alive("S2") {
		t.Fatal("an unreadable list has no live pane")
	}
	if err := (&cmuxBackend{}).hide(); err == nil {
		t.Fatal("hide needs the editor's surface")
	}
}

func TestCmuxOnlyOnMacInCmux(t *testing.T) {
	a, f := cmuxApp(t)
	t.Setenv(envCmuxWorkspace, "W1")
	t.Setenv(envCmuxSurface, "S1")
	old := cmuxGOOS
	defer func() { cmuxGOOS = old }()
	cmuxGOOS = "linux"
	a.resolveMux()
	if a.MuxOn() {
		t.Fatal("cmux must not be used on Linux")
	}
	cmuxGOOS = "darwin"
	t.Setenv(envCmuxSurface, "")
	a.resolveMux()
	if a.MuxOn() {
		t.Fatal("cmux must not be used outside cmux")
	}
	t.Setenv(envCmuxSurface, "S1")
	a.resolveMux()
	if !a.MuxOn() {
		t.Fatal("cmux must be used on macOS inside cmux")
	}
	a.cfg.Experimental.Multiplexer = false
	if a.MuxOn() {
		t.Fatal("cmux must be off when the setting is off")
	}
	if len(f.calls) != 0 {
		t.Fatalf("unexpected calls: %v", f.calls)
	}
}

func TestCmuxCommandEnd(t *testing.T) {
	cmuxEnv(t)
	a, _ := cmuxApp(t)
	a.mux.id = "S2"
	a.resolveMux()
	a.CommandEnded(1)
	if len(a.sg.fails) != 1 || a.sg.fails[0].code != 1 {
		t.Fatalf("fails: %+v", a.sg.fails)
	}
	if out := a.sg.fails[0].output; len(out) == 0 || out[len(out)-1] != "make: *** error" {
		t.Fatalf("output: %q", out)
	}
	a.CommandEnded(0)
	if len(a.sg.fails) != 0 {
		t.Fatalf("success must clear: %+v", a.sg.fails)
	}
}

func TestCmuxAutoFollowsParents(t *testing.T) {
	cmuxEnv(t)
	a, _ := cmuxApp(t)
	a.cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "cmux", "launchd")
	if !a.MuxOn() || a.mux.b.name() != "cmux" {
		t.Fatalf("cmux parent not used: %v", a.mux.b)
	}
}
