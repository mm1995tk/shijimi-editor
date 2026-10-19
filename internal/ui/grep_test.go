package ui

import (
	"reflect"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/config"
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

func TestParseGrepArgsQuotes(t *testing.T) {
	q, inc, exc := parseGrepArgs(`-x '*.lock,"dist/**"' -i "*.go" foo`, config.Search{})
	if q != "foo" || !reflect.DeepEqual(inc, []string{"*.go"}) || !reflect.DeepEqual(exc, []string{"*.lock", "dist/**"}) {
		t.Fatalf("q=%q inc=%v exc=%v", q, inc, exc)
	}
}

func TestGitGlobs(t *testing.T) {
	for in, want := range map[string][]string{
		"*.lock":   {"**/*.lock", "**/*.lock/**"},
		"dist/**":  {"dist/**"},
		"/a/b":     {"a/b", "a/b/**"},
		"internal": {"**/internal", "**/internal/**"},
	} {
		if got := gitGlobs(in); !reflect.DeepEqual(got, want) {
			t.Errorf("gitGlobs(%q) = %v, want %v", in, got, want)
		}
	}
}
