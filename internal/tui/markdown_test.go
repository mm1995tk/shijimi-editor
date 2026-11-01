package tui

import (
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

const mdSample = "# 見出し\n\n段落に **太字** と `code` と [リンク](http://x)。\n\n- 項目1\n- 項目2\n  - 入れ子\n\n1. 一\n2. 二\n\n```go\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```mermaid\ngraph TD; A-->B\n```\n"

func TestRenderMarkdownWidth(t *testing.T) {
	for _, w := range []int{20, 40, 80} {
		called := false
		lines := testMd(mdSample, w, mdOpts{mermaid: func(code string, w int) *mdMedia {
			called = strings.Contains(code, "A-->B")
			return &mdMedia{Lines: []screen.Line{mdRow(plainSpan(strings.Repeat("x", w)))}}
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
	lines := testMd(src, w, mdOpts{})
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

func TestMermaidBlocks(t *testing.T) {
	got := mermaidBlocks(mdSample)
	if len(got) != 1 || got[0] != "graph TD; A-->B" {
		t.Fatalf("got %q", got)
	}
}

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
	cfg := &config.Config{Editor: config.Editor{MermaidCommand: "cp " + shellQuote(src) + " {out}"}}
	a := &App{cfg: cfg, tabs: []*tab{newTab(b)}}
	a.tabs[0].preview = previewSplit

	if cmd := a.syncPreview(); cmd == nil {
		t.Fatal("expected a scheduled render")
	}
	code := mermaidBlocks(b.Text())[0]
	key := a.mermaidKey(code)
	if got := a.testMedia(a.mermaidLines(code, 30), 30); !strings.Contains(ansi.Strip(strings.Join(got, "\n")), "描画中") {
		t.Fatalf("pending placeholder expected: %q", got)
	}
	msg := cmdOf(a, a.handleMermaidStart(mermaidStartMsg{key, code}))()
	done, ok := msg.(mermaidDoneMsg)
	if !ok || done.err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	a.handleMermaidDone(done)
	lines := a.testMedia(a.mermaidLines(code, 30), 30)
	if len(lines) < 3 || strings.Contains(ansi.Strip(strings.Join(lines, "")), "mermaid") {
		t.Fatalf("diagram expected, got %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) != 30 {
			t.Fatalf("width %d", ansi.StringWidth(l))
		}
	}
}

func TestMermaidBlocksMatchRenderer(t *testing.T) {
	cases := map[string][]string{
		"```mermaid\nA-->B\n```\n```mermaid\nC-->D\n```\n": {"A-->B", "C-->D"},
		"```mermaid\n```\n":              {""},
		"```mermaid\nA-->B":              {"A-->B"},
		"> ```mermaid\n> A-->B\n> ```\n": {"A-->B"},
	}
	for src, want := range cases {
		got := mermaidBlocks(src)
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("%q: got %q want %q", src, got, want)
		}
	}
}

func TestLongFenceNotClosedByShorter(t *testing.T) {
	src := "````md\n```mermaid\ngraph TD\n```\n````\n\nafter text\n"
	if got := mermaidBlocks(src); len(got) != 0 {
		t.Fatalf("example inside a longer fence must not be drawn: %q", got)
	}
	plain := ansi.Strip(strings.Join(testMd(src, 40, mdOpts{}), "\n"))
	if !strings.Contains(plain, "after text") || strings.Contains(plain, "````") {
		t.Fatalf("unexpected render:\n%s", plain)
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
	a := &App{cfg: &config.Config{Editor: config.Editor{MermaidCommand: "true {in} {out}"}}, tabs: []*tab{newTab(a1), newTab(b1)}}
	a.tabs[0].preview = previewSplit

	if a.syncPreview() == nil {
		t.Fatal("expected a scheduled render")
	}
	code := mermaidBlocks(a1.Text())[0]
	key := a.mermaidKey(code)
	a.active = 1 // switch away before the timer fires
	if cmd := a.handleMermaidStart(mermaidStartMsg{key, code}); cmd != nil {
		t.Fatal("job for a hidden tab must not start")
	}
	if _, ok := a.mermaid[key]; ok {
		t.Fatal("job should have been dropped")
	}
	a.active = 0 // back, without editing
	if a.syncPreview() == nil {
		t.Fatal("job must be re-registered after returning to the tab")
	}
	if _, ok := a.mermaid[key]; !ok {
		t.Fatal("job missing after return")
	}
}

func TestTogglePreviewCyclesToFullWidth(t *testing.T) {
	b := buffer.New()
	b.Insert("# 見出し\n")
	b.Path = "x.md"
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}}
	l := layout{rightW: 81, editH: 5}
	tb := a.tabs[0]

	want := []struct {
		mode  previewMode
		editW int
	}{{previewSplit, 40}, {previewFull, 0}, {previewOff, 81}}
	for _, w := range want {
		a.togglePreview()
		if tb.preview != w.mode || a.editW(tb, l) != w.editW {
			t.Fatalf("mode %v editW %d, want %v %d", tb.preview, a.editW(tb, l), w.mode, w.editW)
		}
	}

	tb.preview = previewFull
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
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}, helix: true, mode: modeInsert}
	tb := a.tabs[0]
	tb.preview = previewFull
	ver := b.Version

	press := func(s string) {
		a.handleHelixKey(textKey(s))
	}
	for _, k := range []string{"i", "a", "o", "d", "x", "x", "K"} {
		press(k)
		if a.mode != modeNormal || b.Version != ver {
			t.Fatalf("key %q: mode %v, version %d (want %d)", k, a.mode, b.Version, ver)
		}
	}

	press("j") // scrolling still works
	if a.pending != "" {
		t.Fatalf("pending %q", a.pending)
	}

	a.Update(hoverMsg{text: "doc"})
	if a.hoverText != "" {
		t.Fatalf("hover shown in full preview: %q", a.hoverText)
	}
}

func TestFullPreviewBlocksLeaderPasteAndVscodeKeymap(t *testing.T) {
	b := buffer.New()
	b.Insert("abc\n")
	b.Path = "x.md"
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}, helix: true, pal: &palette{}, search: &searchPanel{}}
	a.tabs[0].preview = previewFull
	ver := b.Version

	a.handleHelixKey(textKey(" "))
	a.handleHelixKey(textKey("p"))
	if b.Version != ver || a.pending != "" {
		t.Fatalf("Space p: version %d (want %d), pending %q", b.Version, ver, a.pending)
	}

	a.helix = false
	a.handleKey(textKey("j"))
	a.handlePaste(Paste{Content: "pasted"})
	if b.Version != ver {
		t.Fatalf("vscode keymap edited the hidden buffer: version %d, want %d", b.Version, ver)
	}
}

