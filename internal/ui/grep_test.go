package ui

import (
	"reflect"
	"testing"

	"github.com/mm1995tk/gh-editor/internal/config"
)

func TestParseGrepArgs(t *testing.T) {
	def := config.Search{Exclude: []string{"vendor/**"}}
	q, inc, exc := parseGrepArgs("-x *.lock,dist/** --include=*.go foo bar", def)
	if q != "foo bar" {
		t.Fatalf("query = %q", q)
	}
	if !reflect.DeepEqual(inc, []string{"*.go"}) {
		t.Fatalf("include = %v", inc)
	}
	if !reflect.DeepEqual(exc, []string{"vendor/**", "*.lock", "dist/**"}) {
		t.Fatalf("exclude = %v", exc)
	}
	if q, _, _ := parseGrepArgs("-foo", config.Search{}); q != "-foo" {
		t.Fatalf("unknown flag query = %q", q)
	}
}

func TestGitGlob(t *testing.T) {
	for in, want := range map[string]string{"*.lock": "**/*.lock", "dist/**": "dist/**", "/a/b": "a/b"} {
		if got := gitGlob(in); got != want {
			t.Errorf("gitGlob(%q) = %q, want %q", in, got, want)
		}
	}
}
