package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"
)

func TestReviewToRoot(t *testing.T) {
	a := appOf(&core.Core{Root: "/repo/sub", GitTop: "/repo"})
	if got := a.ReviewToRoot("sub/pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("with gitTop: %q", got)
	}
	a.GitTop = ""
	if got := a.ReviewToRoot("pkg/f.go"); got != "pkg/f.go" {
		t.Errorf("without gitTop: %q", got)
	}
}
