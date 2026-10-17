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

func TestIconForFollowsTheme(t *testing.T) {
	prev := curTheme
	t.Cleanup(func() { applyTheme(prev) })

	n := node{name: "main.go"}
	applyTheme(&darkPlus)
	if got := iconFor(&n); got != icoGo {
		t.Fatalf("dark-plus: got %+v want %+v", got, icoGo)
	}

	custom := darkPlus
	custom.Icons = map[string]iconOverride{"go": {Color: "#123456"}}
	applyTheme(&custom)
	got := iconFor(&n)
	if got.color != "#123456" || got.glyph != icoGo.glyph {
		t.Errorf("color override: got %+v", got)
	}

	custom.Icons = map[string]iconOverride{"go": {Glyph: "G"}}
	applyTheme(&custom)
	got = iconFor(&n)
	if got.glyph != "G" || got.color != icoGo.color {
		t.Errorf("glyph override: got %+v", got)
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
