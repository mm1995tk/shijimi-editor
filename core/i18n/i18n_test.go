package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/mm1995tk/shijimi-editor/core/msg"
)

func TestDefaultIsEnglish(t *testing.T) {
	SetLang("")
	if Lang() != English {
		t.Fatalf("default language = %q, want en", Lang())
	}
	SetLang("fr")
	if Lang() != English {
		t.Fatalf("unknown language = %q, want en", Lang())
	}
	SetLang(Japanese)
	t.Cleanup(func() { SetLang(Default) })
	if Lang() != Japanese || T("help.usage") != ja["help.usage"] {
		t.Fatalf("SetLang(ja) did not switch the table")
	}
}

func TestMissingKeyFallsBack(t *testing.T) {
	SetLang(Japanese)
	t.Cleanup(func() { SetLang(Default) })
	en["test.only.en"] = "english only"
	t.Cleanup(func() { delete(en, "test.only.en") })
	if got := T("test.only.en"); got != "english only" {
		t.Fatalf("fallback to English = %q", got)
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Fatalf("unknown key = %q, want the key itself", got)
	}
}

func TestTablesHaveTheSameKeys(t *testing.T) {
	for k := range en {
		if _, ok := ja[k]; !ok {
			t.Errorf("key %q is missing from the Japanese table", k)
		}
	}
	for k := range ja {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q is missing from the English table", k)
		}
	}
}

var verbRE = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)

// Both languages are passed the same arguments, so they must take the same
// verbs in the same order.
func TestFormatVerbsMatch(t *testing.T) {
	for k, e := range en {
		j, ok := ja[k]
		if !ok {
			continue
		}
		ev, jv := verbRE.FindAllString(e, -1), verbRE.FindAllString(j, -1)
		if strings.Join(ev, " ") != strings.Join(jv, " ") {
			t.Errorf("%s: verbs differ: en %v, ja %v", k, ev, jv)
		}
	}
}

func TestEnglishHasNoJapanese(t *testing.T) {
	for k, e := range en {
		for _, r := range e {
			if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
				t.Errorf("%s: the English text contains %q", k, r)
				break
			}
		}
	}
}

// goFiles lists the non-test Go files of the module.
func goFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "manual") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// Every key handed to T exists in the tables, and code outside the tables
// holds no Japanese string literal.
func TestSourceUsesTablesOnly(t *testing.T) {
	used := map[string]bool{}
	fset := token.NewFileSet()
	for _, p := range goFiles(t) {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, p, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		slash := filepath.ToSlash(p)
		isTable := strings.HasSuffix(slash, "core/i18n/ja.go") || strings.HasSuffix(slash, "core/i18n/en.go")
		// The fake agent used by tests answers in Japanese on purpose.
		isFake := strings.Contains(slash, "core/acptest/")
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || len(n.Args) == 0 {
					return true
				}
				if !isKeyCall(sel, len(n.Args), slash) {
					return true
				}
				lit, ok := n.Args[0].(*ast.BasicLit)
				if !ok {
					// A key taken from a table (menu items, symbol kinds);
					// the unused-key check below covers those.
					return true
				}
				key, _ := strconv.Unquote(lit.Value)
				if sel.Sel.Name != "T" || id0(sel) != "i18n" {
					for _, tbl := range []map[string]string{en, ja} {
						if strings.Contains(tbl[key], "%w") {
							t.Errorf("%s: key %q has %%w, which Sprintf does not know; use %%s", fset.Position(n.Pos()), key)
						}
					}
				}
				if _, ok := en[key]; !ok {
					t.Errorf("%s: unknown i18n key %q", fset.Position(n.Pos()), key)
				}
			case *ast.BasicLit:
				if n.Kind != token.STRING || isTable || isFake {
					return true
				}
				if s, err := strconv.Unquote(n.Value); err == nil {
					used[s] = true
				}
				for _, r := range n.Value {
					if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
						t.Errorf("%s: Japanese string literal in code: %s", fset.Position(n.Pos()), n.Value)
						break
					}
				}
			}
			return true
		})
	}
	for k := range en {
		if !used[k] {
			t.Errorf("key %q is not used by any code", k)
		}
	}
}

// isKeyCall reports whether a call of sel with n arguments hands its first
// argument to the tables as a key: i18n.T(key), msg.T(key, ...), msg.Err(key, ...)
// and, in the core and the TUI, Core.Info(key, ...).
func isKeyCall(sel *ast.SelectorExpr, n int, slash string) bool {
	id, _ := sel.X.(*ast.Ident)
	switch sel.Sel.Name {
	case "T":
		return id != nil && (id.Name == "i18n" && n == 1 || id.Name == "msg")
	case "Err":
		return id != nil && id.Name == "msg"
	case "Info":
		return strings.Contains(slash, "core/") || strings.Contains(slash, "ui/tui/")
	}
	return false
}

func TestMsgTextFillsArgs(t *testing.T) {
	SetLang(English)
	en["test.args"] = "%s has %d"
	t.Cleanup(func() { delete(en, "test.args") })
	m := msg.T("test.args", msg.Cat("a", "b"), 3)
	if got := Text(m); got != "ab has 3" {
		t.Errorf("Text = %q", got)
	}
	if got := ErrText(msg.Err("test.args", msg.S("x"), 1)); got != "x has 1" {
		t.Errorf("ErrText = %q", got)
	}
	if got := Text(msg.T("test.args", msg.Err("test.args", "y", 2), 3)); got != "y has 2 has 3" {
		t.Errorf("nested error = %q", got)
	}
	if got := Text(msg.S("plain").Add(" ", msg.T("test.args", "z", 4))); got != "plain z has 4" {
		t.Errorf("Add = %q", got)
	}
}

func id0(sel *ast.SelectorExpr) string {
	id, _ := sel.X.(*ast.Ident)
	if id == nil {
		return ""
	}
	return id.Name
}

// Table is the selected language over English, so the GUI is never blank where
// a language lacks a string.
func TestTableFallsBackToEnglish(t *testing.T) {
	t.Cleanup(func() { SetLang(Default) })
	SetLang(Japanese)
	tb := Table()
	if len(tb) < len(en) || tb["acp.notfound"] != ja["acp.notfound"] {
		t.Errorf("table: %d keys, %q", len(tb), tb["acp.notfound"])
	}
	delete(ja, "acp.notfound")
	defer func() { ja["acp.notfound"] = tb["acp.notfound"] }()
	if got := Table()["acp.notfound"]; got != en["acp.notfound"] {
		t.Errorf("fallback = %q", got)
	}
}
