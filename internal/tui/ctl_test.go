package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
)

func TestReviewToRoot(t *testing.T) {
	a := appOf(&Core{root: "/repo/sub", gitTop: "/repo"})
	if got := a.reviewToRoot("sub/pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("with gitTop: %q", got)
	}
	a.gitTop = ""
	if got := a.reviewToRoot("pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("without gitTop: %q", got)
	}
}
