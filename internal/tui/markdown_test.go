package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestMermaidRenderWithExternalCommand(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	src := filepath.Join(t.TempDir(), "fixture.png")
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	f, _ := os.Create(src)
	png.Encode(f, img)
	f.Close()

	b := buffer.New()
	b.Insert("# t\n\n```mermaid\ngraph TD; A-->B\n```\n")
	b.Path = "x.md"
	cfg := &config.Config{Editor: config.Editor{MermaidCommand: "cp " + core.ShellQuote(src) + " {out}"}}
	a := appOf(&core.Core{Cfg: cfg, Tabs: []*core.Tab{core.NewTab(b)}})
	a.Tabs[0].Preview = core.PreviewSplit

	if cmd := a.SyncPreview(); cmd == nil {
		t.Fatal("expected a scheduled render")
	}
	code := core.MermaidBlocks(b.Text())[0]
	key := a.MermaidKey(code)
	if got := a.testMedia(a.MermaidLines(code, 30), 30); !strings.Contains(ansi.Strip(strings.Join(got, "\n")), "描画中") {
		t.Fatalf("pending placeholder expected: %q", got)
	}
	msg := cmdOf(a, a.HandleMermaidStart(core.MermaidStartMsg{key, code}))()
	done, ok := msg.(core.MermaidDoneMsg)
	if !ok || done.Err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	a.HandleMermaidDone(done)
	lines := a.testMedia(a.MermaidLines(code, 30), 30)
	if len(lines) < 3 || strings.Contains(ansi.Strip(strings.Join(lines, "")), "mermaid") {
		t.Fatalf("diagram expected, got %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) != 30 {
			t.Fatalf("width %d", ansi.StringWidth(l))
		}
	}
}

func TestMermaidJobRestoredAfterTabSwitch(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	a1 := buffer.New()
	a1.Insert("```mermaid\nA-->B\n```\n")
	a1.Path = "a.md"
	b1 := buffer.New()
	b1.Insert("other")
	b1.Path = "b.md"
	a := appOf(&core.Core{Cfg: &config.Config{Editor: config.Editor{MermaidCommand: "true {in} {out}"}}, Tabs: []*core.Tab{core.NewTab(a1), core.NewTab(b1)}})
	a.Tabs[0].Preview = core.PreviewSplit

	if a.SyncPreview() == nil {
		t.Fatal("expected a scheduled render")
	}
	code := core.MermaidBlocks(a1.Text())[0]
	key := a.MermaidKey(code)
	a.Active = 1 // switch away before the timer fires
	if cmd := a.HandleMermaidStart(core.MermaidStartMsg{key, code}); cmd != nil {
		t.Fatal("job for a hidden tab must not start")
	}
	if _, ok := a.Mermaid[key]; ok {
		t.Fatal("job should have been dropped")
	}
	a.Active = 0 // back, without editing
	if a.SyncPreview() == nil {
		t.Fatal("job must be re-registered after returning to the tab")
	}
	if _, ok := a.Mermaid[key]; !ok {
		t.Fatal("job missing after return")
	}
}

func TestTogglePreviewCyclesToFullWidth(t *testing.T) {
	b := buffer.New()
	b.Insert("# 見出し\n")
	b.Path = "x.md"
	a := appOf(&core.Core{Cfg: &config.Config{}, Tabs: []*core.Tab{core.NewTab(b)}})
	l := layout{rightW: 81, editH: 5}
	tb := a.Tabs[0]

	want := []struct {
		mode  core.PreviewMode
		editW int
	}{{core.PreviewSplit, 40}, {core.PreviewFull, 0}, {core.PreviewOff, 81}}
	for _, w := range want {
		a.TogglePreview()
		if tb.Preview != w.mode || a.editW(tb, l) != w.editW {
			t.Fatalf("mode %v editW %d, want %v %d", tb.Preview, a.editW(tb, l), w.mode, w.editW)
		}
	}

	tb.Preview = core.PreviewFull
	lines := a.renderText(tb, l, true)
	if len(lines) != l.editH || !strings.Contains(ansi.Strip(lines[0]), "見出し") {
		t.Fatalf("full preview: %q", lines)
	}
	for _, s := range lines {
		if ansi.StringWidth(s) != l.rightW {
			t.Fatalf("width %d, want %d", ansi.StringWidth(s), l.rightW)
		}
	}
}

