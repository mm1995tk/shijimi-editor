package ctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSubcommand(t *testing.T) {
	for _, s := range []string{"open", "diff", "status", "skill"} {
		if !IsSubcommand(s) {
			t.Errorf("%s should be a subcommand", s)
		}
	}
	for _, s := range []string{"./status", "main.go", ""} {
		if IsSubcommand(s) {
			t.Errorf("%q should not be a subcommand", s)
		}
	}
}

func TestParseTarget(t *testing.T) {
	dir := t.TempDir()
	colon := filepath.Join(dir, "a:3")
	os.WriteFile(colon, nil, 0o644)
	cases := []struct {
		in   string
		path string
		line int
	}{
		{"foo.go:12", "foo.go", 12},
		{"foo.go", "foo.go", 0},
		{"foo.go:x", "foo.go:x", 0},
		{"foo.go:0", "foo.go:0", 0},
		{colon, colon, 0},
	}
	for _, c := range cases {
		p, l := ParseTarget(c.in)
		if p != c.path || l != c.line {
			t.Errorf("ParseTarget(%q) = %q, %d; want %q, %d", c.in, p, l, c.path, c.line)
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
		if r.Cmd == "status" {
			return Response{OK: true, Status: &Status{Root: "/r", Tabs: []string{}}}
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
	resp, err := Call(Request{Cmd: "status"})
	if err != nil || !resp.OK || resp.Status.Root != "/r" {
		t.Fatalf("status: %+v %v", resp, err)
	}
	resp, _ = Call(Request{Cmd: "x"})
	if resp.OK || resp.Error != "bad x" {
		t.Fatalf("error: %+v", resp)
	}
}

func TestCallWithoutSocket(t *testing.T) {
	t.Setenv(EnvSocket, "")
	if _, err := Call(Request{Cmd: "status"}); err == nil {
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
