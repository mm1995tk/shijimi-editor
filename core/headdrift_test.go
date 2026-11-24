package core

import (
	"os"
	"path/filepath"
	"testing"
)

// driftApp is prApp (branch feature, the pull request's head is HEAD) with the
// branch pushed to a remote of its own, so that it has an upstream. The diff
// mode is the pull request's base <- working tree.
func driftApp(t *testing.T) (*App, string) {
	t.Helper()
	a, file := prApp(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitIn(t, a.Root, "init", "-q", "--bare", remote)
	gitIn(t, a.Root, "remote", "add", "origin", remote)
	gitIn(t, a.Root, "push", "-q", "-u", "origin", "feature")
	loadSides(a)
	return a, file
}

// commitMore makes a commit in the repository of a.
func commitMore(t *testing.T, a *App, name string) {
	t.Helper()
	os.WriteFile(filepath.Join(a.Root, name), []byte(name+"\n"), 0o644)
	gitIn(t, a.Root, "add", name)
	gitIn(t, a.Root, "commit", "-q", "-m", name)
}

func driftNote(a *App) (left, right bool, note string) {
	l, r, m := a.DriftSides()
	return l, r, tt(m)
}

func TestHeadDriftFollowsTheRemote(t *testing.T) {
	a, _ := driftApp(t)
	a.Gd.Right = RightHead
	loadSides(a)
	if l, r, n := driftNote(a); l || r || n != "" || a.DiffBandRows() != 0 {
		t.Fatalf("in sync: %v %v %q", l, r, n)
	}

	// ahead (not pushed): the right side, HEAD, is gray and the band says why
	commitMore(t, a, "c1.txt")
	loadSides(a)
	if l, r, n := driftNote(a); l || !r || n != "gd.drift.ahead(origin/feature, 1)" || a.DiffBandRows() != 1 {
		t.Fatalf("ahead: %v %v %q", l, r, n)
	}
	body := a.BodyH()

	// pushing brings it back
	gitIn(t, a.Root, "push", "-q")
	loadSides(a)
	if _, r, n := driftNote(a); r || n != "" || a.BodyH() != body+1 {
		t.Fatalf("pushed: %v %q body=%d", r, n, a.BodyH())
	}

	// behind: the remote has a commit HEAD lacks
	gitIn(t, a.Root, "reset", "-q", "--hard", "HEAD~1")
	loadSides(a)
	if _, r, n := driftNote(a); !r || n != "gd.drift.behind(origin/feature, 1)" {
		t.Fatalf("behind: %v %q", r, n)
	}

	// diverged: a commit on each side
	commitMore(t, a, "c2.txt")
	loadSides(a)
	if _, r, n := driftNote(a); !r || n != "gd.drift.diverged(origin/feature, 1, 1)" {
		t.Fatalf("diverged: %v %q", r, n)
	}
}

// Without HEAD as a side, a drift changes nothing; with HEAD on the left it
// grays the left.
func TestHeadDriftOnlyWhereHeadIsASide(t *testing.T) {
	a, _ := driftApp(t)
	commitMore(t, a, "c1.txt")
	loadSides(a) // PR's base <- working tree
	if l, r, n := driftNote(a); l || r || n != "" || a.DiffBandRows() != 0 {
		t.Errorf("PR <- work: %v %v %q", l, r, n)
	}

	a.Gd.Right = RightHead
	loadSides(a)
	if l, r, _ := driftNote(a); l || !r {
		t.Errorf("PR <- HEAD: left=%v right=%v", l, r)
	}

	// HEAD <- working tree: the left side is HEAD
	a.Gd.Base, a.Gd.Right, a.Gd.Pr = BaseHead, RightWork, nil
	loadSides(a)
	if l, r, n := driftNote(a); !l || r || n != "gd.drift.ahead(origin/feature, 1)" {
		t.Errorf("HEAD <- work: %v %v %q", l, r, n)
	}
}

// A branch without an upstream has no remote to differ from.
func TestHeadDriftWithoutUpstream(t *testing.T) {
	a, _ := prApp(t)
	a.Gd.Right = RightHead
	loadSides(a)
	commitMore(t, a, "c1.txt")
	loadSides(a)
	if l, r, n := driftNote(a); l || r || n != "" {
		t.Errorf("no upstream: %v %v %q", l, r, n)
	}
}

// With the pull request's branch not checked out, HEAD is compared with the
// pull request's head commit.
func TestHeadDriftAgainstThePullRequestHead(t *testing.T) {
	a, _ := prApp(t)
	a.Gd.Pr.HeadRefName = "elsewhere"
	a.Gd.Right = RightHead
	commitMore(t, a, "c1.txt")
	loadSides(a)
	if _, r, n := driftNote(a); !r || n != "gd.drift.pr(PR #7, 1, 0)" {
		t.Errorf("%v %q", r, n)
	}
}
