package ui

import (
	"fmt"
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
	a.cfg.Experimental.Herdr = true
	f := &fakeHerdr{}
	a.hd.call = f.call
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
	if !f.has("pane split --current --direction down --ratio 0.3 --cwd "+a.root+" --focus") || a.hd.id != "w1-p2" || !a.hd.visible {
		t.Fatalf("not created: %v id=%q", f.calls, a.hd.id)
	}
	a.Update(ctrlBacktick)
	if !f.has("pane zoom --current --on") || a.hd.visible {
		t.Fatalf("not hidden: %v", f.calls)
	}
	a.Update(ctrlBacktick)
	if !f.has("pane zoom --current --off") || !f.has("pane focus --current --direction down") || !a.hd.visible {
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
	if !f.has("pane run w1-p2 -- make test") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.execCommand("sh hide")
	if !f.has("pane zoom --current --on") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.ctx = &ctxPreview{text: "ctx"}
	a.handleContextKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !f.has("pane send-text w1-p2 -- ctx") || f.has("pane run w1-p2 -- ctx") {
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
	a.cfg.Experimental.Herdr = false
	if a.herdrOn() {
		t.Fatal("herdr must be off when the setting is off")
	}
	a.cfg.Experimental.Herdr = true
	t.Setenv("HERDR_ENV", "")
	if a.herdrOn() {
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
	a.hd.id = "w1-p2"
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
