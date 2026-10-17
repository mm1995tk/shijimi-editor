package ui

import "testing"

func TestVersionLabel(t *testing.T) {
	defer func(v string) { Version = v }(Version)
	for in, want := range map[string]string{"0.1": "v0.1", "v1.2.3": "v1.2.3"} {
		Version = in
		if got := versionLabel(); got != want {
			t.Errorf("Version=%q: got %q, want %q", in, got, want)
		}
	}
}
