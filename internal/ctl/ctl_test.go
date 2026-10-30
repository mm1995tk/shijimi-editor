package ctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/channel"
)

func TestIsSubcommand(t *testing.T) {
	for _, s := range []string{"diff", "exec", "skill"} {
		if !IsSubcommand(s) {
			t.Errorf("%s should be a subcommand", s)
		}
	}
	for _, s := range []string{"./diff", "main.go", "open", "status", ""} {
		if IsSubcommand(s) {
			t.Errorf("%q should not be a subcommand", s)
		}
	}
}

func TestServerRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "ghe")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s.sock")
	srv, err := Listen(sock, func(r Request) Response {
		if r.Cmd == "diff" {
			return Response{OK: true}
		}
		return Response{Error: "bad " + r.Cmd}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	st, _ := os.Stat(sock)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("socket mode = %v, want 0600", st.Mode().Perm())
	}
	t.Setenv(EnvSocket, sock)
	resp, err := Call(Request{Cmd: "diff"})
	if err != nil || !resp.OK {
		t.Fatalf("diff: %+v %v", resp, err)
	}
	resp, _ = Call(Request{Cmd: "x"})
	if resp.OK || resp.Error != "bad x" {
		t.Fatalf("error: %+v", resp)
	}
}

func TestCallWithoutSocket(t *testing.T) {
	isolate(t)
	if _, err := Call(Request{Cmd: "diff"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSkillInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x", "shijimi")
	var out, errOut []string
	code := Run([]string{"skill", "install", dir},
		func(s string) { out = append(out, s) }, func(s string) { errOut = append(errOut, s) })
	if code != 0 {
		t.Fatalf("code=%d err=%v", code, errOut)
	}
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil || !strings.HasPrefix(string(b), "---\nname: shijimi\n") {
		t.Fatalf("bad SKILL.md: %v %q", err, b)
	}
	if Run([]string{"skill"}, func(string) {}, func(string) {}) != 2 {
		t.Fatal("skill without install should be a usage error")
	}
}

func TestRunExec(t *testing.T) {
	dir, err := os.MkdirTemp("", "ghe")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s.sock")
	var got string
	srv, err := Listen(sock, func(r Request) Response {
		got = r.Cmd + "|" + r.Line
		if r.Line == ":bad" {
			return Response{Error: "no"}
		}
		return Response{OK: true, Message: "done"}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	t.Setenv(EnvSocket, sock)
	var out, errOut []string
	o := func(s string) { out = append(out, s) }
	e := func(s string) { errOut = append(errOut, s) }
	if Run([]string{"exec", ":grep", "foo"}, o, e) != 0 || got != "exec|:grep foo" || len(out) != 1 || out[0] != "done" {
		t.Fatalf("got=%q out=%v err=%v", got, out, errOut)
	}
	if Run([]string{"exec", ":bad"}, o, e) != 1 {
		t.Fatal("error reply should exit 1")
	}
	if Run([]string{"exec"}, o, e) != 2 {
		t.Fatal("exec without a command is a usage error")
	}
}

func TestSocketPathIsSeparatePerChannel(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	channel.SetForTest(t, "")
	stable, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	channel.SetForTest(t, channel.Nightly)
	nightly, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(stable) == filepath.Dir(nightly) {
		t.Fatalf("stable and nightly share the directory %s", filepath.Dir(stable))
	}
	if base := filepath.Base(filepath.Dir(nightly)); !strings.HasPrefix(base, "shijimi-nightly-") {
		t.Fatalf("nightly dir = %s", base)
	}
	if base := filepath.Base(filepath.Dir(stable)); !strings.HasPrefix(base, "shijimi-") || strings.Contains(base, "nightly") {
		t.Fatalf("stable dir = %s", base)
	}
}

// isolate points the socket directory at a fresh place and clears the
// variables that FindSocket reads, so that an editor running around the tests
// is not found.
func isolate(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	for _, k := range []string{EnvSocket, EnvHerdr, EnvHerdrWorkspace, EnvHerdrPane} {
		t.Setenv(k, "")
	}
	dir, err := socketDir(true)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func listen(t *testing.T, path string) *Server {
	t.Helper()
	srv, err := Listen(path, func(Request) Response { return Response{OK: true} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestFindSocketPrefersEnv(t *testing.T) {
	dir := isolate(t)
	listen(t, filepath.Join(dir, "1.sock"))
	t.Setenv(EnvSocket, "/nowhere/x.sock")
	if got, err := FindSocket(); err != nil || got != "/nowhere/x.sock" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestFindSocketUsesTheOnlyEditor(t *testing.T) {
	dir := isolate(t)
	if _, err := FindSocket(); err == nil {
		t.Fatal("nothing running: expected error")
	}
	// A socket left behind by a killed editor, and a stray file, are skipped.
	if err := os.WriteFile(filepath.Join(dir, "7.sock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FindSocket(); err == nil {
		t.Fatal("only a dead socket: expected error")
	}
	one := listen(t, filepath.Join(dir, "1.sock"))
	if got, err := FindSocket(); err != nil || got != one.Path {
		t.Fatalf("got %q, %v; want %q", got, err, one.Path)
	}
	two := listen(t, filepath.Join(dir, "2.sock"))
	if _, err := FindSocket(); err == nil || !strings.Contains(err.Error(), "2 ") {
		t.Fatalf("two running: got %v", err)
	}
	two.Close()
	if got, err := FindSocket(); err != nil || got != one.Path {
		t.Fatalf("after close: got %q, %v", got, err)
	}
}

func TestHerdrLink(t *testing.T) {
	dir := isolate(t)
	t.Setenv(EnvHerdr, "1")
	t.Setenv(EnvHerdrWorkspace, "w1")
	t.Setenv(EnvHerdrPane, "w1:p2")
	srv := listen(t, filepath.Join(dir, "1.sock"))
	link := filepath.Join(dir, "herdr-w1", "w1_p2.sock")
	if target, err := os.Readlink(link); err != nil || target != srv.Path {
		t.Fatalf("link = %q, %v; want %q", target, err, srv.Path)
	}
	// shj in the same pane, without SHIJIMI_SOCKET, reaches the editor.
	if got, err := FindSocket(); err != nil || got != link {
		t.Fatalf("same pane: got %q, %v", got, err)
	}
	// Another pane of the workspace without an editor still reaches the only one.
	t.Setenv(EnvHerdrPane, "w1:p9")
	if got, err := FindSocket(); err != nil || got != link {
		t.Fatalf("other pane: got %q, %v", got, err)
	}
	t.Setenv(EnvHerdrPane, "")
	if got, err := FindSocket(); err != nil || got != link {
		t.Fatalf("no pane: got %q, %v", got, err)
	}
	// Another workspace does not see it, even though it is the only editor.
	t.Setenv(EnvHerdrWorkspace, "w2")
	if _, err := FindSocket(); err == nil {
		t.Fatal("other workspace: expected error")
	}
	// A second editor in the workspace makes the choice ambiguous.
	t.Setenv(EnvHerdrWorkspace, "w1")
	t.Setenv(EnvHerdrPane, "w1:p3")
	second := listen(t, filepath.Join(dir, "2.sock"))
	if got, err := FindSocket(); err != nil || got != filepath.Join(dir, "herdr-w1", "w1_p3.sock") {
		t.Fatalf("own pane with two editors: got %q, %v", got, err)
	}
	t.Setenv(EnvHerdrPane, "")
	if _, err := FindSocket(); err == nil {
		t.Fatal("two editors in the workspace: expected error")
	}
	// Quitting removes the link; a dead link left by a kill is skipped.
	second.Close()
	if _, err := os.Lstat(filepath.Join(dir, "herdr-w1", "w1_p3.sock")); !os.IsNotExist(err) {
		t.Fatalf("link not removed: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone.sock"), filepath.Join(dir, "herdr-w1", "dead.sock")); err != nil {
		t.Fatal(err)
	}
	if got, err := FindSocket(); err != nil || got != link {
		t.Fatalf("with a dead link: got %q, %v", got, err)
	}
	srv.Close()
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("link not removed: %v", err)
	}
}

func TestHerdrLinkWithoutPane(t *testing.T) {
	dir := isolate(t)
	t.Setenv(EnvHerdr, "1")
	t.Setenv(EnvHerdrWorkspace, "w1")
	listen(t, filepath.Join(dir, "1.sock"))
	entries, err := os.ReadDir(filepath.Join(dir, "herdr-w1"))
	if err != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "pid-") {
		t.Fatalf("entries = %v, %v", entries, err)
	}
}

func TestNoHerdrLinkOutsideHerdr(t *testing.T) {
	dir := isolate(t)
	t.Setenv(EnvHerdrWorkspace, "w1")
	listen(t, filepath.Join(dir, "1.sock"))
	if _, err := os.Stat(filepath.Join(dir, "herdr-w1")); !os.IsNotExist(err) {
		t.Fatalf("link made outside herdr: %v", err)
	}
}
