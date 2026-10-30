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
	t.Setenv(EnvSocket, "")
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
	old := channel.Channel
	t.Cleanup(func() { channel.Channel = old })

	channel.Channel = ""
	stable, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	channel.Channel = channel.Nightly
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
