package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
)

func ctlCall(a *App, r ctl.Request) ctl.Response {
	ch := make(chan ctl.Response, 1)
	a.handleCtl(ctlMsg{req: r, reply: ch})
	return <-ch
}

func TestReviewToRoot(t *testing.T) {
	a := &App{root: "/repo/sub", gitTop: "/repo"}
	if got := a.reviewToRoot("sub/pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("with gitTop: %q", got)
	}
	a.gitTop = ""
	if got := a.reviewToRoot("pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("without gitTop: %q", got)
	}
}

func TestCtlDiffTurnsOnGitDiffMode(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("1\n"), 0o644)
	a := New(dir, "")
	if r := ctlCall(a, ctl.Request{Cmd: "diff", Path: f}); !r.OK {
		t.Fatalf("diff: %+v", r)
	}
	if !a.gd.on || a.curText() == nil || a.curText().buf.Path != f {
		t.Errorf("on=%v cur=%v", a.gd.on, a.curText())
	}
}

func TestCtlExec(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.md")
	os.WriteFile(f, []byte("# a\n"), 0o644)
	a := New(dir, f)

	if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: ":keymap vscode"}); !r.OK || r.Message == "" || a.helix {
		t.Errorf("keymap: %+v helix=%v", r, a.helix)
	}
	r := ctlCall(a, ctl.Request{Cmd: "exec", Line: "nosuch"})
	if r.OK || r.Error != "不明なコマンド: nosuch" {
		t.Errorf("unknown: %+v", r)
	}
	for _, line := range []string{":q", ":q!", ":wq", ":bc!", "qa"} {
		if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: line}); r.OK || r.Error == "" {
			t.Errorf("%s should be refused: %+v", line, r)
		}
	}
	for _, line := range []string{":o", ":pr", ":pr foo", ":theme", ":symbol", ":action", ":grep", ":replace"} {
		if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: line}); r.OK || a.pal.open {
			t.Errorf("%s should need a human: %+v open=%v", line, r, a.pal.open)
		}
	}
	if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: "  "}); r.OK {
		t.Errorf("empty: %+v", r)
	}
	a.curText().buf.Dirty = true
	for i := 0; i < 2; i++ {
		if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: ":bc"}); r.OK {
			t.Errorf("bc with unsaved changes: %+v", r)
		}
	}
	if len(a.tabs) != 1 || a.confirmClose {
		t.Errorf("tab lost or confirmClose left on: tabs=%d confirm=%v", len(a.tabs), a.confirmClose)
	}
	if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: ":reload"}); r.OK {
		t.Errorf("reload with unsaved changes: %+v", r)
	}
}
