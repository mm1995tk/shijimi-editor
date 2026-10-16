package ui

import (
	"sort"
	"testing"
)

func TestNodeLessDotfilesLast(t *testing.T) {
	ns := []*node{
		{name: ".gitignore"}, {name: ".github", isDir: true}, {name: "README.md"},
		{name: "src", isDir: true}, {name: "go.mod"}, {name: ".env"},
	}
	sort.SliceStable(ns, func(i, j int) bool { return nodeLess(ns[i], ns[j]) })
	want := []string{"src", "go.mod", "README.md", ".github", ".env", ".gitignore"}
	for i, n := range ns {
		if n.name != want[i] {
			t.Fatalf("pos %d: got %s want %s", i, n.name, want[i])
		}
	}
}
