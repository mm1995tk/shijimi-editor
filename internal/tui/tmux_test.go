package tui

import (
	"fmt"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"strings"
	"testing"
)

// fakeTmux plays the tmux server: it records the commands and keeps the zoom
// flag and the list of panes.
type fakeTmux struct {
	calls  []string
	zoomed bool
	dead   bool // the new pane has ended
	gone   bool // the new pane has been closed
	fail   string
}

func (f *fakeTmux) call(args ...string) ([]byte, error) {
	line := strings.Join(args, " ")
	f.calls = append(f.calls, line)
	if f.fail != "" && strings.Contains(line, f.fail) {
		return nil, fmt.Errorf("boom")
	}
	switch args[0] {
	case "list-panes":
		out := "%1 0\n"
		if !f.gone {
			if f.dead {
				out += "%2 1\n"
			} else {
				out += "%2 0\n"
			}
		}
		return []byte(out), nil
	case "split-window":
		f.gone, f.dead = false, false
		return []byte("%2\n"), nil
	case "display-message":
		if f.zoomed {
			return []byte("1\n"), nil
		}
		return []byte("0\n"), nil
	case "resize-pane":
		f.zoomed = !f.zoomed
	case "kill-pane":
		f.gone = true
	case "capture-pane":
		return []byte("old\nmake: *** error\n\n\n"), nil
	}
	return nil, nil
}

func (f *fakeTmux) has(s string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, s) {
			return true
		}
	}
	return false
}

func (f *fakeTmux) count(s string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, s) {
			n++
		}
	}
	return n
}

func tmuxApp(t *testing.T) (*App, *fakeTmux) {
	t.Helper()
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1,0")
	a := newTestApp(t, "hello")
	a.Cfg.Experimental.Multiplexer = true
	f := &fakeTmux{}
	a.mux.newBackend = func(kind string) muxBackend {
		if kind == "tmux" {
			return &tmuxBackend{editorPane: "%1", call: f.call}
		}
		return nil
	}
	a.Cfg.Terminal.Multiplexer = "tmux"
	return a, f
}

func TestTmuxToggle(t *testing.T) {
	a, f := tmuxApp(t)
	ctrlBacktick := core.Key{Code: '`', Mod: core.ModCtrl}
	a.Focus = core.FocusEditor
	a.Update(ctrlBacktick)
	if a.showTerm || a.term.cur() != nil {
		t.Fatal("the built-in terminal must stay closed")
	}
	if !f.has("split-window -v -l 30% -c "+a.Root+" -P -F #{pane_id}") || a.mux.id != "%2" || !a.mux.visible {
		t.Fatalf("not created: %v id=%q", f.calls, a.mux.id)
	}
	var split string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "split-window") {
			split = c
		}
	}
	if !strings.HasSuffix(split, "-t %1") {
		t.Fatalf("split must target the editor's pane: %q", split)
	}
	a.Update(ctrlBacktick)
	if !f.has("resize-pane -Z -t %1") || !f.zoomed || a.mux.visible {
		t.Fatalf("not hidden: %v", f.calls)
	}
	a.Update(ctrlBacktick)
	if f.zoomed || !f.has("select-pane -t %2") || !a.mux.visible {
		t.Fatalf("not shown again: %v", f.calls)
	}
	if n := f.count("split-window"); n != 1 {
		t.Fatalf("split %d times", n)
	}
	// An ended pane is replaced.
	f.dead = true
	a.Update(ctrlBacktick)
	if n := f.count("split-window"); n != 2 {
		t.Fatalf("ended pane: split %d times, want 2", n)
	}
	// Quitting closes the pane.
	a.Quit()
	if !f.has("kill-pane -t %2") {
		t.Fatalf("pane not closed: %v", f.calls)
	}
}

func TestTmuxHideWhenAlreadyZoomed(t *testing.T) {
	f := &fakeTmux{zoomed: true}
	b := &tmuxBackend{editorPane: "%1", call: f.call}
	if err := b.hide(); err != nil || f.has("resize-pane") {
		t.Fatalf("a zoomed window must stay zoomed: %v %v", err, f.calls)
	}
}

