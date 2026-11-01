package core

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

const (
	MaxSearchHits     = 5000
	maxSearchFileSize = 2 << 20
)

// SearchOpts are the conditions of one project-wide search.
type SearchOpts struct {
	Query, Repl      string
	Include, Exclude []string
	CaseSens, Word   bool
	Regex, KeepCase  bool
}

// SearchHit is one matching line. All occurrences on the line are replaced
// together; skip excludes the line from the replacement.
type SearchHit struct {
	Path  string
	Row   int
	Line  string
	Spans [][2]int // rune offsets of each occurrence in line
	Repls []string // replacement text of each occurrence
	New   string   // line after the replacement
	Skip  bool
}

type SearchResult struct {
	Hits      []*SearchHit
	Files     int
	Truncated bool
	Err       error
}

func (o SearchOpts) compile() (*regexp.Regexp, error) {
	pat := o.Query
	if !o.Regex {
		pat = regexp.QuoteMeta(pat)
	}
	if !o.CaseSens {
		pat = "(?i)" + pat
	}
	return regexp.Compile(pat)
}

// preserveCase gives repl the letter case of matched: all upper, all lower or
// capitalized. Anything else leaves repl as it is.
func preserveCase(matched, repl string) string {
	var letters []rune
	for _, r := range matched {
		if unicode.IsLetter(r) {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return repl
	}
	allUpper, allLower, restLower := true, true, true
	for i, r := range letters {
		if !unicode.IsUpper(r) {
			allUpper = false
		}
		if !unicode.IsLower(r) {
			allLower = false
			if i > 0 {
				restLower = false
			}
		}
	}
	switch {
	case allUpper && len(letters) > 1:
		return strings.ToUpper(repl)
	case allLower:
		return strings.ToLower(repl)
	case unicode.IsUpper(letters[0]) && restLower:
		r, n := utf8.DecodeRuneInString(repl)
		if n == 0 {
			return repl
		}
		return string(unicode.ToUpper(r)) + repl[n:]
	}
	return repl
}

// isWordRune reports whether r is part of a word (Unicode-aware, unlike \b).
func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// wordBounded reports whether line[start:end] is not glued to a word character
// on either side.
func wordBounded(line string, start, end int) bool {
	if r, n := utf8.DecodeLastRuneInString(line[:start]); n > 0 && isWordRune(r) {
		return false
	}
	if r, n := utf8.DecodeRuneInString(line[end:]); n > 0 && isWordRune(r) {
		return false
	}
	return true
}

// planHit finds the occurrences in one line and computes their replacements;
// ok is false when the line does not match.
func planHit(re *regexp.Regexp, o SearchOpts, line string) (h *SearchHit, ok bool) {
	h = &SearchHit{Line: line}
	var sb strings.Builder
	last := 0
	for _, loc := range re.FindAllStringSubmatchIndex(line, -1) {
		if loc[0] == loc[1] {
			continue
		}
		if o.Word && !wordBounded(line, loc[0], loc[1]) {
			continue
		}
		repl := o.Repl
		if o.Regex {
			repl = string(re.ExpandString(nil, o.Repl, line, loc))
		}
		if o.KeepCase {
			repl = preserveCase(line[loc[0]:loc[1]], repl)
		}
		sb.WriteString(line[last:loc[0]])
		sb.WriteString(repl)
		last = loc[1]
		h.Spans = append(h.Spans, [2]int{utf8.RuneCountInString(line[:loc[0]]), utf8.RuneCountInString(line[:loc[1]])})
		h.Repls = append(h.Repls, repl)
	}
	if len(h.Spans) == 0 {
		return nil, false
	}
	sb.WriteString(line[last:])
	h.New = sb.String()
	return h, true
}

// projectFiles lists the files under root as slash-separated relative paths:
// tracked and untracked-but-not-ignored files in a git work tree, otherwise a
// walk that skips the usual build directories.
func projectFiles(root string) []string {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		var files []string
		for f := range strings.SplitSeq(string(out), "\x00") {
			if f != "" {
				files = append(files, f)
			}
		}
		return files
	}
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && SkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	return files
}

