package ui

import "testing"

func TestFormatVersion(t *testing.T) {
	for _, c := range []struct{ v, commit, want string }{
		{"0.1", "", "v0.1"},
		{"v1.2.3", "", "v1.2.3"},
		{"", "", "dev"},
		{"0.1", "be64521", "v0.1 (be64521)"},
		{"0.1", "be64521e0123456789abcdef0123456789abcdef", "v0.1 (be64521)"},
		{"0.1", "be64521e0123456789abcdef0123456789abcdef-dirty", "v0.1 (be64521-dirty)"},
		{"", "be64521-dirty", "dev (be64521-dirty)"},
	} {
		if got := formatVersion(c.v, c.commit); got != c.want {
			t.Errorf("formatVersion(%q, %q) = %q, want %q", c.v, c.commit, got, c.want)
		}
	}
}
