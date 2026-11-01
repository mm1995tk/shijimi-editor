package tui

import (
	"fmt"
	"strings"
	"testing"
)

// fakeZellij plays the zellij session: it records the actions and keeps the
// fullscreen flag and the list of panes.
type fakeZellij struct {
	calls      []string
	fullscreen bool
	dead       bool // the new pane has ended
	gone       bool // the new pane has been closed
	fail       string
}

func (f *fakeZellij) call(args ...string) ([]byte, error) {
	line := strings.Join(args, " ")
	f.calls = append(f.calls, line)
	if f.fail != "" && strings.Contains(line, f.fail) {
		return nil, fmt.Errorf("boom")
	}
	switch args[0] {
	case "list-panes":
		out := fmt.Sprintf(`[{"id":0,"is_plugin":true,"is_fullscreen":false,"exited":false},{"id":1,"is_plugin":false,"is_fullscreen":%v,"exited":false}`, f.fullscreen)
		if !f.gone {
			out += fmt.Sprintf(`,{"id":2,"is_plugin":false,"is_fullscreen":false,"exited":%v},{"id":2,"is_plugin":true,"is_fullscreen":false,"exited":true}`, f.dead)
		}
		return []byte(out + "]"), nil
	case "new-pane":
		f.gone, f.dead = false, false
		return []byte("terminal_2\n"), nil
	case "toggle-fullscreen":
		f.fullscreen = !f.fullscreen
	case "close-pane":
		f.gone = true
	case "dump-screen":
		return []byte("old\nmake: *** error\n\n\n"), nil
	}
	return nil, nil
}

func (f *fakeZellij) has(s string) bool { return f.count(s) > 0 }

func (f *fakeZellij) count(s string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, s) {
			n++
		}
	}
	return n
}

func zellijApp(t *testing.T) (*App, *fakeZellij) {
	t.Helper()
	t.Setenv("ZELLIJ", "0")
	t.Setenv("SHELL", "/bin/zsh")
	a := newTestApp(t, "hello")
	a.cfg.Experimental.Multiplexer = true
	f := &fakeZellij{}
	a.mux.newBackend = func(kind string) muxBackend {
		if kind == "zellij" {
			return &zellijBackend{editorPane: "terminal_1", call: f.call}
		}
		return nil
	}
	a.cfg.Terminal.Multiplexer = "zellij"
	return a, f
}

func TestZellijToggle(t *testing.T) {
	a, f := zellijApp(t)
	ctrlBacktick := Key{Code: '`', Mod: ModCtrl}
	a.focus = focusEditor
	a.Update(ctrlBacktick)
	if a.showTerm || a.term.cur() != nil {
		t.Fatal("the built-in terminal must stay closed")
	}
	if !f.has("new-pane --direction down --close-on-exit --near-current-pane --cwd "+a.root+" -- env ") ||
		!f.has("focus-pane-id terminal_2") || a.mux.id != "terminal_2" || !a.mux.visible {
		t.Fatalf("not created: %v id=%q", f.calls, a.mux.id)
	}
	a.Update(ctrlBacktick)
	if !f.has("toggle-fullscreen --pane-id terminal_1") || !f.fullscreen || a.mux.visible {
		t.Fatalf("not hidden: %v", f.calls)
	}
	a.Update(ctrlBacktick)
	if f.fullscreen || a.mux.visible != true || f.count("focus-pane-id terminal_2") != 2 {
		t.Fatalf("not shown again: %v", f.calls)
	}
	if n := f.count("new-pane"); n != 1 {
		t.Fatalf("created %d times", n)
	}
	// An ended pane is replaced.
	f.dead = true
	a.Update(ctrlBacktick)
	if n := f.count("new-pane"); n != 2 {
		t.Fatalf("ended pane: created %d times, want 2", n)
	}
	// Quitting closes the pane.
	a.quit()
	if !f.has("close-pane --pane-id terminal_2") {
		t.Fatalf("pane not closed: %v", f.calls)
	}
}

func TestZellijHideWhenAlreadyFullscreen(t *testing.T) {
	f := &fakeZellij{fullscreen: true}
	b := &zellijBackend{editorPane: "terminal_1", call: f.call}
	if err := b.hide(); err != nil || f.has("toggle-fullscreen") {
		t.Fatalf("a fullscreen pane must stay so: %v %v", err, f.calls)
	}
}