func TestPreviewFrontMatterAsYAML(t *testing.T) {
	src := "---\nname: skill\ndescription: 説明\n---\n\n# 見出し\n"
	lines := testMd(src, 40, mdOpts{frontMatter: true})
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, " yaml") || !strings.Contains(plain, "name: skill") || !strings.Contains(plain, "description: 説明") {
		t.Fatalf("front matter not shown as YAML:\n%s", plain)
	}
	if strings.Contains(plain, "─") && strings.Index(plain, "─") < strings.Index(plain, "見出し") {
		t.Fatalf("front matter fences drawn as rules:\n%s", plain)
	}

	// without the option (review comments), "---" stays a rule
	plain = ansi.Strip(strings.Join(testMd(src, 40, mdOpts{}), "\n"))
	if strings.Contains(plain, "yaml") {
		t.Fatalf("front matter outside the preview:\n%s", plain)
	}
}

func TestLeavingPreviewKeepsNoSidewaysScroll(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x", 60)+"\n")
	tb := a.curText()
	tb.buf.Path = "x.md"
	tb.buf.Cur.Col = 50
	a.follow = true
	a.render()
	if tb.buf.ScrollCol != 0 {
		t.Fatalf("scrolled %d before the preview", tb.buf.ScrollCol)
	}
	for _, mode := range []previewMode{previewSplit, previewFull, previewOff} {
		a.togglePreview()
		a.follow = true // as after a cursor move
		a.render()
		if tb.preview != mode {
			t.Fatalf("mode %v, want %v", tb.preview, mode)
		}
	}
	if tb.buf.ScrollCol != 0 {
		t.Fatalf("scrolled %d after leaving the preview", tb.buf.ScrollCol)
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
		lines := testMd("```\n"+tc.line+"\n```\n", tc.w, mdOpts{
			wrapped: func(row int, space bool, indent int) { conts[row] = true },
			exact:   func(row, from, to int) { exact[row] = [2]int{from, to} },
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
			got.WriteString(cutCols(ansi.Strip(l), x[0], x[1]))
		}
		if want := strings.ReplaceAll(tc.line, "\t", "    "); got.String() != want {
			t.Fatalf("w=%d: rows join to\n%q\nwant\n%q", tc.w, got.String(), want)
		}
	}

	// a line that fits is not reported
	called := false
	testMd("```\nshort\n```\n", 20, mdOpts{exact: func(int, int, int) { called = true }})
	if called {
		t.Fatal("a line that fits reported as cut")
	}
}

func TestPreviewSourcesFindsSoleImages(t *testing.T) {
	src := "![図](img/a.png)\n\n文の中の ![b](b.png) は文字のまま\n\n> ![c](c.png)\n"
	_, images := previewSources(src)
	if strings.Join(images, "|") != "img/a.png|c.png" {
		t.Fatalf("images = %q", images)
	}
}

func TestMDImagePath(t *testing.T) {
	cases := map[string]string{
		"a.png":                 "/doc/a.png",
		"../img/a%20b.png":      "/img/a b.png",
		"/abs/a.png":            "/abs/a.png",
		"https://example.com/a": "",
		"data:image/png;base64": "",
		"":                      "",
	}
	for src, want := range cases {
		if got := mdImagePath("/doc/x.md", src); got != want {
			t.Errorf("%q: got %q want %q", src, got, want)
		}
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
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}}
	a.tabs[0].preview = previewSplit
	const w = 30

	plain := func() string { return ansi.Strip(strings.Join(a.previewLines(a.tabs[0], w, 40), "\n")) }
	if got := plain(); !strings.Contains(got, "読み込み中") || !strings.Contains(got, "🖼 図") {
		t.Fatalf("pending placeholder expected:\n%s", got)
	}
	cmd := a.syncPreview()
	if cmd == nil {
		t.Fatal("expected images to load")
	}
	if len(a.mdImages) != 2 {
		t.Fatalf("only local images should load: %v", a.mdImages)
	}
	for path := range a.mdImages {
		a.handleMDImageDone(mdImageDoneMsg{path, a.mdImages[path].stamp, newImageView(path)})
	}
	lines := a.previewLines(a.tabs[0], w, 40)
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
	if a.syncPreview() != nil {
		t.Fatal("loaded images must not load again")
	}
}

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h)))
	f.Close()
}

func TestMarkdownImageReloadedWhenFileChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	b := buffer.New()
	b.Insert("![図](out.png)\n")
	b.Path = filepath.Join(dir, "x.md")
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}}
	a.tabs[0].preview = previewSplit
	load := func() {
		t.Helper()
		if a.syncPreview() == nil {
			t.Fatal("expected the image to load")
		}
		a.handleMDImageDone(mdImageDoneMsg{path, a.mdImages[path].stamp, newImageView(path)})
	}

	// written in the document before the file exists
	load()
	if a.mdImages[path].err == "" {
		t.Fatal("missing file should be an error")
	}
	writePNG(t, path, 40, 20)
	load()
	job := a.mdImages[path]
	if job.err != "" || job.img == nil {
		t.Fatalf("image expected after the file was made: %q", job.err)
	}
	first := job.img.rowsFor(30)

	writePNG(t, path, 40, 400)
	os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second))
	old := job.stamp
	a.syncPreview()
	if job.stamp == old || job.img == nil {
		t.Fatal("the old image should stay shown while the new one loads")
	}
	// a result read before the last change is dropped
	a.handleMDImageDone(mdImageDoneMsg{path, old, newImageView(path)})
	if job.img.rowsFor(30) != first {
		t.Fatal("stale result must be ignored")
	}
	a.handleMDImageDone(mdImageDoneMsg{path, job.stamp, newImageView(path)})
	if job.img.rowsFor(30) == first {
		t.Fatal("image should have been read again")
	}

	// removing the image from the document frees it
	b.Undo()
	b.Insert("text\n")
	gen := a.previewGen
	a.syncPreview()
	if len(a.mdImages) != 0 {
		t.Fatalf("unused image should be dropped: %v", a.mdImages)
	}
	if a.previewGen == gen {
		t.Fatal("previews drawn with the freed image must be drawn again")
	}
}

// testMd renders src as lines of w cells, with no images drawn.
func testMd(src string, w int, o mdOpts) []string {
	return (&App{}).drawMdDoc(renderMarkdown(src, w, o), w)
}

// testMedia draws the rows of m as lines of w cells.
func (a *App) testMedia(m *mdMedia, w int) []string {
	doc := mdDoc{Lines: m.Lines}
	if m.Image != nil {
		doc.Images = []screen.PreviewImage{{Rows: len(m.Lines), Image: *m.Image}}
	}
	return a.drawMdDoc(doc, w)
}
