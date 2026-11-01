package core

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

func TestThemeIconKindsExist(t *testing.T) {
	kinds := map[string]bool{iconFolder.kind: true, iconFolderOpen.kind: true, iconDefault.kind: true}
	for _, ic := range iconByName {
		kinds[ic.kind] = true
	}
	for _, ic := range iconByExt {
		kinds[ic.kind] = true
	}
	for _, th := range themes {
		for k := range th.Icons {
			if !kinds[k] {
				t.Errorf("%s: unknown icon kind %q", th.Name, k)
			}
		}
	}
}
