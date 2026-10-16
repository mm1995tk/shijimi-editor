// Package diff parses unified diffs (git diff / gh pr diff output).
package diff

import (
	"strconv"
	"strings"
)

// Kind is the type of a diff line.
type Kind byte

const (
	Context Kind = ' '
	Add     Kind = '+'
	Del     Kind = '-'
)

// Line is one line of a hunk. OldNo/NewNo are 0 when not applicable.
type Line struct {
	Kind         Kind
	Text         string
	OldNo, NewNo int
}

// Hunk is a contiguous group of changes.
type Hunk struct {
	Header string
	Lines  []Line
}

// File is the diff of one file.
type File struct {
	OldPath, NewPath string
	Status           string // "modified", "added", "deleted", "renamed", "binary"
	Hunks            []Hunk
	Added, Deleted   int
}

// Path is the path shown for the file (the new path unless deleted).
func (f *File) Path() string {
	if f.NewPath == "" || f.NewPath == "/dev/null" {
		return f.OldPath
	}
	return f.NewPath
}

// Parse parses unified diff text.
func Parse(text string) []*File {
	var files []*File
	var f *File
	var h *Hunk
	oldNo, newNo := 0, 0
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			f = &File{Status: "modified"}
			files = append(files, f)
			h = nil
			if a, b, ok := splitGitPaths(strings.TrimPrefix(l, "diff --git ")); ok {
				f.OldPath, f.NewPath = a, b
			}
		case f == nil:
			continue
		case h == nil && strings.HasPrefix(l, "--- "):
			if p := trimPath(l[4:]); p != "" {
				f.OldPath = p
			} else {
				f.Status = "added"
			}
		case h == nil && strings.HasPrefix(l, "+++ "):
			if p := trimPath(l[4:]); p != "" {
				f.NewPath = p
			} else {
				f.Status = "deleted"
			}
		case strings.HasPrefix(l, "new file mode"):
			f.Status = "added"
		case strings.HasPrefix(l, "deleted file mode"):
			f.Status = "deleted"
		case strings.HasPrefix(l, "rename from "):
			f.Status = "renamed"
			f.OldPath = strings.TrimPrefix(l, "rename from ")
		case strings.HasPrefix(l, "rename to "):
			f.NewPath = strings.TrimPrefix(l, "rename to ")
		case strings.HasPrefix(l, "Binary files"):
			f.Status = "binary"
		case strings.HasPrefix(l, "@@"):
			f.Hunks = append(f.Hunks, Hunk{Header: l})
			h = &f.Hunks[len(f.Hunks)-1]
			oldNo, newNo = parseHunkHeader(l)
		case h != nil && len(l) > 0:
			switch l[0] {
			case '+':
				h.Lines = append(h.Lines, Line{Kind: Add, Text: l[1:], NewNo: newNo})
				newNo++
				f.Added++
			case '-':
				h.Lines = append(h.Lines, Line{Kind: Del, Text: l[1:], OldNo: oldNo})
				oldNo++
				f.Deleted++
			case ' ':
				h.Lines = append(h.Lines, Line{Kind: Context, Text: l[1:], OldNo: oldNo, NewNo: newNo})
				oldNo++
				newNo++
			}
		}
	}
	return files
}

func trimPath(p string) string {
	p = strings.TrimSpace(p)
	if i := strings.IndexByte(p, '\t'); i >= 0 {
		p = p[:i]
	}
	if p == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		return p[2:]
	}
	return p
}

// splitGitPaths splits "a/x b/x" (paths without spaces in the common case).
func splitGitPaths(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "a/") {
		return "", "", false
	}
	i := strings.Index(s, " b/")
	if i < 0 {
		return "", "", false
	}
	return s[2:i], s[i+3:], true
}

func parseHunkHeader(h string) (int, int) {
	// @@ -a,b +c,d @@
	parts := strings.Fields(h)
	if len(parts) < 3 {
		return 1, 1
	}
	num := func(s string) int {
		s = strings.TrimLeft(s, "-+")
		if i := strings.IndexByte(s, ','); i >= 0 {
			s = s[:i]
		}
		n, _ := strconv.Atoi(s)
		return n
	}
	return num(parts[1]), num(parts[2])
}
