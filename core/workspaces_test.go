package core

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/internal/direnv"

	"github.com/mm1995tk/shijimi-editor/core/screen"
)

// writeFile makes dir/name holding text.
func writeFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Two workspaces in one process each have a core of their own: the files one
// opens and edits are not the other's, and both share the process's parts.
func TestWorkspacesEditSeparately(t *testing.T) {
	r1, r2 := t.TempDir(), t.TempDir()
	p1 := writeFile(t, r1, "a.txt", "one\n")
	p2 := writeFile(t, r2, "a.txt", "two\n")

	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	w1, w2 := openWS(ws, r1), openWS(ws, r2)
	if w1 == w2 {
		t.Fatal("two roots got the same workspace")
	}
	if openWS(ws, r1) != w1 {
		t.Fatal("opening a root again made another core")
	}

	w1.Do(func(c *Core) {
		if !c.OpenFile(p1) {
			t.Fatalf("open %s: %s", p1, c.Msg)
		}
		c.Focus = FocusEditor
		keys(c, "i", "X", "esc")
	})
	w2.Do(func(c *Core) {
		if !c.OpenFile(p2) {
			t.Fatalf("open %s: %s", p2, c.Msg)
		}
		c.Focus = FocusEditor
		keys(c, "i", "Y", "esc")
	})

	var c1, c2 *Core
	w1.Do(func(c *Core) { c1 = c })
	w2.Do(func(c *Core) { c2 = c })
	if c1.Root == c2.Root {
		t.Fatalf("both cores are rooted at %s", c1.Root)
	}
	if got := c1.CurText().Buf.Text(); got != "Xone\n" {
		t.Errorf("workspace 1 = %q, want %q", got, "Xone\n")
	}
	if got := c2.CurText().Buf.Text(); got != "Ytwo\n" {
		t.Errorf("workspace 2 = %q, want %q", got, "Ytwo\n")
	}
	if !slices.Equal(c1.OpenPaths(), []string{p1}) || !slices.Equal(c2.OpenPaths(), []string{p2}) {
		t.Errorf("open files = %v and %v, want [%s] and [%s]", c1.OpenPaths(), c2.OpenPaths(), p1, p2)
	}
	if c1.shared != c2.shared {
		t.Error("the cores do not share the process's parts")
	}

	// Closing one workspace leaves the other's core and files alone.
	ws.Close(r1)
	if ws.Lookup(r1) != nil {
		t.Error("the closed workspace is still there")
	}
	if ws.Lookup(r2) != w2 || len(c2.OpenPaths()) != 1 {
		t.Error("closing workspace 1 touched workspace 2")
	}
	if b, _ := os.ReadFile(p1); string(b) != "one\n" {
		t.Errorf("closing wrote the file: %q", b)
	}
}

// Before the server stops, the unsaved edits of every workspace go next to
// their files as <name>.bk; files without edits get nothing, and the files
// themselves stay as they were.
func TestWorkspacesWriteBackups(t *testing.T) {
	r1, r2 := t.TempDir(), t.TempDir()
	p1 := writeFile(t, r1, "a.txt", "one\n")
	p2 := writeFile(t, r2, "b.txt", "two\n")
	clean := writeFile(t, r2, "c.txt", "three\n")

	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	openWS(ws, r1).Do(func(c *Core) {
		c.OpenFile(p1)
		c.CurText().Buf.Lines[0] = []rune("ONE")
		c.CurText().Buf.Dirty = true
	})
	openWS(ws, r2).Do(func(c *Core) {
		c.OpenFile(clean)
		c.OpenFile(p2)
		c.CurText().Buf.Lines[0] = []rune("TWO")
		c.CurText().Buf.Dirty = true
	})

	write := func(path string, data []byte) (string, error) {
		name := path + ".bk"
		return name, os.WriteFile(name, data, 0o600)
	}
	got, err := ws.WriteBackups(write)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{p1 + ".bk", p2 + ".bk"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("written = %v, want %v", got, want)
	}
	for p, text := range map[string]string{p1 + ".bk": "ONE\n", p2 + ".bk": "TWO\n", p1: "one\n", p2: "two\n"} {
		if b, _ := os.ReadFile(p); string(b) != text {
			t.Errorf("%s = %q, want %q", p, b, text)
		}
	}
	if _, err := os.Stat(clean + ".bk"); err == nil {
		t.Error("a file with no edits got a backup")
	}
}

