package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/gh-editor/internal/ctl"
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
