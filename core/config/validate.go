package config

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// Problem is one mistake in a configuration file: where it is and why it is
// wrong. The reason is a msg.Msg: the UI turns it into text, in the language
// it has selected by then (which it takes from the settings that did read).
type Problem struct {
	File string // path of the file
	Line int    // line in File; 0 when unknown (Nix has no lines)
	Key  string // the setting, such as "editor.auto-save.after-delay.timeout"; empty for the file as a whole
	// code is an i18n key; args are its format arguments.
	code string
	args []any
	// Warning marks a problem that does not stop shijimi-editor: a key this
	// version does not read. Load reports it in Config.Warnings, not in Error.
	Warning bool
}

// typeKey is the i18n key of a type's name. It stays a key in Problem.args and
// becomes a Msg in Reason, like the reason itself, so that a message is in one
// language: the one the UI selects after Load has read the settings.
type typeKey string

// Reason is the problem without its place.
func (p Problem) Reason() msg.Msg {
	args := make([]any, len(p.args))
	for i, a := range p.args {
		if k, ok := a.(typeKey); ok {
			a = msg.T(string(k))
		}
		args[i] = a
	}
	return msg.T(p.code, args...)
}

// Msg is "file:line: key: reason" (the line and the key are left out when
// unknown).
func (p Problem) Msg() msg.Msg {
	parts := []any{p.File}
	if p.Line > 0 {
		parts = append(parts, fmt.Sprintf(":%d", p.Line))
	}
	parts = append(parts, ": ")
	if p.Key != "" {
		parts = append(parts, p.Key, ": ")
	}
	return msg.Cat(append(parts, p.Reason())...)
}

// Error is Msg for logs and tests: the key of the reason, not text in a
// language. A UI turns Msg into text.
func (p Problem) Error() string { return p.Msg().String() }

// Error is what Load returns when a configuration file has mistakes: all of
// them, one per line in Msg.
type Error struct {
	Problems []Problem
}

// Msg lists the problems, one per line.
func (e *Error) Msg() msg.Msg {
	var parts []any
	for i, p := range e.Problems {
		if i > 0 {
			parts = append(parts, "\n")
		}
		parts = append(parts, p.Msg())
	}
	return msg.Cat(parts...)
}

// Error is Msg for logs and tests. A UI turns Msg into text.
func (e *Error) Error() string { return e.Msg().String() }

// Summary is the first problem and how many more there are, on one line, for
// the status line.
func (e *Error) Summary() msg.Msg { return Summary(e.Problems) }

// Summary is the first of ps and how many more there are, on one line. ps must
// not be empty.
func Summary(ps []Problem) msg.Msg {
	m := ps[0].Msg()
	if n := len(ps) - 1; n > 0 {
		m = msg.Cat(m, msg.T("config.err.more", n))
	}
	return m
}

type kind int

const (
	kString    kind = iota
	kBool           // true / false
	kInt            // an integer
	kStrings        // an array of strings
	kStringMap      // a table whose values are strings
	kAny            // anything (a language server's own settings)
	kTable          // a table with the keys of sub
	kTableOf        // a table of tables, each with the keys of sub (language-server.<name>)
	kTables         // an array of tables with the keys of sub ([[language]])
	kItems          // an array whose items are one of alts
)

// field is what a setting must be.
type field struct {
	kind kind
	sub  map[string]field
	alts []field // kItems: each item must match one of these
	// required lists the keys every table of a kTables must have (non-empty).
	required []string
	// userOnly settings are read only from the user's own directory.
	userOnly bool
	// check returns the reason code (and arguments) when a value of the right
	// type is not acceptable; "" when it is.
	check func(v any) (string, []any)
}

var (
	fString  = field{kind: kString}
	fBool    = field{kind: kBool}
	fInt     = field{kind: kInt}
	fStrings = field{kind: kStrings}
)

func table(sub map[string]field) field { return field{kind: kTable, sub: sub} }

