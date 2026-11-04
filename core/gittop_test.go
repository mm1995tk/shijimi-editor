package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A project opened through a symbolic link keeps the link in the paths of the
// tree and the buffers, so the top of the repository must be spelled the same
// way (git itself prints it with links resolved, as macOS does for /var).
func TestGitTopKeepsTheLinkOfTheOpenedPath(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	realTop, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	// A link into the middle of the repository has no spelling of the top.
	mid := filepath.Join(base, "mid")
	if err := os.Symlink(filepath.Join(repo, "sub"), mid); err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string]string{
		link:                               link,
		filepath.Join(link, "sub"):         link,
		filepath.Join(link, "sub", "deep"): link,
		filepath.Join(mid, "deep"):         realTop,
		filepath.Join(repo, "sub"):         repo,
	} {
		got, err := gitTop(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if got != want {
			t.Errorf("gitTop(%s) = %s, want %s", dir, got, want)
		}
	}
}

func TestGitTopOutsideARepository(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(t.TempDir()))
	if _, err := gitTop(t.TempDir()); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}
