package core

import (
	"os/exec"
	"strings"
	"testing"
)

// bannedDeps are the import path prefixes core must not depend on
// (docs/design/gui-protocol.md): the terminal UI libraries, and every UI (ui/, which includes the terminal image protocol).
var bannedDeps = []string{
	"charm.land/",
	"github.com/charmbracelet/",
	"github.com/mm1995tk/shijimi-editor/ui/",
}

// allowedDeps are exceptions to bannedDeps. github.com/charmbracelet/x/ansi
// still measures text in terminal cells in the Markdown preview, the agent
// panel and a few more places that build rows to draw; those go to the front
// end in a later step.
var allowedDeps = []string{
	"github.com/charmbracelet/x/ansi",
}

func TestNoTerminalDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
next:
	for dep := range strings.FieldsSeq(string(out)) {
		for _, ok := range allowedDeps {
			if dep == ok || strings.HasPrefix(dep, ok+"/") {
				continue next
			}
		}
		for _, banned := range bannedDeps {
			if strings.HasPrefix(dep, banned) {
				t.Errorf("core depends on %s", dep)
			}
		}
	}
}

// The core and its parts say what they mean as a msg.Msg (a key and values);
// the UI translates it. So none of them imports core/i18n, directly:
// config still reads the language and translates its own error report.
func TestNoTranslationInCoreAndParts(t *testing.T) {
	for _, pkg := range []string{".", "./internal/buffer", "./internal/lsp", "./internal/acp", "./internal/proxy", "./screen", "./msg"} {
		out, err := exec.Command("go", "list", "-f", `{{join .Imports "\n"}}`, pkg).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", pkg, err)
		}
		for imp := range strings.FieldsSeq(string(out)) {
			if imp == "github.com/mm1995tk/shijimi-editor/core/i18n" {
				t.Errorf("%s imports %s", pkg, imp)
			}
		}
	}
}

// The parts under core/internal are named by the UIs and cmd only through
// core's own API (export.go); nothing outside core imports them. (The compiler
// refuses it already; this keeps the rule in view and covers tests too.)
func TestUIsDoNotImportInternal(t *testing.T) {
	out, err := exec.Command("go", "list", "-test", "-f", `{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`, "../ui/...", "../cmd/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		for _, imp := range f[1:] {
			if strings.HasPrefix(imp, "github.com/mm1995tk/shijimi-editor/core/internal/") {
				t.Errorf("%s imports %s", f[0], imp)
			}
		}
	}
}
