package tui

import (
	"fmt"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"os"
	"strings"
	"testing"
)

type fakeHerdr struct {
	calls []string
	dead  bool // pane get fails
	fail  string
}

func (f *fakeHerdr) call(args ...string) ([]byte, error) {
	line := strings.Join(args, " ")
	f.calls = append(f.calls, line)
	if f.fail != "" && strings.Contains(line, f.fail) {
		return nil, fmt.Errorf("boom")
	}
	switch {
	case strings.HasPrefix(line, "pane get"):
		if f.dead {
			return nil, fmt.Errorf("no such pane")
		}
	case strings.HasPrefix(line, "pane split"):
		return []byte(`{"id":"1","result":{"pane":{"pane_id":"w1-p2"}}}`), nil
	case strings.HasPrefix(line, "pane read"):
		return []byte("make: *** error\n"), nil
	}
	return nil, nil
}

func (f *fakeHerdr) has(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func herdrApp(t *testing.T) (*App, *fakeHerdr) {
	t.Helper()
	t.Setenv("HERDR_ENV", "1")
	a := newTestApp(t, "hello")
	a.Cfg.Experimental.Multiplexer = true
	f := &fakeHerdr{}
	a.mux.newBackend = func(kind string) muxBackend {
		if kind == "herdr" {
			return &herdrBackend{call: f.call}
		}
		return nil
	}
	a.Cfg.Terminal.Multiplexer = "herdr"
	return a, f
}

func TestHerdrToggle(t *testing.T) {
	a, f := herdrApp(t)
	ctrlBacktick := core.Key{Code: '`', Mod: core.ModCtrl}
	a.Focus = core.FocusEditor
	a.Update(ctrlBacktick)
	if a.showTerm || a.term.cur() != nil {
		t.Fatal("the built-in terminal must stay closed")
	}
	if !f.has("pane split --current --direction down --ratio 0.3 --cwd "+a.Root+" --focus") || a.mux.id != "w1-p2" || !a.mux.visible {
		t.Fatalf("not created: %v id=%q", f.calls, a.mux.id)
	}
	a.Update(ctrlBacktick)
	if !f.has("pane zoom --current --on") || a.mux.visible {
		t.Fatalf("not hidden: %v", f.calls)
	}
	a.Update(ctrlBacktick)
	if !f.has("pane zoom --current --off") || !f.has("pane focus --current --direction down") || !a.mux.visible {
		t.Fatalf("not shown again: %v", f.calls)
	}
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, "pane split") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("split %d times", n)
	}
	// An ended pane is replaced.
	f.dead = true
	a.Update(ctrlBacktick)
	f.dead = false
	n = 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, "pane split") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("ended pane: split %d times, want 2", n)
	}
	// Quitting closes the pane.
	a.Quit()
	if !f.has("pane close w1-p2") {
		t.Fatalf("pane not closed: %v", f.calls)
	}
}

func TestHerdrCommands(t *testing.T) {
	a, f := herdrApp(t)
	a.ExecCommand("sh make test")
	if !f.has("pane run w1-p2 make test") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.ExecCommand("sh hide")
	if !f.has("pane zoom --current --on") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.Ctx = &core.CtxPreview{Text: "ctx"}
	a.HandleContextKey(core.Key{Code: KeyEnter})
	if !f.has("pane send-text w1-p2 ctx") || f.has("pane run w1-p2 ctx") {
		t.Fatalf("context must be pasted without Enter: %v", f.calls)
	}
	f.fail = "pane zoom"
	a.Msg = ""
	a.ExecCommand("sh hide")
	if !a.MsgErr || !strings.Contains(a.Msg, "boom") {
		t.Fatalf("error not shown: %q", a.Msg)
	}
}

func TestHerdrOffKeepsBuiltinTerminal(t *testing.T) {
	a, f := herdrApp(t)
	a.Cfg.Experimental.Multiplexer = false
	if a.MuxOn() {
		t.Fatal("herdr must be off when the setting is off")
	}
	a.Cfg.Experimental.Multiplexer = true
	t.Setenv("HERDR_ENV", "")
	if a.MuxOn() {
		t.Fatal("herdr must be off outside herdr")
	}
	a.ToggleTerm()
	if !a.showTerm || len(f.calls) != 0 {
		t.Fatalf("built-in terminal expected: showTerm=%v calls=%v", a.showTerm, f.calls)
	}
	a.term.closeAll()
}

func TestHerdrCommandEnd(t *testing.T) {
	a, _ := herdrApp(t)
	a.mux.id = "w1-p2"
	a.resolveMux() // commandEnded needs the backend that a created pane would have
	a.CommandEnded(1)
	if len(a.Sg.Fails) != 1 || a.Sg.Fails[0].Code != 1 || len(a.Sg.Fails[0].Output) == 0 {
		t.Fatalf("fails: %+v", a.Sg.Fails)
	}
	a.ScanTerminals()
	if len(a.Sg.Fails) != 1 {
		t.Fatal("scanTerminals dropped the herdr failure")
	}
	a.CommandEnded(130)
	a.CommandEnded(0)
	if len(a.Sg.Fails) != 0 {
		t.Fatalf("success must clear: %+v", a.Sg.Fails)
	}
}