func TestFullPreviewBlocksEditing(t *testing.T) {
	b := buffer.New()
	b.Insert("# 見出し\n")
	b.Path = "x.md"
	a := appOf(&core.Core{Cfg: &config.Config{}, Tabs: []*core.Tab{core.NewTab(b)}, Helix: true, Mode: core.ModeInsert})
	tb := a.Tabs[0]
	tb.Preview = core.PreviewFull
	ver := b.Version

	press := func(s string) {
		a.HandleHelixKey(textKey(s))
	}
	for _, k := range []string{"i", "a", "o", "d", "x", "x", "K"} {
		press(k)
		if a.Mode != core.ModeNormal || b.Version != ver {
			t.Fatalf("key %q: mode %v, version %d (want %d)", k, a.Mode, b.Version, ver)
		}
	}

	press("j") // scrolling still works
	if a.Pending != "" {
		t.Fatalf("pending %q", a.Pending)
	}

	a.Update(core.HoverMsg{Text: "doc"})
	if a.HoverText != "" {
		t.Fatalf("hover shown in full preview: %q", a.HoverText)
	}
}

func TestFullPreviewBlocksLeaderPasteAndVscodeKeymap(t *testing.T) {
	b := buffer.New()
	b.Insert("abc\n")
	b.Path = "x.md"
	a := appOf(&core.Core{Cfg: &config.Config{}, Tabs: []*core.Tab{core.NewTab(b)}, Helix: true, Pal: &core.Palette{}, Search: &core.SearchPanel{}})
	a.Tabs[0].Preview = core.PreviewFull
	ver := b.Version

	a.HandleHelixKey(textKey(" "))
	a.HandleHelixKey(textKey("p"))
	if b.Version != ver || a.Pending != "" {
		t.Fatalf("Space p: version %d (want %d), pending %q", b.Version, ver, a.Pending)
	}

	a.Helix = false
	a.HandleKey(textKey("j"))
	a.HandlePaste(core.Paste{Content: "pasted"})
	if b.Version != ver {
		t.Fatalf("vscode keymap edited the hidden buffer: version %d, want %d", b.Version, ver)
	}
}

func TestLeavingPreviewKeepsNoSidewaysScroll(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 60)+"\n")
	tb := a.CurText()
	tb.Buf.Path = "x.md"
	tb.Buf.Cur.Col = 50
	a.Follow = true
	a.render()
	if tb.Buf.ScrollCol != 0 {
		t.Fatalf("scrolled %d before the preview", tb.Buf.ScrollCol)
	}
	for _, mode := range []core.PreviewMode{core.PreviewSplit, core.PreviewFull, core.PreviewOff} {
		a.TogglePreview()
		a.Follow = true // as after a cursor move
		a.render()
		if tb.Preview != mode {
			t.Fatalf("mode %v, want %v", tb.Preview, mode)
		}
	}
	if tb.Buf.ScrollCol != 0 {
		t.Fatalf("scrolled %d after leaving the preview", tb.Buf.ScrollCol)
	}
}

func TestMarkdownImageInPreview(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "a.png"))
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 40, 20)))
	f.Close()

	b := buffer.New()
	b.Insert("# t\n\n![図](a.png)\n\n![無い](missing.png)\n\n![外](https://example.com/x.png)\n")
	b.Path = filepath.Join(dir, "x.md")
	a := appOf(&core.Core{Cfg: &config.Config{}, Tabs: []*core.Tab{core.NewTab(b)}})
	a.Tabs[0].Preview = core.PreviewSplit
	const w = 30

	plain := func() string { return ansi.Strip(strings.Join(a.previewLines(a.Tabs[0], w, 40), "\n")) }
	if got := plain(); !strings.Contains(got, "読み込み中") || !strings.Contains(got, "🖼 図") {
		t.Fatalf("pending placeholder expected:\n%s", got)
	}
	cmd := a.SyncPreview()
	if cmd == nil {
		t.Fatal("expected images to load")
	}
	if len(a.MdImages) != 2 {
		t.Fatalf("only local images should load: %v", a.MdImages)
	}
	for path := range a.MdImages {
		a.HandleMDImageDone(core.MdImageDoneMsg{path, a.MdImages[path].Stamp, core.NewImageView(path)})
	}
	lines := a.previewLines(a.Tabs[0], w, 40)
	for _, l := range lines {
		if ansi.StringWidth(l) != w {
			t.Fatalf("width %d: %q", ansi.StringWidth(l), ansi.Strip(l))
		}
	}
	got := plain()
	if strings.Contains(got, "🖼 図") || strings.Contains(got, "読み込み中") {
		t.Fatalf("image expected instead of the placeholder:\n%s", got)
	}
	if !strings.Contains(got, "missing.png: no such file") || strings.Contains(got, dir) || !strings.Contains(got, "🖼 無い") {
		t.Fatalf("error for the missing image expected:\n%s", got)
	}
	if !strings.Contains(got, "🖼 外") {
		t.Fatalf("remote image should stay text:\n%s", got)
	}
	if a.SyncPreview() != nil {
		t.Fatal("loaded images must not load again")
	}
}

func TestMarkdownImageReloadedWhenFileChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	b := buffer.New()
	b.Insert("![図](out.png)\n")
	b.Path = filepath.Join(dir, "x.md")
	a := appOf(&core.Core{Cfg: &config.Config{}, Tabs: []*core.Tab{core.NewTab(b)}})
	a.Tabs[0].Preview = core.PreviewSplit
	load := func() {
		t.Helper()
		if a.SyncPreview() == nil {
			t.Fatal("expected the image to load")
		}
		a.HandleMDImageDone(core.MdImageDoneMsg{path, a.MdImages[path].Stamp, core.NewImageView(path)})
	}

	// written in the document before the file exists
	load()
	if a.MdImages[path].Err == "" {
		t.Fatal("missing file should be an error")
	}
	writePNG(t, path, 40, 20)
	load()
	job := a.MdImages[path]
	if job.Err != "" || job.Img == nil {
		t.Fatalf("image expected after the file was made: %q", job.Err)
	}
	first := job.Img.RowsFor(30)

	writePNG(t, path, 40, 400)
	os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second))
	old := job.Stamp
	a.SyncPreview()
	if job.Stamp == old || job.Img == nil {
		t.Fatal("the old image should stay shown while the new one loads")
	}
	// a result read before the last change is dropped
	a.HandleMDImageDone(core.MdImageDoneMsg{path, old, core.NewImageView(path)})
	if job.Img.RowsFor(30) != first {
		t.Fatal("stale result must be ignored")
	}
	a.HandleMDImageDone(core.MdImageDoneMsg{path, job.Stamp, core.NewImageView(path)})
	if job.Img.RowsFor(30) == first {
		t.Fatal("image should have been read again")
	}

	// removing the image from the document frees it
	b.Undo()
	b.Insert("text\n")
	gen := a.PreviewGen
	a.SyncPreview()
	if len(a.MdImages) != 0 {
		t.Fatalf("unused image should be dropped: %v", a.MdImages)
	}
	if a.PreviewGen == gen {
		t.Fatal("previews drawn with the freed image must be drawn again")
	}
}

// testMd renders src as lines of w cells, with no images drawn.
func testMd(src string, w int, o core.MdOpts) []string {
	return (&App{}).drawMdDoc(core.RenderMarkdown(src, w, o), w)
}

// testMedia draws the rows of m as lines of w cells.
func (a *App) testMedia(m *core.MdMedia, w int) []string {
	doc := core.MdDoc{Lines: m.Lines}
	if m.Image != nil {
		doc.Images = []screen.PreviewImage{{Rows: len(m.Lines), Image: *m.Image}}
	}
	return a.drawMdDoc(doc, w)
}

func TestRenderMarkdownWidth(t *testing.T) {
	for _, w := range []int{20, 40, 80} {
		called := false
		lines := testMd(mdSample, w, core.MdOpts{Mermaid: func(code string, w int) *core.MdMedia {
			called = strings.Contains(code, "A-->B")
			return &core.MdMedia{Lines: []screen.Line{core.MdRow(core.PlainSpan(strings.Repeat("x", w)))}}
		}})
		if !called {
			t.Fatal("mermaid callback not called")
		}
		for i, l := range lines {
			if got := ansi.StringWidth(l); got != w {
				t.Fatalf("w=%d line %d width=%d: %q", w, i, got, ansi.Strip(l))
			}
		}
		plain := ansi.Strip(strings.Join(lines, "\n"))
		for _, want := range []string{"見出し", "太字", "• 項目1", "1.", "func main", "┌", "xxxx"} {
			if !strings.Contains(plain, want) {
				t.Errorf("w=%d: missing %q in\n%s", w, want, plain)
			}
		}
		if strings.Contains(plain, "**") || strings.Contains(plain, "```") {
			t.Errorf("w=%d: markup left over\n%s", w, plain)
		}
	}
}

