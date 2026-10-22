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
		isTable := strings.HasSuffix(slash, "internal/i18n/ja.go")
		// The fake agent used by tests answers in Japanese on purpose.
		isFake := strings.Contains(slash, "internal/acp/acptest/")
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "T" || len(n.Args) != 1 {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "i18n" {
					return true
				}
				lit, ok := n.Args[0].(*ast.BasicLit)
				if !ok {
					// A key taken from a table (menu items, symbol kinds);
					// the unused-key check below covers those.
					return true
				}
				key, _ := strconv.Unquote(lit.Value)
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
