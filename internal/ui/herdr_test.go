package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	a.cfg.Experimental.Multiplexer = true
	f := &fakeHerdr{}
	a.mux.herdrCall = f.call
	a.cfg.Terminal.Multiplexer = "herdr"
	return a, f
}

func TestHerdrToggle(t *testing.T) {
	a, f := herdrApp(t)
	ctrlBacktick := tea.KeyPressMsg{Code: '`', Mod: tea.ModCtrl}
	a.focus = focusEditor
	a.Update(ctrlBacktick)
	if a.showTerm || a.term.cur() != nil {
		t.Fatal("the built-in terminal must stay closed")
	}
	if !f.has("pane split --current --direction down --ratio 0.3 --cwd "+a.root+" --focus") || a.mux.id != "w1-p2" || !a.mux.visible {
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
	a.quit()
	if !f.has("pane close w1-p2") {
		t.Fatalf("pane not closed: %v", f.calls)
	}
}

func TestHerdrCommands(t *testing.T) {
	a, f := herdrApp(t)
	a.execCommand("sh make test")
	if !f.has("pane run w1-p2 make test") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.execCommand("sh hide")
	if !f.has("pane zoom --current --on") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.ctx = &ctxPreview{text: "ctx"}
	a.handleContextKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !f.has("pane send-text w1-p2 ctx") || f.has("pane run w1-p2 ctx") {
		t.Fatalf("context must be pasted without Enter: %v", f.calls)
	}
	f.fail = "pane zoom"
	a.msg = ""
	a.execCommand("sh hide")
	if !a.msgErr || !strings.Contains(a.msg, "boom") {
		t.Fatalf("error not shown: %q", a.msg)
	}
}

func TestHerdrOffKeepsBuiltinTerminal(t *testing.T) {
	a, f := herdrApp(t)
	a.cfg.Experimental.Multiplexer = false
	if a.muxOn() {
		t.Fatal("herdr must be off when the setting is off")
	}
	a.cfg.Experimental.Multiplexer = true
	t.Setenv("HERDR_ENV", "")
	if a.muxOn() {
		t.Fatal("herdr must be off outside herdr")
	}
	a.toggleTerm()
	if !a.showTerm || len(f.calls) != 0 {
		t.Fatalf("built-in terminal expected: showTerm=%v calls=%v", a.showTerm, f.calls)
	}
	a.term.closeAll()
}

func TestHerdrCommandEnd(t *testing.T) {
	a, _ := herdrApp(t)
	a.mux.id = "w1-p2"
	a.muxOn()
	a.commandEnded(1)
	if len(a.sg.fails) != 1 || a.sg.fails[0].code != 1 || len(a.sg.fails[0].output) == 0 {
		t.Fatalf("fails: %+v", a.sg.fails)
	}
	a.scanTerminals()
	if len(a.sg.fails) != 1 {
		t.Fatal("scanTerminals dropped the herdr failure")
	}
	a.commandEnded(130)
	a.commandEnded(0)
	if len(a.sg.fails) != 0 {
		t.Fatalf("success must clear: %+v", a.sg.fails)
	}
}

func TestHerdrShellIntegration(t *testing.T) {
	z := herdrZshEnv("/opt/it's/shj")
	if strings.Contains(z, "print -n") || !strings.Contains(z, `'/opt/it'\''s/shj' command-end $s`) {
		t.Fatalf("zsh hook:\n%s", z)
	}
	if env := herdrShellIntegration("/bin/bash", "/bin/shj"); len(env) != 1 || !strings.HasPrefix(env[0], "PROMPT_COMMAND=") || !strings.Contains(env[0], "'/bin/shj' command-end") {
		t.Fatalf("bash: %v", env)
	}
	if env := herdrShellIntegration("/bin/fish", "/bin/shj"); env != nil {
		t.Fatalf("fish: %v", env)
	}
}

func TestMuxNoneKeepsBuiltinTerminal(t *testing.T) {
	a, f := herdrApp(t)
	a.cfg.Terminal.Multiplexer = "none"
	if a.muxOn() {
		t.Fatal("none must use the built-in terminal even inside herdr")
	}
	a.toggleTerm()
	if !a.showTerm || len(f.calls) != 0 {
		t.Fatalf("built-in terminal expected: showTerm=%v calls=%v", a.showTerm, f.calls)
	}
	a.term.closeAll()
}

func TestMuxAutoFollowsParents(t *testing.T) {
	a, _ := herdrApp(t)
	a.cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "herdr", "systemd")
	if !a.muxOn() || a.mux.b.name() != "herdr" {
		t.Fatalf("herdr parent not used: %v", a.mux.b)
	}
}

func TestMuxAutoInnerWins(t *testing.T) {
	// herdr -> tmux -> zsh -> shj: tmux is inner, and it is not supported yet.
	a, f := herdrApp(t)
	a.cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "tmux: server", "herdr", "systemd")
	if a.muxOn() || len(f.calls) != 0 {
		t.Fatal("the inner tmux must win over herdr")
	}
}

func TestMuxAutoNoMultiplexer(t *testing.T) {
	a, _ := herdrApp(t)
	a.cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "alacritty", "init")
	if a.muxOn() {
		t.Fatal("no multiplexer among the parents")
	}
}

func TestMuxUnsupportedFallsBack(t *testing.T) {
	a, _ := herdrApp(t)
	a.cfg.Terminal.Multiplexer = "tmux"
	if a.muxOn() || !strings.Contains(a.msg, "tmux") {
		t.Fatalf("expected the built-in terminal and a message: %q", a.msg)
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