// readTextLines reads a text file as lines; ok is false for binary, huge or
// unreadable files.
func readTextLines(p string) (lines []string, ok bool) {
	st, err := os.Stat(p)
	if err != nil || st.Size() > maxSearchFileSize {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil || bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 || !utf8.Valid(b) {
		return nil, false
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return strings.Split(s, "\n"), true
}

// RunSearch scans the project. open maps absolute paths of open buffers to
// their (possibly unsaved) lines, which take precedence over the disk.
// cancelled is polled between files so a superseded search stops early.
func RunSearch(root string, o SearchOpts, open map[string][]string, cancelled func() bool) SearchResult {
	var res SearchResult
	if o.Query == "" {
		return res
	}
	re, err := o.compile()
	if err != nil {
		res.Err = err
		return res
	}
	paths := projectFiles(root)
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		seen[filepath.Join(root, filepath.FromSlash(p))] = true
	}
	// unsaved buffers may be on disk only partially, or not at all
	for p := range open {
		if !seen[p] && inRoot(root, p) {
			paths = append(paths, filepath.ToSlash(RelTo(root, p)))
		}
	}
	inc, exc := globMatcher(o.Include), globMatcher(o.Exclude)
	for _, rel := range paths {
		if cancelled() {
			return res
		}
		if (len(o.Include) > 0 && !inc(rel)) || exc(rel) {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		lines, ok := open[p]
		if !ok {
			if lines, ok = readTextLines(p); !ok {
				continue
			}
		}
		n := 0
		for row, l := range lines {
			h, ok := planHit(re, o, l)
			if !ok {
				continue
			}
			if len(res.Hits) >= MaxSearchHits {
				res.Truncated = true
				return res
			}
			h.Path, h.Row = p, row
			res.Hits = append(res.Hits, h)
			n++
		}
		if n > 0 {
			res.Files++
		}
	}
	return res
}

func inRoot(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && !strings.HasPrefix(r, "..")
}

// HitEdits builds the buffer edits for hits (all in one file) against the
// current buffer lines. Lines whose content no longer matches what was found
// are counted in stale and left untouched.
func HitEdits(hits []*SearchHit, cur [][]rune) (edits []buffer.Edit, replaced, stale int) {
	for _, h := range hits {
		if h.Skip {
			continue
		}
		if h.Row >= len(cur) || string(cur[h.Row]) != h.Line {
			stale++
			continue
		}
		replaced++
		for i, sp := range h.Spans {
			edits = append(edits, buffer.Edit{Start: buffer.Pos{Row: h.Row, Col: sp[0]}, End: buffer.Pos{Row: h.Row, Col: sp[1]}, Text: h.Repls[i]})
		}
	}
	return
}

// GitGlobs converts a glob to git pathspec globs: a bare pattern such as
// "*.lock" matches at any depth, and a name that matches a directory also
// matches everything under it.
func GitGlobs(g string) []string {
	g = strings.TrimSuffix(g, "/")
	if !strings.Contains(g, "/") {
		g = "**/" + g
	} else {
		g = strings.TrimPrefix(g, "/")
	}
	if strings.HasSuffix(g, "/**") {
		return []string{g}
	}
	return []string{g, g + "/**"}
}

// globMatcher matches slash-separated relative paths against globs with the
// same rules as gitGlobs (bare names match at any depth, directories match
// their contents, ** crosses directories).
func globMatcher(globs []string) func(rel string) bool {
	var res []*regexp.Regexp
	for _, g := range globs {
		for _, p := range GitGlobs(g) {
			var sb strings.Builder
			sb.WriteString("^")
			for i := 0; i < len(p); i++ {
				switch {
				case strings.HasPrefix(p[i:], "**/"):
					sb.WriteString("(?:.*/)?")
					i += 2
				case strings.HasPrefix(p[i:], "**"):
					sb.WriteString(".*")
					i++
				case p[i] == '*':
					sb.WriteString("[^/]*")
				case p[i] == '?':
					sb.WriteString("[^/]")
				default:
					sb.WriteString(regexp.QuoteMeta(string(p[i])))
				}
			}
			sb.WriteString("$")
			res = append(res, regexp.MustCompile(sb.String()))
		}
	}
	return func(rel string) bool {
		for _, re := range res {
			if re.MatchString(rel) {
				return true
			}
		}
		return false
	}
}

// replaceSep separates the search text from the replacement in
// `:replace [-x globs] [-i globs] query => replacement`.
const replaceSep = " => "

// SplitReplaceArgs separates `query => replacement` (the first " => " wins).
func SplitReplaceArgs(arg string) (query, repl string, ok bool) {
	q, r, ok := strings.Cut(arg, replaceSep)
	if !ok {
		// allow an empty replacement: `query =>`
		if t, ok := strings.CutSuffix(strings.TrimRight(arg, " "), " =>"); ok {
			return t, "", true
		}
	}
	return q, r, ok
}

// SkipDirs are the directories that file walks (project search, the file
// palette) do not descend into.
var SkipDirs = map[string]bool{".git": true, "node_modules": true, ".direnv": true, "target": true, "dist": true, ".next": true}

// RelTo is p relative to root, or p itself when it is outside root.
func RelTo(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}