// configSchema is every setting of config.toml / config.nix.
var configSchema = map[string]field{
	"keymap": fString,
	"theme":  fString,
	"search": table(map[string]field{
		"exclude": fStrings,
		"include": fStrings,
	}),
	"editor": table(map[string]field{
		"mermaid-command": fString,
		"watch-files":     fBool,
		"keymap":          fString,
		"line-number":     fString,
		"file-icons":      fBool,
		"mouse-hover":     fBool,
		"theme":           fString,
		"indent-guides":   table(map[string]field{"render": fBool}),
		"auto-save": table(map[string]field{
			// on-switch was what `shj serve` did with the open files of a workspace that
			// switched folders. Nothing reads it now; it is still accepted so that a file
			// that has it keeps loading.
			"on-switch": {kind: kString, check: checkOnSwitch},
			"after-delay": table(map[string]field{
				"enable":  fBool,
				"timeout": {kind: kInt, check: checkPositive},
			}),
		}),
	}),
	"browser": table(map[string]field{"listen-host": fString}),
	"agent": {kind: kTable, userOnly: true, sub: map[string]field{
		"command":     fString,
		"args":        fStrings,
		"environment": {kind: kStringMap},
	}},
	"keys": {kind: kTable, userOnly: true, sub: map[string]field{
		"normal": {kind: kStringMap},
		"insert": {kind: kStringMap},
		"select": {kind: kStringMap},
	}},
}

// languagesSchema is every setting of languages.toml / languages.nix.
var languagesSchema = map[string]field{
	"language-server": {kind: kTableOf, sub: map[string]field{
		"command":     fString,
		"args":        fStrings,
		"environment": {kind: kStringMap},
		"config":      {kind: kAny},
	}},
	"language": {kind: kTables, required: []string{"name"}, sub: map[string]field{
		"name":        fString,
		"language-id": fString,
		"file-types": {kind: kItems, alts: []field{
			fString,
			table(map[string]field{"glob": fString}),
		}},
		"roots": fStrings,
		"language-servers": {kind: kItems, alts: []field{
			fString,
			table(map[string]field{
				"name":            fString,
				"only-features":   fStrings,
				"except-features": fStrings,
			}),
		}},
		"indent": table(map[string]field{
			"unit":      fString,
			"tab-width": fInt,
		}),
		"formatter": table(map[string]field{
			"command": fString,
			"args":    fStrings,
		}),
	}},
}

func checkOnSwitch(v any) (string, []any) {
	if s, _ := v.(string); s != "save" && s != "discard" {
		return "config.err.oneof", []any{"save, discard"}
	}
	return "", nil
}

func checkNonNegative(v any) (string, []any) {
	if n, _ := v.(int64); n < 0 {
		return "config.err.nonneg", nil
	}
	return "", nil
}

func checkPositive(v any) (string, []any) {
	if n, _ := v.(int64); n <= 0 {
		return "config.err.positive", nil
	}
	return "", nil
}

// validator collects the problems of one file.
type validator struct {
	file  string
	user  bool
	lines *lineIndex // nil for a file without lines (Nix)
	out   []Problem
}

// validate checks raw (the decoded file) against schema and returns what is
// wrong with it. text is the source when it is TOML (for the lines), "" for Nix.
func validate(raw map[string]any, schema map[string]field, file, text string, user bool) []Problem {
	v := &validator{file: file, user: user}
	if text != "" {
		v.lines = indexLines(text)
	}
	v.table(raw, schema, nil)
	// in the order of the file
	sort.SliceStable(v.out, func(i, j int) bool { return v.out[i].Line < v.out[j].Line })
	return v.out
}

func (v *validator) add(path []string, code string, args ...any) {
	p := Problem{File: v.file, Key: pathString(path), code: code, args: args, Warning: code == codeUnknown}
	if v.lines != nil {
		p.Line = v.lines.find(path)
	}
	v.out = append(v.out, p)
}

func pathString(path []string) string {
	var b strings.Builder
	for i, s := range path {
		if !strings.HasPrefix(s, "[") && i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s)
	}
	return b.String()
}

func extend(path []string, seg string) []string {
	return append(append(make([]string, 0, len(path)+1), path...), seg)
}

