package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/gh-editor/internal/ctl"
	"github.com/mm1995tk/gh-editor/internal/diff"
)

func TestHandleCtlOpenAndStatus(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("1\n2\n3\n"), 0o644)
	a := New(dir, "")
	call := func(r ctl.Request) ctl.Response {
		ch := make(chan ctl.Response, 1)
		a.handleCtl(ctlMsg{req: r, reply: ch})
		return <-ch
	}
	if r := call(ctl.Request{Cmd: "open", Path: f, Line: 3}); !r.OK {
		t.Fatalf("open: %+v", r)
	}
	st := call(ctl.Request{Cmd: "status"}).Status
	if st.Active != f || st.Line != 3 || len(st.Tabs) != 1 {
		t.Errorf("status = %+v", st)
	}
	if r := call(ctl.Request{Cmd: "bogus"}); r.OK {
		t.Errorf("bogus accepted")
	}
}

func ctlDiffFiles() []*diff.File {
	return []*diff.File{{NewPath: "a/x.go"}, {NewPath: "b/y.go"}, {NewPath: "c/z.go"}}
}

func ctlCall(a *App, r ctl.Request) ctl.Response {
	ch := make(chan ctl.Response, 1)
	a.handleCtl(ctlMsg{req: r, reply: ch})
	return <-ch
}

func TestReviewPath(t *testing.T) {
	a := &App{root: "/repo/sub", gitTop: "/repo"}
	if got := a.reviewPath("/repo/sub/pkg/f.go"); got != "sub/pkg/f.go" {
		t.Errorf("with gitTop: %q", got)
	}
	a.gitTop = ""
	if got := a.reviewPath("/repo/sub/pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("without gitTop: %q", got)
	}
}

func TestCtlDiffBeforeReviewLoaded(t *testing.T) {
	a := New(t.TempDir(), "")
	a.gitTop = a.root
	if r := ctlCall(a, ctl.Request{Cmd: "diff", Path: filepath.Join(a.root, "b/y.go")}); !r.OK {
		t.Fatalf("diff: %+v", r)
	}
	rev := a.tabs[a.active].rev
	rev.applyLoaded(reviewLoadedMsg{id: rev.id, files: ctlDiffFiles()})
	if rev.fileSel != 1 || rev.want != "" {
		t.Errorf("fileSel=%d want=%q", rev.fileSel, rev.want)
	}
}

func TestCtlDiffAfterReviewLoaded(t *testing.T) {
	a := New(t.TempDir(), "")
	a.gitTop = a.root
	ctlCall(a, ctl.Request{Cmd: "diff"})
	rev := a.tabs[a.active].rev
	rev.applyLoaded(reviewLoadedMsg{id: rev.id, files: ctlDiffFiles()})
	if rev.fileSel != 0 {
		t.Fatalf("fileSel=%d", rev.fileSel)
	}
	ctlCall(a, ctl.Request{Cmd: "diff", Path: filepath.Join(a.root, "c/z.go")})
	if rev.fileSel != 2 || rev.want != "" {
		t.Fatalf("immediate: fileSel=%d want=%q", rev.fileSel, rev.want)
	}
	rev.applyLoaded(reviewLoadedMsg{id: rev.id, files: ctlDiffFiles()})
	if rev.fileSel != 2 {
		t.Errorf("after reload: fileSel=%d", rev.fileSel)
	}
}
