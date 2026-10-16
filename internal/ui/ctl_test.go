package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/gh-editor/internal/ctl"
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