func TestHerdrShellIntegration(t *testing.T) {
	z := muxZshEnv("/opt/it's/shj")
	if strings.Contains(z, "print -n") || !strings.Contains(z, `'/opt/it'\''s/shj' command-end $s`) {
		t.Fatalf("zsh hook:\n%s", z)
	}
	if env := muxShellIntegration("/bin/bash", "/bin/shj"); len(env) != 1 || !strings.HasPrefix(env[0], "PROMPT_COMMAND=") || !strings.Contains(env[0], "'/bin/shj' command-end") {
		t.Fatalf("bash: %v", env)
	}
	if env := muxShellIntegration("/bin/fish", "/bin/shj"); env != nil {
		t.Fatalf("fish: %v", env)
	}
}

func TestMuxNoneKeepsBuiltinTerminal(t *testing.T) {
	a, f := herdrApp(t)
	a.Cfg.Terminal.Multiplexer = "none"
	if a.MuxOn() {
		t.Fatal("none must use the built-in terminal even inside herdr")
	}
	a.ToggleTerm()
	if !a.showTerm || len(f.calls) != 0 {
		t.Fatalf("built-in terminal expected: showTerm=%v calls=%v", a.showTerm, f.calls)
	}
	a.term.closeAll()
}

func TestMuxAutoFollowsParents(t *testing.T) {
	a, _ := herdrApp(t)
	a.Cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "herdr", "systemd")
	if !a.MuxOn() || a.mux.b.name() != "herdr" {
		t.Fatalf("herdr parent not used: %v", a.mux.b)
	}
}

func TestMuxAutoInnerWins(t *testing.T) {
	// herdr -> tmux -> zsh -> shj: tmux is inner (here it is not usable: TMUX is unset), so herdr must not be used.
	t.Setenv("TMUX", "")
	a, f := herdrApp(t)
	a.Cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "tmux: server", "herdr", "systemd")
	if a.MuxOn() || len(f.calls) != 0 {
		t.Fatal("the inner tmux must win over herdr")
	}
}

func TestMuxAutoNoMultiplexer(t *testing.T) {
	a, _ := herdrApp(t)
	a.Cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "alacritty", "init")
	if a.MuxOn() {
		t.Fatal("no multiplexer among the parents")
	}
}

func TestMuxUnsupportedFallsBack(t *testing.T) {
	a, _ := herdrApp(t)
	a.Cfg.Terminal.Multiplexer = "tmux"
	if a.MuxOn() || !strings.Contains(a.Msg, "tmux") {
		t.Fatalf("expected the built-in terminal and a message: %q", a.Msg)
	}
}

func fakeChain(comms ...string) func(int) (int, string, bool) {
	pids := []int{os.Getpid()}
	for i := range comms[1:] {
		pids = append(pids, 2000+i)
	}
	pids = append(pids, 0)
	return func(pid int) (int, string, bool) {
		for i, p := range pids[:len(comms)] {
			if p == pid {
				return pids[i+1], comms[i], true
			}
		}
		return 0, "", false
	}
}

func TestSystemParent(t *testing.T) {
	ppid, comm, ok := systemParent(os.Getpid())
	if !ok || ppid != os.Getppid() || comm == "" {
		t.Fatalf("ppid=%d comm=%q ok=%v", ppid, comm, ok)
	}
}

func TestMuxKindOf(t *testing.T) {
	for comm, want := range map[string]string{"herdr": "herdr", "tmux: server": "tmux", "/usr/bin/zellij": "zellij", "cmux": "cmux", "zsh": ""} {
		if got := muxKindOf(comm); got != want {
			t.Errorf("muxKindOf(%q) = %q, want %q", comm, got, want)
		}
	}
}

func TestMuxHerdrWithoutEnv(t *testing.T) {
	a, _ := herdrApp(t)
	t.Setenv("HERDR_ENV", "")
	if a.MuxOn() || strings.Contains(a.Msg, "not supported") || !strings.Contains(a.Msg, "herdr") {
		t.Fatalf("expected the unavailable message: %q", a.Msg)
	}
}

func TestMuxConfigReload(t *testing.T) {
	a, f := herdrApp(t)
	if !a.MuxOn() {
		t.Fatal("herdr expected")
	}
	a.mux.id, a.mux.visible = "w1-p2", true
	a.Cfg.Terminal.Multiplexer = "none"
	a.mux.resolved = false // what :config-reload does
	if a.MuxOn() {
		t.Fatal("none must take effect after a reload")
	}
	if !f.has("pane close w1-p2") || a.mux.id != "" {
		t.Fatalf("old pane not closed: %v", f.calls)
	}
}
