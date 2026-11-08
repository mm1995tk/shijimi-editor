package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

func ctlCall(a *App, r ctl.Request) ctl.Response {
	ch := make(chan CtlResponse, 1)
	for _, e := range a.handleCtl(CtlRequest{Req: r, Reply: ch}) {
		if rc, ok := e.(ReplyCtl); ok {
			rc.To <- rc.Resp
		}
	}
	return (<-ch).Wire(tt)
}

func TestCtlDiffTurnsOnGitDiffMode(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("1\n"), 0o644)
	a := newApp(dir, "")
	if r := ctlCall(a, ctl.Request{Cmd: "diff", Path: f}); !r.OK {
		t.Fatalf("diff: %+v", r)
	}
	if !a.Gd.On || a.CurText() == nil || a.CurText().Buf.Path != f {
		t.Errorf("on=%v cur=%v", a.Gd.On, a.CurText())
	}
}

func TestCtlExec(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.md")
	os.WriteFile(f, []byte("# a\n"), 0o644)
	a := newApp(dir, f)

	if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: ":keymap vscode"}); !r.OK || r.Message == "" || a.Helix {
		t.Errorf("keymap: %+v helix=%v", r, a.Helix)
	}
	r := ctlCall(a, ctl.Request{Cmd: "exec", Line: "nosuch"})
	if r.OK || r.Error != "cmd.unknowncmd(nosuch)" {
		t.Errorf("unknown: %+v", r)
	}
	for _, line := range []string{":q", ":q!", ":wq", ":bc!", "qa"} {
		if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: line}); r.OK || r.Error == "" {
			t.Errorf("%s should be refused: %+v", line, r)
		}
	}
	for _, line := range []string{":o", ":pr", ":pr foo", ":theme", ":symbol", ":action", ":grep", ":replace"} {
		if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: line}); r.OK || a.Pal.Open {
			t.Errorf("%s should need a human: %+v open=%v", line, r, a.Pal.Open)
		}
	}
	if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: "  "}); r.OK {
		t.Errorf("empty: %+v", r)
	}
	a.CurText().Buf.Dirty = true
	for range 2 {
		if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: ":bc"}); r.OK {
			t.Errorf("bc with unsaved changes: %+v", r)
		}
	}
	if len(a.Tabs) != 1 || a.confirmClose {
		t.Errorf("tab lost or confirmClose left on: tabs=%d confirm=%v", len(a.Tabs), a.confirmClose)
	}
	if r := ctlCall(a, ctl.Request{Cmd: "exec", Line: ":reload"}); r.OK {
		t.Errorf("reload with unsaved changes: %+v", r)
	}
}

func TestReviewToRoot(t *testing.T) {
	a := &Core{Root: "/repo/sub", GitTop: "/repo"}
	if got := a.ReviewToRoot("sub/pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("with gitTop: %q", got)
	}
	a.GitTop = ""
	if got := a.ReviewToRoot("pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("without gitTop: %q", got)
	}
}

func TestCtlOpenOpensTheFileAndFocusesTheEditor(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "b.txt")
	os.WriteFile(f, []byte("1\n"), 0o644)
	a := newApp(dir, "")
	a.Focus = FocusExplorer
	if r := ctlCall(a, ctl.Request{Cmd: "open", Path: f}); !r.OK {
		t.Fatalf("open: %+v", r)
	}
	if a.CurText() == nil || a.CurText().Buf.Path != f || a.Focus != FocusEditor {
		t.Errorf("cur=%v focus=%v", a.CurText(), a.Focus)
	}
	if r := ctlCall(a, ctl.Request{Cmd: "open", Path: "b.txt"}); r.OK {
		t.Error("a relative path must be refused")
	}
}

func TestCtlChangesKeyActsOnlyWhenTheGUIShowsTheList(t *testing.T) {
	a := newApp(t.TempDir(), "")
	if r := ctlCall(a, ctl.Request{Cmd: "changes-key", Line: "s"}); r.OK {
		t.Error("refused outside diff mode")
	}
	a.Gd.On, a.Gd.Base = true, BaseHead
	if r := ctlCall(a, ctl.Request{Cmd: "changes-key", Line: "s"}); r.OK {
		t.Error("refused when the shj shows the list itself")
	}
	a.SetChangesInGUI(true)
	if r := ctlCall(a, ctl.Request{Cmd: "changes-key", Line: "space"}); !r.OK || a.Pending != " " {
		t.Errorf("space: %+v pending=%q", r, a.Pending)
	}
	a.Pending = ""
	if r := ctlCall(a, ctl.Request{Cmd: "changes-key", Line: "r"}); !r.OK {
		t.Errorf("r: %+v", r)
	}
	if r := ctlCall(a, ctl.Request{Cmd: "changes-key", Line: "j"}); r.OK {
		t.Error("a key the GUI handles itself must be refused")
	}
}

func TestSpaceEAsksTheGUIChanges(t *testing.T) {
	a := newApp(t.TempDir(), "")
	calls := 0
	a.SetGUIChanges(func() error { calls++; return nil })
	a.Gd.On, a.Gd.Base = true, BaseHead
	a.SetChangesInGUI(true)
	a.SpaceMode("E")
	a.focusChangesList()
	if calls != 2 || a.Focus == FocusChanges {
		t.Errorf("calls=%d focus=%v", calls, a.Focus)
	}
	// The diff mode against a pull request keeps the shj's own list.
	a.Gd.Base = BasePR
	a.SpaceMode("E")
	if calls != 2 || a.Focus != FocusChanges {
		t.Errorf("PR: calls=%d focus=%v", calls, a.Focus)
	}
}