// Two workspaces can point at the same folder: each has a core of its own,
// named by its key.
func TestWorkspacesSameFolder(t *testing.T) {
	r := t.TempDir()
	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	a, err := ws.Open("a", r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ws.Open("b", r)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a.Root != b.Root || ws.Lookup("a") != a || ws.Lookup("b") != b {
		t.Fatal("two workspaces on one folder share a core or are not both there")
	}
	ws.Close("a")
	if ws.Lookup("a") != nil || ws.Lookup("b") != b {
		t.Error("closing one touched the other")
	}
}

// SaveFiles saves the files with edits and keeps them open; a file that
// changed on disk meanwhile is reported, not overwritten.
func TestSaveFiles(t *testing.T) {
	r := t.TempDir()
	p1 := writeFile(t, r, "a.txt", "one\n")
	p2 := writeFile(t, r, "b.txt", "two\n")
	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	openWS(ws, r).Do(func(c *Core) {
		c.OpenFile(p1)
		c.CurText().Buf.Lines[0] = []rune("ONE")
		c.CurText().Buf.Dirty = true
		c.OpenFile(p2)
		if failed := c.SaveFiles(); len(failed) != 0 {
			t.Fatalf("failed = %v", failed)
		}
		if b, _ := os.ReadFile(p1); string(b) != "ONE\n" {
			t.Errorf("a.txt = %q, want it saved", b)
		}
		if len(c.OpenPaths()) != 2 {
			t.Errorf("open files = %v, want both still open", c.OpenPaths())
		}
	})
}

// openWS is ws.Open for a configuration that must be valid.
func openWS(ws *Workspaces, root string) *Workspace {
	w, err := ws.Open(root, root)
	if err != nil {
		panic(err)
	}
	return w
}

// Two workspaces on one folder that open the same file share its text: what
// one types is in the other before it is saved, the cursors are each one's
// own, and closing the file in one leaves it open in the other.
func TestWorkspacesShareBuffer(t *testing.T) {
	root := t.TempDir()
	p := writeFile(t, root, "a.txt", "one\ntwo\nthree\n")
	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	wa, err := ws.Open("a", root)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := ws.Open("b", root)
	if err != nil {
		t.Fatal(err)
	}

	wa.Do(func(c *Core) {
		if !c.OpenFile(p) {
			t.Fatal(c.Msg)
		}
	})
	wb.Do(func(c *Core) {
		if !c.OpenFile(p) {
			t.Fatal(c.Msg)
		}
		c.Focus = FocusEditor
		keys(c, "j", "j") // B's cursor is on "three"
	})
	wa.Do(func(c *Core) {
		c.Focus = FocusEditor
		keys(c, "i", "X", "esc")
	})

	wb.Do(func(c *Core) {
		b := c.CurText().Buf
		if got := b.Text(); got != "Xone\ntwo\nthree\n" {
			t.Errorf("workspace B = %q, want A's edit before any save", got)
		}
		if b.Cur.Row != 2 {
			t.Errorf("B's cursor row = %d, want 2: A's typing moved it", b.Cur.Row)
		}
		c.BuildScreen(screen.DefaultView)
	})
	wa.Do(func(c *Core) {
		if row := c.CurText().Buf.Cur.Row; row != 0 {
			t.Errorf("A's cursor row = %d, want 0", row)
		}
	})
	if b, _ := os.ReadFile(p); string(b) != "one\ntwo\nthree\n" {
		t.Errorf("the file was written without a save: %q", b)
	}

	// A line added above B's cursor moves it along with its text.
	wa.Do(func(c *Core) { keys(c, "o", "new", "esc") })
	wb.Do(func(c *Core) {
		b := c.CurText().Buf
		c.settleViews()
		if b.Cur.Row != 3 || string(b.Lines[b.Cur.Row]) != "three" {
			t.Errorf("B's cursor = %+v on %q, want to stay on three", b.Cur, b.Lines[min(b.Cur.Row, len(b.Lines)-1)])
		}
	})

	// Saving in one clears the other's unsaved mark.
	wa.Do(func(c *Core) {
		if failed := c.SaveFiles(); len(failed) > 0 {
			t.Fatal(failed)
		}
	})
	wb.Do(func(c *Core) {
		if c.CurText().Buf.Dirty {
			t.Error("B still sees the text as unsaved after A saved it")
		}
	})

	// Closing it in A leaves it open in B, with the edits.
	wa.Do(func(c *Core) { c.CloseTab(true) })
	wb.Do(func(c *Core) {
		if len(c.Tabs) != 1 || c.CurText().Buf.Views() != 1 {
			t.Fatalf("B has %d tabs after A closed the file", len(c.Tabs))
		}
		keys(c, "i", "Y", "esc")
		if got := c.CurText().Buf.Lines[3]; string(got) != "Ythree" {
			t.Errorf("B could not go on editing: %q", got)
		}
	})
	// Closing the workspace drops its view, and the last one frees the text.
	ws.Close("b")
	if ws.shared.Buffers.Len() != 0 {
		t.Errorf("%d files are still held after every workspace closed", ws.shared.Buffers.Len())
	}
}

// A workspace whose folder has an .envrc waits for direnv, then starts its
// language server from the PATH direnv made.
func TestWorkspaceDirenvEnv(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "direnv"), []byte("#!/bin/sh\necho '{\"PATH\":\"/envbin\",\"X\":\"1\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	root := t.TempDir()
	writeFile(t, root, ".envrc", "use flake\n")

	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	w := openWS(ws, root)
	deadline := time.Now().Add(5 * time.Second)
	for {
		var env []string
		var waiting bool
		w.Do(func(c *Core) { env, waiting = c.Lsp.env, c.Lsp.waitEnv })
		if !waiting {
			if !slices.Contains(env, "PATH=/envbin") {
				t.Fatalf("env not applied: %v", env)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("direnv was not read")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// With no .envrc nothing changes: no waiting, the process's environment.
func TestWorkspaceNoEnvrc(t *testing.T) {
	ws := NewWorkspaces(NewShared(testBuild()))
	defer ws.CloseAll()
	w := openWS(ws, t.TempDir())
	w.Do(func(c *Core) {
		if c.Lsp.waitEnv || c.Lsp.env != nil {
			t.Fatal("environment was touched")
		}
	})
}

// While direnv is being read no server starts; once blocked, the process's PATH is used.
func TestDirenvBlocked(t *testing.T) {
	c := &Core{Lsp: newLSPManager(nil, "")}
	c.Lsp.waitEnv = true
	c.handleDirenvLoaded(DirenvLoaded{Err: direnv.ErrBlocked})
	if c.Lsp.waitEnv || c.Lsp.env != nil || c.Msg.Key != "direnv.blocked" {
		t.Fatalf("waitEnv=%v env=%v msg=%v", c.Lsp.waitEnv, c.Lsp.env, c.Msg)
	}
}

// fakeNix puts a `nix` on PATH (and only it) that prints a dev shell with
// PATH=/flakebin and appends a line to the returned log on every run.
func fakeNix(t *testing.T, withDirenv bool) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "nix.log")
	script := "#!/bin/sh\necho run >> " + log + "\necho '{\"variables\":{\"PATH\":{\"type\":\"exported\",\"value\":\"/flakebin\"}}}'\n"
	if err := os.WriteFile(filepath.Join(bin, "nix"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if withDirenv {
		if err := os.WriteFile(filepath.Join(bin, "direnv"), []byte("#!/bin/sh\necho '{}'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return log
}

func nixRuns(log string) int {
	data, _ := os.ReadFile(log)
	return strings.Count(string(data), "run")
}

func waitEnvRead(t *testing.T, w *Workspace) (env []string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		w.Do(func(c *Core) { env, waiting = c.Lsp.env, c.Lsp.waitEnv })
		if !waiting {
			return env
		}
		if time.Now().After(deadline) {
			t.Fatal("the environment was not read")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Without an .envrc, a flake.nix is asked about once: yes reads the dev shell,
// and the answer is kept, so the next time it is read without asking.
func TestWorkspaceNixAskOnce(t *testing.T) {
	log := fakeNix(t, false)
	root := t.TempDir()
	writeFile(t, root, "flake.nix", "{}\n")
	state := t.TempDir()
	build := Build{StateDir: state}

	ws := NewWorkspaces(NewShared(build))
	w := openWS(ws, root)
	var asked, waiting bool
	w.Do(func(c *Core) { asked, waiting = c.Confirm != nil, c.Lsp.waitEnv })
	if !asked || !waiting || nixRuns(log) != 0 {
		t.Fatalf("asked=%v waiting=%v runs=%d, want a question and no nix yet", asked, waiting, nixRuns(log))
	}
	w.Update(textKey("y"))
	if env := waitEnvRead(t, w); !slices.Contains(env, "PATH=/flakebin") {
		t.Fatalf("env not applied: %v", env)
	}
	ws.CloseAll()

	ws = NewWorkspaces(NewShared(build))
	defer ws.CloseAll()
	w = openWS(ws, root)
	w.Do(func(c *Core) { asked = c.Confirm != nil })
	if asked {
		t.Fatal("asked again")
	}
	if env := waitEnvRead(t, w); !slices.Contains(env, "PATH=/flakebin") {
		t.Fatalf("env not applied the second time: %v", env)
	}
}

// A no is kept too: the folder is not asked again and nix does not run.
func TestWorkspaceNixNo(t *testing.T) {
	log := fakeNix(t, false)
	root := t.TempDir()
	writeFile(t, root, "flake.nix", "{}\n")
	build := Build{StateDir: t.TempDir()}

	ws := NewWorkspaces(NewShared(build))
	w := openWS(ws, root)
	w.Update(textKey("n"))
	w.Do(func(c *Core) {
		if c.Confirm != nil || c.Lsp.waitEnv || c.Lsp.env != nil {
			t.Fatalf("confirm=%v waitEnv=%v env=%v", c.Confirm, c.Lsp.waitEnv, c.Lsp.env)
		}
	})
	ws.CloseAll()

	ws = NewWorkspaces(NewShared(build))
	defer ws.CloseAll()
	w = openWS(ws, root)
	w.Do(func(c *Core) {
		if c.Confirm != nil || c.Lsp.waitEnv {
			t.Fatalf("asked or waited again: confirm=%v waitEnv=%v", c.Confirm, c.Lsp.waitEnv)
		}
	})
	if nixRuns(log) != 0 {
		t.Fatal("nix ran for a folder answered no")
	}
}

// With an .envrc (and direnv installed) the flake is not looked at.
func TestWorkspaceEnvrcSkipsNix(t *testing.T) {
	log := fakeNix(t, true)
	root := t.TempDir()
	writeFile(t, root, ".envrc", "use flake\n")
	writeFile(t, root, "flake.nix", "{}\n")
	ws := NewWorkspaces(NewShared(Build{StateDir: t.TempDir()}))
	defer ws.CloseAll()
	w := openWS(ws, root)
	waitEnvRead(t, w)
	w.Do(func(c *Core) {
		if c.Confirm != nil {
			t.Fatal("asked about the flake")
		}
	})
	if nixRuns(log) != 0 {
		t.Fatal("nix ran")
	}
}
