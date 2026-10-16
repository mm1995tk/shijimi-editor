package ui

import "testing"

func TestIconFor(t *testing.T) {
	cases := []struct {
		n    node
		want fileIcon
	}{
		{node{name: "main.go"}, icoGo},
		{node{name: "flake.nix"}, icoNix},
		{node{name: "README.md"}, icoMarkdown},
		{node{name: "a.PNG"}, icoImage},
		{node{name: "go.sum"}, icoGo},
		{node{name: "unknown.xyz"}, iconDefault},
		{node{name: "src", isDir: true}, iconFolder},
		{node{name: "src", isDir: true, expanded: true}, iconFolderOpen},
	}
	for _, c := range cases {
		if got := iconFor(&c.n); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.n.name, got, c.want)
		}
	}
}
