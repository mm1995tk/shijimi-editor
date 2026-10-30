package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touchLater(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
}

func TestWatchReloadsCleanBuffer(t *testing.T) {
	a := newTestApp(t, "one\n")
	tb := a.curText()
	touchLater(t, tb.buf.Path, "agent\n")
	a.Update(watchMsg{})
	if got := tb.buf.Text(); got != "agent\n" || tb.dirty() {
		t.Fatalf("not reloaded: %q dirty=%v", got, tb.dirty())
	}
	if a.msgErr || a.msg == "" {
		t.Fatalf("no info message: %q", a.msg)
	}
}

func TestWatchKeepsUnsavedEdits(t *testing.T) {
	a := newTestApp(t, "one\n")
	tb := a.curText()
	keys(a, "i", "x")
	touchLater(t, tb.buf.Path, "agent\n")
	a.Update(watchMsg{})
	if got := tb.buf.Text(); got != "xone\n" || !tb.conflict || !a.msgErr {
		t.Fatalf("edits lost or no warning: %q conflict=%v", got, tb.conflict)
	}
	// auto-save must not overwrite the other process's change
	a.Update(autoSaveMsg{tab: tb, ver: tb.buf.Version})
	if b, _ := os.ReadFile(tb.buf.Path); string(b) != "agent\n" {
		t.Fatalf("auto-save overwrote disk: %q", b)
	}
	// :reload takes the disk version
	a.execCommand("reload")
	if got := tb.buf.Text(); got != "agent\n" || tb.buf.DiskChanged() {
		t.Fatalf("reload: %q", got)
	}
	a.Update(watchMsg{})
	if tb.conflict {
		t.Fatal("conflict not cleared")
	}
}

func TestWatchDisabled(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.cfg.Editor.WatchFiles = false
	tb := a.curText()
	touchLater(t, tb.buf.Path, "agent\n")
	a.Update(watchMsg{})
	if tb.buf.Text() != "one\n" {
		t.Fatal("reloaded while disabled")
	}
}

func TestWatchIgnoresDeletedFile(t *testing.T) {
	a := newTestApp(t, "one\n")
	tb := a.curText()
	os.Remove(tb.buf.Path)
	a.Update(watchMsg{})
	if got := tb.buf.Text(); got != "one\n" || tb.dirty() {
		t.Fatalf("buffer changed after delete: %q", got)
	}
}

func TestWatchRefreshesTreeAfterExternalChange(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	a := newTestApp(t, "one\n")
	dir := a.explorer.root.path
	// b.txt is not open in a tab, so only the periodic refresh can notice it.
	path := filepath.Join(dir, "b.txt")
	os.WriteFile(path, []byte("b\n"), 0o644)
	if out, err := run(dir, nil, "git", "init", "-q"); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	git := []string{"-c", "user.name=t", "-c", "user.email=t@t"}
	if out, err := run(dir, nil, "git", append(git, "add", "a.txt", "b.txt")...); err != nil {
		t.Skipf("git add failed: %v %s", err, out)
	}
	if out, err := run(dir, nil, "git", append(git, "commit", "-qm", "init")...); err != nil {
		t.Skipf("git commit failed: %v %s", err, out)
	}
	a.explorer.refresh()
	if a.explorer.git[path] != "" {
		t.Fatalf("clean file marked: %q", a.explorer.git[path])
	}
	tick := func(n int) {
		for range n {
			a.Update(watchMsg{})
		}
	}
	hasNode := func(name string) bool {
		for _, n := range a.explorer.flat {
			if n.name == name {
				return true
			}
		}
		return false
	}

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o644)
	os.WriteFile(path, []byte("changed\n"), 0o644)
	tick(treeRefreshTicks - 1)
	if a.explorer.git[path] != "" || hasNode("new.txt") {
		t.Fatal("tree refreshed before treeRefreshTicks")
	}
	tick(1)
	if a.explorer.git[path] != "M" {
		t.Fatalf("modified mark missing: %q", a.explorer.git[path])
	}
	if !hasNode("new.txt") {
		t.Fatal("externally created file not in tree")
	}

	run(dir, nil, "git", "checkout", ".")
	tick(treeRefreshTicks - 1)
	if a.explorer.git[path] != "M" {
		t.Fatalf("mark cleared before treeRefreshTicks: %q", a.explorer.git[path])
	}
	tick(1)
	if a.explorer.git[path] != "" {
		t.Fatalf("stale mark: %q", a.explorer.git[path])
	}
}
