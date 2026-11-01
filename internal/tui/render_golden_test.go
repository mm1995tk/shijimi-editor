package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// These tests pin what render() draws for a few representative states, so that
// the split of internal/ui into core and the TUI renderer (see
// docs/design/gui-protocol.md) can be checked not to change the TUI's look.
//
// The frames are stored without escape sequences (colors are not compared) in
// testdata/render/<name>.txt. After an intended change of the look, rewrite
// them with
//
//	go test ./internal/tui -run TestRenderGolden -update
//
// and look at the diff.
var updateGolden = flag.Bool("update", false, "rewrite the golden files of TestRenderGolden")

// goldenFrame is what a.render() draws, as plain text with trailing spaces
// trimmed and the temporary directory replaced by <root>.
func goldenFrame(a *App) string {
	rows := strings.Split(ansi.Strip(a.render()), "\n")
	for i, r := range rows {
		rows[i] = strings.TrimRight(r, " ")
	}
	s := strings.Join(rows, "\n") + "\n"
	return strings.ReplaceAll(s, a.Root, "<root>")
}

func checkGolden(t *testing.T, name string, a *App) {
	t.Helper()
	got := goldenFrame(a)
	path := filepath.Join("testdata", "render", name+".txt")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (create it with -update)", err)
	}
	if got != string(want) {
		t.Errorf("render differs from %s (rewrite with -update if intended)\n--- got ---\n%s--- want ---\n%s", path, got, want)
	}
}

const goldenSrc = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfor i := 0; i < 3; i++ {\n\t\tfmt.Println(i)\n\t}\n}\n"