func TestZellijCommands(t *testing.T) {
	a, f := zellijApp(t)
	a.execCommand("sh make test")
	if !f.has("write-chars --pane-id terminal_2 -- make test") || !f.has("write --pane-id terminal_2 13") {
		t.Fatalf("calls: %v", f.calls)
	}
	a.execCommand("sh hide")
	if !f.fullscreen {
		t.Fatalf("calls: %v", f.calls)
	}
	f.calls = nil
	a.ctx = &ctxPreview{text: "ctx"}
	a.handleContextKey(Key{Code: KeyEnter})
	if !f.has("paste --pane-id terminal_2 -- ctx") || f.has("write") {
		t.Fatalf("context must be pasted without Enter: %v", f.calls)
	}
	f.fail = "toggle-fullscreen"
	a.msg = ""
	a.execCommand("sh hide")
	if !a.msgErr || !strings.Contains(a.msg, "boom") {
		t.Fatalf("error not shown: %q", a.msg)
	}
}

func TestZellijSplitEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	f := &fakeZellij{}
	b := &zellijBackend{call: f.call}
	id, err := b.split("/r", []string{"SHIJIMI_SOCKET=/s", "A=b c"})
	if err != nil || id != "terminal_2" {
		t.Fatalf("%q %v", id, err)
	}
	want := "new-pane --direction down --close-on-exit --near-current-pane --cwd /r -- env SHIJIMI_SOCKET=/s A=b c /bin/zsh"
	if f.calls[0] != want {
		t.Fatalf("got  %q\nwant %q", f.calls[0], want)
	}
	if f.calls[1] != "focus-pane-id terminal_2" {
		t.Fatalf("focus: %v", f.calls)
	}
}

func TestZellijBadReply(t *testing.T) {
	b := &zellijBackend{call: func(args ...string) ([]byte, error) { return []byte("ok\n"), nil }}
	if _, err := b.split("/r", nil); err == nil {
		t.Fatal("a reply without a pane id must fail")
	}
	if b.alive("terminal_2") {
		t.Fatal("an unreadable list has no live pane")
	}
}

func TestZellijOffKeepsBuiltinTerminal(t *testing.T) {
	a, f := zellijApp(t)
	a.cfg.Experimental.Multiplexer = false
	if a.muxOn() {
		t.Fatal("zellij must be off when the setting is off")
	}
	a.resolveMux()
	t.Setenv("ZELLIJ", "")
	a.cfg.Experimental.Multiplexer = true
	a.resolveMux()
	if a.muxOn() {
		t.Fatal("zellij must be off outside zellij")
	}
	t.Setenv("SHELL", "/bin/sh") // a shell that fails to start closes the panel at once
	a.toggleTerm()
	if !a.showTerm || len(f.calls) != 0 {
		t.Fatalf("built-in terminal expected: showTerm=%v calls=%v", a.showTerm, f.calls)
	}
	a.term.closeAll()
}

func TestZellijCommandEnd(t *testing.T) {
	a, _ := zellijApp(t)
	a.mux.id = "terminal_2"
	a.resolveMux()
	a.commandEnded(1)
	if len(a.sg.fails) != 1 || a.sg.fails[0].code != 1 {
		t.Fatalf("fails: %+v", a.sg.fails)
	}
	if out := a.sg.fails[0].output; len(out) == 0 || out[len(out)-1] != "make: *** error" {
		t.Fatalf("output must lose the padding: %q", out)
	}
	a.commandEnded(0)
	if len(a.sg.fails) != 0 {
		t.Fatalf("success must clear: %+v", a.sg.fails)
	}
}

func TestZellijAutoFollowsParents(t *testing.T) {
	a, _ := zellijApp(t)
	a.cfg.Terminal.Multiplexer = "auto"
	a.mux.parents = fakeChain("shj", "zsh", "zellij", "systemd")
	if !a.muxOn() || a.mux.b.name() != "zellij" {
		t.Fatalf("zellij parent not used: %v", a.mux.b)
	}
}

func TestZellijShellEnv(t *testing.T) {
	if env := (&zellijBackend{}).shellEnv("/bin/bash", "/bin/shj"); len(env) != 1 || !strings.HasPrefix(env[0], "PROMPT_COMMAND=") {
		t.Fatalf("bash: %v", env)
	}
}

func TestZellijPaneName(t *testing.T) {
	for in, want := range map[string]string{"3": "terminal_3", "terminal_4": "terminal_4"} {
		if got := zellijPaneName(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
