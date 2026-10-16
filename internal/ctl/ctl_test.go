package ctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSubcommand(t *testing.T) {
	for _, s := range []string{"diff", "skill"} {
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
	dir := filepath.Join(t.TempDir(), "x", "gh-editor")
	var out, errOut []string
	code := Run([]string{"skill", "install", dir},
		func(s string) { out = append(out, s) }, func(s string) { errOut = append(errOut, s) })
	if code != 0 {
		t.Fatalf("code=%d err=%v", code, errOut)
	}
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil || !strings.HasPrefix(string(b), "---\nname: gh-editor\n") {
		t.Fatalf("bad SKILL.md: %v %q", err, b)
	}
	if Run([]string{"skill"}, func(string) {}, func(string) {}) != 2 {
		t.Fatal("skill without install should be a usage error")
	}
}