func (v *validator) table(m map[string]any, schema map[string]field, path []string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := extend(path, k)
		f, ok := schema[k]
		if !ok {
			v.add(p, codeUnknown)
			continue
		}
		if f.userOnly && !v.user {
			v.add(p, "config.err.useronly")
			continue
		}
		v.value(m[k], f, p)
	}
}

func (v *validator) value(x any, f field, path []string) {
	bad := func(want string) { v.add(path, "config.err.type", typeKey(want), describe(x)) }
	switch f.kind {
	case kString:
		if _, ok := x.(string); !ok {
			bad("config.type.string")
			return
		}
	case kBool:
		if _, ok := x.(bool); !ok {
			bad("config.type.bool")
			return
		}
	case kInt:
		if _, ok := x.(int64); !ok {
			bad("config.type.int")
			return
		}
	case kStrings:
		list, ok := asList(x)
		if !ok {
			bad("config.type.strings")
			return
		}
		for i, e := range list {
			if _, ok := e.(string); !ok {
				v.add(extend(path, fmt.Sprintf("[%d]", i)), "config.err.type", typeKey("config.type.string"), describe(e))
			}
		}
		return
	case kStringMap:
		m, ok := x.(map[string]any)
		if !ok {
			bad("config.type.stringmap")
			return
		}
		for _, k := range sortedKeys(m) {
			if _, ok := m[k].(string); !ok {
				v.add(extend(path, k), "config.err.type", typeKey("config.type.string"), describe(m[k]))
			}
		}
		return
	case kAny:
		return
	case kTable:
		m, ok := x.(map[string]any)
		if !ok {
			bad("config.type.table")
			return
		}
		v.table(m, f.sub, path)
		return
	case kTableOf:
		m, ok := x.(map[string]any)
		if !ok {
			bad("config.type.table")
			return
		}
		for _, k := range sortedKeys(m) {
			e, ok := m[k].(map[string]any)
			if !ok {
				v.add(extend(path, k), "config.err.type", typeKey("config.type.table"), describe(m[k]))
				continue
			}
			v.table(e, f.sub, extend(path, k))
		}
		return
	case kTables:
		list, ok := asList(x)
		if !ok {
			bad("config.type.tables")
			return
		}
		for i, e := range list {
			p := extend(path, fmt.Sprintf("[%d]", i))
			m, ok := e.(map[string]any)
			if !ok {
				v.add(p, "config.err.type", typeKey("config.type.table"), describe(e))
				continue
			}
			for _, k := range f.required {
				if val, has := m[k]; !has || val == "" {
					v.add(p, "config.err.required", k)
				}
			}
			v.table(m, f.sub, p)
		}
		return
	case kItems:
		list, ok := asList(x)
		if !ok {
			bad("config.type.array")
			return
		}
		for i, e := range list {
			p := extend(path, fmt.Sprintf("[%d]", i))
			switch e := e.(type) {
			case string:
				// a plain string is always one of the alternatives
			case map[string]any:
				for _, alt := range f.alts {
					if alt.kind == kTable {
						v.table(e, alt.sub, p)
					}
				}
			default:
				v.add(p, "config.err.type", typeKey("config.type.stringortable"), describe(e))
			}
		}
		return
	}
	if f.check != nil {
		if code, args := f.check(x); code != "" {
			v.add(path, code, args...)
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// asList reads an array the way the TOML decoder (array of tables:
// []map[string]any) and the JSON decoder ([]any) give it.
func asList(x any) ([]any, bool) {
	switch x := x.(type) {
	case []any:
		return x, true
	case []map[string]any:
		out := make([]any, len(x))
		for i, m := range x {
			out[i] = m
		}
		return out, true
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

// describe names the type of a decoded value for a message.
func describe(x any) typeKey {
	switch x.(type) {
	case string:
		return typeKey("config.type.string")
	case bool:
		return typeKey("config.type.bool")
	case int64:
		return typeKey("config.type.int")
	case float64:
		return typeKey("config.type.float")
	case map[string]any:
		return typeKey("config.type.table")
	case []any, []map[string]any, []string:
		return typeKey("config.type.array")
	case time.Time:
		return typeKey("config.type.datetime")
	case nil:
		return typeKey("config.type.null")
	}
	return typeKey(fmt.Sprintf("%T", x))
}

// lineIndex maps the keys of a TOML source to their lines. It reads the source
// line by line, which is enough to point at a key: the decoder keeps the
// positions to itself, and gives them only for syntax errors.
type lineIndex struct {
	lines map[string]int
}

const pathSep = "\x00"

func (ix *lineIndex) find(path []string) int {
	for n := len(path); n > 0; n-- {
		if l, ok := ix.lines[strings.Join(path[:n], pathSep)]; ok {
			return l
		}
	}
	return 0
}

func (ix *lineIndex) set(path []string, line int) {
	k := strings.Join(path, pathSep)
	if _, ok := ix.lines[k]; !ok {
		ix.lines[k] = line
	}
}

func indexLines(text string) *lineIndex {
	ix := &lineIndex{lines: map[string]int{}}
	// arrays counts the [[x]] headers seen, to number the items: language[1].
	arrays := map[string]int{}
	// resolve puts the index of the latest item after each array of tables in
	// the path, so `[language.indent]` after the second [[language]] is
	// language[1].indent.
	resolve := func(segs []string) []string {
		out := make([]string, 0, len(segs)+1)
		for i, s := range segs {
			out = append(out, s)
			if n, ok := arrays[strings.Join(segs[:i+1], pathSep)]; ok {
				out = append(out, fmt.Sprintf("[%d]", n-1))
			}
		}
		return out
	}
	var decl []string // the table the next keys belong to, as written
	for i, line := range strings.Split(text, "\n") {
		no := i + 1
		s := strings.TrimSpace(line)
		switch {
		case s == "" || s[0] == '#':
			continue
		case strings.HasPrefix(s, "[["):
			end := strings.Index(s, "]]")
			if end < 0 {
				continue
			}
			segs, _ := parseKey(s[2:end])
			arrays[strings.Join(segs, pathSep)]++
			decl = segs
			ix.set(resolve(decl), no)
		case s[0] == '[':
			end := strings.IndexByte(s, ']')
			if end < 0 {
				continue
			}
			segs, _ := parseKey(s[1:end])
			decl = segs
			ix.set(resolve(decl), no)
		default:
			segs, ok := parseKeyBeforeEquals(s)
			if !ok {
				continue
			}
			full := append(append([]string{}, decl...), segs...)
			ix.set(resolve(full), no)
		}
	}
	return ix
}

// parseKeyBeforeEquals reads the (dotted) key at the start of a line that has
// `key = value`.
func parseKeyBeforeEquals(s string) ([]string, bool) {
	segs, rest := parseKey(s)
	if len(segs) == 0 {
		return nil, false
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "=") {
		return nil, false
	}
	return segs, true
}

// parseKey reads a dotted key (bare or quoted parts) from the start of s and
// returns its parts and what follows it.
func parseKey(s string) ([]string, string) {
	var segs []string
	for {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			return segs, ""
		}
		var part string
		switch s[0] {
		case '"':
			end := 1
			for end < len(s) && s[end] != '"' {
				if s[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(s) {
				return nil, ""
			}
			part, s = s[1:end], s[end+1:]
		case '\'':
			end := strings.IndexByte(s[1:], '\'')
			if end < 0 {
				return nil, ""
			}
			part, s = s[1:1+end], s[end+2:]
		default:
			end := 0
			for end < len(s) && isBare(s[end]) {
				end++
			}
			if end == 0 {
				return nil, ""
			}
			part, s = s[:end], s[end:]
		}
		segs = append(segs, part)
		t := strings.TrimLeft(s, " \t")
		if !strings.HasPrefix(t, ".") {
			return segs, s
		}
		s = t[1:]
	}
}

func isBare(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// codeUnknown is the reason of a key this version does not read. It is the one
// problem that is a warning (Problem.Warning).
const codeUnknown = "config.warn.unknown"
