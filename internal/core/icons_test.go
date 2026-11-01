package core

import "testing"

func TestIconFor(t *testing.T) {
	cases := []struct {
		n    Node
		want fileIcon
	}{
		{Node{Name: "main.go"}, IcoGo},
		{Node{Name: "flake.nix"}, icoNix},
		{Node{Name: "README.md"}, icoMarkdown},
		{Node{Name: "a.PNG"}, icoImage},
		{Node{Name: "go.sum"}, IcoGo},
		{Node{Name: "unknown.xyz"}, iconDefault},
		{Node{Name: "src", isDir: true}, iconFolder},
		{Node{Name: "src", isDir: true, expanded: true}, iconFolderOpen},
	}
	for _, c := range cases {
		if got := IconFor(&c.n); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.n.Name, got, c.want)
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
	for _, th := range Themes {
		for k := range th.Icons {
			if !kinds[k] {
				t.Errorf("%s: unknown icon kind %q", th.Name, k)
			}
		}
	}
}

func TestIconForFollowsTheme(t *testing.T) {
	prev := CurTheme
	t.Cleanup(func() { ActivateTheme(prev) })

	n := Node{Name: "main.go"}
	ActivateTheme(&DarkPlus)
	if got := IconFor(&n); got != IcoGo {
		t.Fatalf("dark-plus: got %+v want %+v", got, IcoGo)
	}

	custom := DarkPlus
	custom.Icons = map[string]IconOverride{"go": {Color: "#123456"}}
	ActivateTheme(&custom)
	got := IconFor(&n)
	if got.Color != "#123456" || got.Glyph != IcoGo.Glyph {
		t.Errorf("color override: got %+v", got)
	}

	custom.Icons = map[string]IconOverride{"go": {Glyph: "G"}}
	ActivateTheme(&custom)
	got = IconFor(&n)
	if got.Glyph != "G" || got.Color != IcoGo.Color {
		t.Errorf("glyph override: got %+v", got)
	}
}