// goldenApp is a 100x30 editor on a.txt, a file with no language server, so the status line
// does not depend on what is installed. The UI language is fixed to Japanese.
func goldenApp(t *testing.T) *App {
	t.Helper()
	i18n.SetLang(i18n.Japanese)
	t.Cleanup(func() { i18n.SetLang(i18n.Default) })
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte(goldenSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	a := New(dir, p)
	a.Helix = true
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a
}

func TestRenderGolden(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		checkGolden(t, "file", goldenApp(t))
	})

	t.Run("split", func(t *testing.T) {
		a := goldenApp(t)
		a.SplitOpen()
		keys(a, "esc") // close the file picker the split opens
		a.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		if !a.Split {
			t.Fatal("the window is not split")
		}
		checkGolden(t, "split", a)
	})

	t.Run("completion", func(t *testing.T) {
		a := goldenApp(t)
		keys(a, "G", "o", "f", "m")
		cur := a.CurText()
		// completing from the start of the word "fm" just typed
		start := buffer.Pos{Row: cur.Buf.Cur.Row, Col: cur.Buf.Cur.Col - len("fm")}
		a.Update(core.CompletionMsg{Tab: cur, Start: start, Items: []lsp.CompletionItem{
			{Label: "fmt", Detail: "package", Kind: 9},
			{Label: "fmt.Println", Detail: "func(a ...any)", Kind: 3},
			{Label: "fmt.Sprintf", Detail: "func(format string, a ...any) string", Kind: 3},
		}})
		if a.Comp == nil {
			t.Fatal("no completion popup")
		}
		checkGolden(t, "completion", a)
	})

	t.Run("hover", func(t *testing.T) {
		a := goldenApp(t)
		keys(a, "j", "j", "j", "j", "j", "w", "l")
		a.HoverText = goplsHover
		checkGolden(t, "hover", a)
	})

	t.Run("confirm", func(t *testing.T) {
		a := goldenApp(t)
		a.Confirm = &core.YesNo{Prompt: "a.txt を閉じますか？ (y/n)"}
		checkGolden(t, "confirm", a)
	})

	t.Run("palette-commands", func(t *testing.T) {
		a := goldenApp(t)
		a.Commands = []core.Command{{Name: "Save", Key: "Ctrl+S"}, {Name: "Save As", Key: "Ctrl+Shift+S"}, {Name: "Quit", Key: "Ctrl+Q"}}
		a.Pal.Show(a.Root, ">")
		keys(a, "s", "a")
		checkGolden(t, "palette-commands", a)
	})

	t.Run("palette-files", func(t *testing.T) {
		a := goldenApp(t)
		a.Pal.Show(a.Root, "")
		checkGolden(t, "palette-files", a)
	})

	t.Run("palette-prompt", func(t *testing.T) {
		a := goldenApp(t)
		a.Pal.Ask("名前", "abc", func(string) []core.Effect { return nil })
		checkGolden(t, "palette-prompt", a)
	})

	t.Run("palette-pick", func(t *testing.T) {
		a := goldenApp(t)
		a.Pal.Pick("テーマ", []core.PaletteItem{{Label: "dark", Detail: "ダーク"}, {Label: "light"}, {Label: "solar"}}, func(int) []core.Effect { return nil })
		keys(a, "l")
		checkGolden(t, "palette-pick", a)
	})

	t.Run("palette-nomatch", func(t *testing.T) {
		a := goldenApp(t)
		a.Pal.Show(a.Root, "zzzzqqq")
		checkGolden(t, "palette-nomatch", a)
	})

	t.Run("search", func(t *testing.T) {
		a := goldenApp(t)
		drive(a, cmdOf(a, a.OpenSearchArgs("fmt", false)))
		if !a.Search.Open || len(a.Search.Res.Hits) == 0 {
			t.Fatalf("search not showing hits: open=%v", a.Search.Open)
		}
		checkGolden(t, "search", a)
	})

	t.Run("cmdline", func(t *testing.T) {
		a := goldenApp(t)
		keys(a, ":", "w", "r", "i")
		if !a.CmdOpen {
			t.Fatal("the command line is not open")
		}
		checkGolden(t, "cmdline", a)
	})

	t.Run("find", func(t *testing.T) {
		a := goldenApp(t)
		drive(a, a.effectCmd(a.OpenFind()...))
		keys(a, "f", "m", "t")
		if !a.FindOpen {
			t.Fatal("the find bar is not open")
		}
		checkGolden(t, "find", a)
	})

	t.Run("status", func(t *testing.T) {
		a := goldenApp(t)
		keys(a, "i", "x", "esc", "v", "l")
		a.Msg = "保存しました"
		checkGolden(t, "status", a)
	})

	t.Run("diff", func(t *testing.T) {
		// the branch name shows in the status line
		t.Setenv("GIT_CONFIG_COUNT", "1")
		t.Setenv("GIT_CONFIG_KEY_0", "init.defaultBranch")
		t.Setenv("GIT_CONFIG_VALUE_0", "main")
		a, _ := gitDiffApp(t)
		a.Gd.Split = true
		checkGolden(t, "diff", a)
	})
}

// render is the frame renderFrame draws, without the cursor.
func (a *App) render() string {
	a.fit()
	frame, _ := a.renderFrame()
	return frame
}

// renderFrameText is the frame renderFrame draws from the state as it is, without
// fit first.
func (a *App) renderFrameText() string {
	frame, _ := a.renderFrame()
	return frame
}
func TestLineStates(t *testing.T) {
	a := goldenApp(t)
	keys(a, "i", "x", "esc")
	a.Msg, a.MsgErr = "失敗", true

	if ti := a.TitleOf(a.Cur()); ti == nil || ti.Text != "a.txt" || !ti.Dirty {
		t.Errorf("title = %+v", ti)
	}
	if p := core.BuildTextPane(a.Cur(), 40, 3, a.EditorOpts(a.Cur(), true).PaneOpts); p.Title.Text != "a.txt" || !p.Title.Dirty {
		t.Errorf("pane title = %+v", p.Title)
	}
	s := a.StatusOf()
	if s.Mode != "NOR" || s.Msg != "失敗" || !s.MsgErr || s.Position != "1:2" {
		t.Errorf("status = %+v", s)
	}
	if a.CmdlineState() != nil || a.FindState() != nil {
		t.Error("lines are not open")
	}
	keys(a, ":", "w", "q")
	if c := a.CmdlineState(); c == nil || c.Prefix != ":" || c.Text != "wq" || c.Cursor != 2 {
		t.Errorf("cmdline = %+v", c)
	}
}