func TestMarkdownTableWrapsLongCells(t *testing.T) {
	src := "| 名前 | 説明 |\n|---|---|\n| foo | とても長い説明文がここに入ります。この文は幅を超えるので折り返されるべきです。 |\n| bar | short words that should wrap at spaces nicely |\n"
	const w = 40
	lines := testMd(src, w, core.MdOpts{})
	var text strings.Builder
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Fatalf("line %d width=%d: %q", i, got, ansi.Strip(l))
		}
		// collect the second column's content
		parts := strings.Split(ansi.Strip(l), "│")
		if len(parts) >= 3 {
			text.WriteString(strings.TrimSpace(parts[2]))
		}
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"とても長い説明文がここに入ります。この文は幅を超えるので折り返されるべきです。", "shortwordsthatshouldwrapatspacesnicely"} {
		if !strings.Contains(strings.ReplaceAll(text.String(), " ", ""), want) {
			t.Errorf("missing %q in\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "│ foo  │") || !strings.Contains(plain, "│      │") {
		t.Errorf("long cell did not wrap onto extra lines\n%s", plain)
	}
}

func TestLongFenceNotClosedByShorter(t *testing.T) {
	src := "````md\n```mermaid\ngraph TD\n```\n````\n\nafter text\n"
	if got := core.MermaidBlocks(src); len(got) != 0 {
		t.Fatalf("example inside a longer fence must not be drawn: %q", got)
	}
	plain := ansi.Strip(strings.Join(testMd(src, 40, core.MdOpts{}), "\n"))
	if !strings.Contains(plain, "after text") || strings.Contains(plain, "````") {
		t.Fatalf("unexpected render:\n%s", plain)
	}
}

func TestPreviewFrontMatterAsYAML(t *testing.T) {
	src := "---\nname: skill\ndescription: 説明\n---\n\n# 見出し\n"
	lines := testMd(src, 40, core.MdOpts{FrontMatter: true})
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, " yaml") || !strings.Contains(plain, "name: skill") || !strings.Contains(plain, "description: 説明") {
		t.Fatalf("front matter not shown as YAML:\n%s", plain)
	}
	if strings.Contains(plain, "─") && strings.Index(plain, "─") < strings.Index(plain, "見出し") {
		t.Fatalf("front matter fences drawn as rules:\n%s", plain)
	}

	// without the option (review comments), "---" stays a rule
	plain = ansi.Strip(strings.Join(testMd(src, 40, core.MdOpts{}), "\n"))
	if strings.Contains(plain, "yaml") {
		t.Fatalf("front matter outside the preview:\n%s", plain)
	}
}

func TestCodeBlockWrapsLongLines(t *testing.T) {
	for _, tc := range []struct {
		line string
		w    int
	}{
		{"description: 打ち合わせで作っておいた GitHub の Issue を並行に消化する。\tend", 12},
		{"description: 打ち合わせで作っておいた GitHub の Issue を並行に消化する。\tend", 33},
		{"x := 1    // note", 10},
		{"ab❤️cdefgh", 6}, // measured as highlightText draws it
		{"abcdéfgh", 7},  // the combining mark stays with its letter
		{"a\tb\tc", 3},    // a tab is cut like four spaces
		{"aaaa日本語", 4},
	} {
		exact := map[int][2]int{}
		conts := map[int]bool{}
		lines := testMd("```\n"+tc.line+"\n```\n", tc.w, core.MdOpts{
			Wrapped: func(row int, space bool, indent int) { conts[row] = true },
			Exact:   func(row, from, to int) { exact[row] = [2]int{from, to} },
		})
		var got strings.Builder
		for i, l := range lines {
			if sw := ansi.StringWidth(l); sw != tc.w {
				t.Fatalf("%q w=%d: line %d width %d: %q", tc.line, tc.w, i, sw, ansi.Strip(l))
			}
			if i == 0 || i == len(lines)-1 { // the edges
				continue
			}
			x, ok := exact[i]
			if !ok || conts[i] != (i > 1) {
				t.Fatalf("%q w=%d: row %d exact %v cont %v", tc.line, tc.w, i, ok, conts[i])
			}
			got.WriteString(core.CutCols(ansi.Strip(l), x[0], x[1]))
		}
		if want := strings.ReplaceAll(tc.line, "\t", "    "); got.String() != want {
			t.Fatalf("w=%d: rows join to\n%q\nwant\n%q", tc.w, got.String(), want)
		}
	}

	// a line that fits is not reported
	called := false
	testMd("```\nshort\n```\n", 20, core.MdOpts{Exact: func(int, int, int) { called = true }})
	if called {
		t.Fatal("a line that fits reported as cut")
	}
}
