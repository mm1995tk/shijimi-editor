package core

import (
	"sort"
	"testing"
)

func TestNodeLessDirsFirstDotFirst(t *testing.T) {
	ns := []*Node{
		{Name: ".gitignore"}, {Name: ".github", isDir: true}, {Name: "README.md"},
		{Name: "src", isDir: true}, {Name: "go.mod"}, {Name: ".env"},
	}
	sort.SliceStable(ns, func(i, j int) bool { return nodeLess(ns[i], ns[j]) })
	want := []string{".github", "src", ".env", ".gitignore", "go.mod", "README.md"}
	for i, n := range ns {
		if n.Name != want[i] {
			t.Fatalf("pos %d: got %s want %s", i, n.Name, want[i])
		}
	}
}