func TestTmuxCommands(t *testing.T) {
	a, f := tmuxApp(t)
	a.ExecCommand("sh make test")
	if !f.has("send-keys -t %2 -l -- make test") || !f.has("send-keys -t %2 Enter") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.ExecCommand("sh hide")
	if !f.zoomed {
		t.Fatalf("calls: %v", f.calls)
	}
	f.calls = nil
	a.Ctx = &core.CtxPreview{Text: "ctx"}
	a.HandleContextKey(core.Key{Code: KeyEnter})
	if !f.has("set-buffer -b shijimi -- ctx") || !f.has("paste-buffer -d -p -b shijimi -t %2") || f.has("send-keys") {
		t.Fatalf("context must be pasted without Enter: %v", f.calls)
	}
	f.fail = "resize-pane"
	a.Msg = ""
	a.ExecCommand("sh hide")
	if !a.MsgErr || !strings.Contains(a.Msg, "boom") {
		t.Fatalf("error not shown: %q", a.Msg)
	}
}

func TestTmuxSplitEnv(t *testing.T) {
	f := &fakeTmux{}
	b := &tmuxBackend{call: f.call}
	if _, err := b.split("/r", []string{"SHIJIMI_SOCKET=/s", "A=b c"}); err != nil {
		t.Fatal(err)
	}
	want := "split-window -v -l 30% -c /r -P -F #{pane_id} -e SHIJIMI_SOCKET=/s -e A=b c"
	if f.calls[0] != want {
		t.Fatalf("got  %q\nwant %q", f.calls[0], want)
	}
}

func TestTmuxOffKeepsBuiltinTerminal(t *testing.T) {
	a, f := tmuxApp(t)
	a.Cfg.Experimental.Multiplexer = false
	if a.MuxOn() {
		t.Fatal("tmux must be off when the setting is off")
	}
	a.resolveMux()
	t.Setenv("TMUX", "")
	a.Cfg.Experimental.Multiplexer = true
	a.resolveMux()
	if a.MuxOn() {
		t.Fatal("tmux must be off outside tmux")
	}
	a.ToggleTerm()
	if !a.showTerm || len(f.calls) != 0 {
		t.Fatalf("built-in terminal expected: showTerm=%v calls=%v", a.showTerm, f.calls)
	}
	a.term.closeAll()
}

func TestTmuxCommandEnd(t *testing.T) {
	a, _ := tmuxApp(t)
	a.mux.id = "%2"
	a.resolveMux()
	a.CommandEnded(1)
	if len(a.Sg.Fails) != 1 || a.Sg.Fails[0].Code != 1 {
		t.Fatalf("fails: %+v", a.Sg.Fails)
	}
	if out := a.Sg.Fails[0].Output; len(out) == 0 || out[len(out)-1] != "make: *** error" {
		t.Fatalf("output must lose the padding: %q", out)
	}
	a.CommandEnded(0)
	if len(a.Sg.Fails) != 0 {
		t.Fatalf("success must clear: %+v", a.Sg.Fails)
	}
}

func TestTmuxAutoFollowsParents(t *testing.T) {
	a, _ := tmuxApp(t)
	a.Cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "tmux: server", "systemd")
	if !a.MuxOn() || a.mux.b.name() != "tmux" {
		t.Fatalf("tmux parent not used: %v", a.mux.b)
	}
}

func TestTmuxShellEnv(t *testing.T) {
	if env := (&tmuxBackend{}).shellEnv("/bin/bash", "/bin/shj"); len(env) != 1 || !strings.HasPrefix(env[0], "PROMPT_COMMAND=") {
		t.Fatalf("bash: %v", env)
	}
}

func TestTmuxSendMultilineIsOnePaste(t *testing.T) {
	f := &fakeTmux{}
	b := &tmuxBackend{call: f.call}
	if err := b.send("%2", "a.go:1\n```\nx\n```\n", false); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || f.calls[0] != "set-buffer -b shijimi -- a.go:1\n```\nx\n```\n" || f.has("send-keys") {
		t.Fatalf("calls: %q", f.calls)
	}
	f.calls = nil
	_ = b.send("%2", "ls;", true)
	if f.calls[0] != `send-keys -t %2 -l -- ls\;` {
		t.Fatalf("trailing semicolon: %q", f.calls)
	}
}
