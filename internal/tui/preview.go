package tui

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// Markdown preview: a read-only rendering shown to the right of the buffer.
// mermaid code blocks are rendered to PNG by an external command
// (`mermaid-command` in config.toml) and drawn with the Kitty graphics protocol.
// Local images that have a line of their own are drawn the same way.

const mermaidDelay = 300 * time.Millisecond

// previewJob is a mermaid diagram or an image of the preview being loaded,
// or the result.
type previewJob struct {
	done bool
	err  string
	img  *imageView

	// stamp is the size and modification time of an image file when it was
	// last read; a change makes it read again.
	stamp fileStamp
}

// fileStamp tells whether a file changed. The zero value is a missing file.
type fileStamp struct {
	size int64
	mod  time.Time
}

func statStamp(path string) fileStamp {
	st, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{st.Size(), st.ModTime()}
}

type mermaidStartMsg struct{ key, code string }

type mermaidDoneMsg struct {
	key string
	png string
	err error
}

// mermaidBlocks returns the source of every mermaid fence in a document.
// It runs the renderer itself so the blocks found are exactly those the
// preview asks to draw.
func mermaidBlocks(text string) []string {
	blocks, _ := previewSources(text)
	return blocks
}

// previewSources returns the mermaid fences and the sources of the images
// shown on their own line, as the preview finds them.
func previewSources(text string) (blocks, images []string) {
	renderMarkdown(text, 80, mdOpts{
		mermaid: func(code string, _ int) *mdMedia {
			blocks = append(blocks, code)
			return nil
		},
		image: func(_, src string, _ int) *mdMedia {
			images = append(images, src)
			return nil
		},
	})
	return blocks, images
}

// mdImagePath resolves an image source against the Markdown file that names
// it. It returns "" for sources that are not local files.
func mdImagePath(mdPath, src string) string {
	if src == "" || strings.Contains(src, "://") || strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "#") {
		return ""
	}
	if u, err := url.PathUnescape(src); err == nil {
		src = u
	}
	if filepath.IsAbs(src) {
		return src
	}
	return filepath.Join(filepath.Dir(mdPath), src)
}

type mdImageDoneMsg struct {
	path  string
	stamp fileStamp
	img   *imageView
}

func (a *App) mermaidKey(code string) string {
	sum := sha1.Sum([]byte(a.cfg.Editor.MermaidCommand + "\x00" + code))
	return hex.EncodeToString(sum[:])
}

func mermaidCacheDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "shijimi", "mermaid")
}

// previewMode is how a Markdown tab shows its preview.
type previewMode int

const (
	previewOff   previewMode = iota
	previewSplit             // beside the buffer
	previewFull              // the preview alone, using the whole editor area
)

// togglePreview cycles off → beside the buffer → full width → off.
func (a *App) togglePreview() []Effect {
	t := a.curText()
	if t == nil || !isMarkdown(t.buf.Path) {
		a.fail(errors.New(i18n.T("preview.needmd")))
		return nil
	}
	t.preview = (t.preview + 1) % (previewFull + 1)
	t.scanned = false
	// the editor width changed: follow the cursor again from the left edge,
	// or a column scrolled to in the narrower view stays scrolled when it widens
	t.buf.ScrollCol, a.follow = 0, true
	a.comp, a.hoverText = nil, "" // popups are anchored to the hidden buffer
	if t.preview == previewFull && a.mode == modeInsert {
		a.mode = modeNormal // keystrokes must not edit a buffer that is not visible
	}
	return nil
}

// syncPreview starts rendering for mermaid blocks and loading the images of
// the visible preview. The lists are cached per buffer version, so each call
// only does a few map lookups and a stat of each image.
func (a *App) syncPreview() []Effect {
	cmds := [][]Effect{a.syncMDImages()}
	blocks := a.wantedMermaid()
	if len(blocks) == 0 {
		return batch(cmds...)
	}
	if a.mermaid == nil {
		a.mermaid = map[string]*previewJob{}
	}
	for _, code := range blocks {
		key := a.mermaidKey(code)
		if _, ok := a.mermaid[key]; ok {
			continue
		}
		job := &previewJob{}
		a.mermaid[key] = job
		png := filepath.Join(mermaidCacheDir(), key+".png")
		if _, err := os.Stat(png); err == nil {
			job.done, job.img = true, newImageView(png)
			continue
		}
		if strings.TrimSpace(a.cfg.Editor.MermaidCommand) == "" {
			job.done, job.err = true, i18n.T("preview.nomermaid")
			continue
		}
		code := code
		cmds = append(cmds, []Effect{StartTimer{After: mermaidDelay, Fire: mermaidStartMsg{key, code}}})
	}
	return batch(cmds...)
}

func (a *App) handleMermaidStart(m mermaidStartMsg) []Effect {
	// drop the job if the user kept typing and the block no longer exists
	wanted := false
	for _, code := range a.wantedMermaid() {
		if a.mermaidKey(code) == m.key {
			wanted = true
		}
	}
	if !wanted {
		delete(a.mermaid, m.key)
		return nil
	}
	command := a.cfg.Editor.MermaidCommand
	return async(func() Event {
		dir := mermaidCacheDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return mermaidDoneMsg{key: m.key, err: err}
		}
		in := filepath.Join(dir, m.key+".mmd")
		out := filepath.Join(dir, m.key+".png")
		if err := os.WriteFile(in, []byte(m.code), 0o644); err != nil {
			return mermaidDoneMsg{key: m.key, err: err}
		}
		defer os.Remove(in)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		line := strings.NewReplacer("{in}", shellQuote(in), "{out}", shellQuote(out)).Replace(command)
		if b, err := exec.CommandContext(ctx, "sh", "-c", line).CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(b))
			if msg == "" {
				msg = err.Error()
			}
			os.Remove(out)
			return mermaidDoneMsg{key: m.key, err: fmt.Errorf("%s", strings.SplitN(msg, "\n", 2)[0])}
		}
		return mermaidDoneMsg{key: m.key, png: out}
	})
}

func (a *App) handleMermaidDone(m mermaidDoneMsg) {
	job := a.mermaid[m.key]
	if job == nil {
		return
	}
	job.done = true
	a.previewGen++
	if m.err != nil {
		job.err = m.err.Error()
		return
	}
	job.img = newImageView(m.png)
	if job.img.err != nil {
		job.err = job.img.err.Error()
		job.img = nil
	}
}

// syncMDImages starts loading the images of the visible preview, and loads
// an image again when its file changed. Decoding runs off the UI loop, so a
// large image does not stall typing. Images no longer in the preview are
// freed.
func (a *App) syncMDImages() []Effect {
	t := a.curText()
	if t == nil || t.preview == previewOff {
		return nil
	}
	wanted := map[string]bool{}
	var cmds [][]Effect
	for _, src := range t.images() {
		path := mdImagePath(t.buf.Path, src)
		if path == "" || wanted[path] {
			continue
		}
		wanted[path] = true
		stamp := statStamp(path)
		job, ok := a.mdImages[path]
		if ok && job.stamp == stamp {
			continue
		}
		if !ok {
			if a.mdImages == nil {
				a.mdImages = map[string]*previewJob{}
			}
			job = &previewJob{}
			a.mdImages[path] = job
		}
		// a job being read again keeps showing the old result until the new one arrives
		job.stamp = stamp
		cmds = append(cmds, async(func() Event { return mdImageDoneMsg{path, stamp, newImageView(path)} }))
	}
	for path, job := range a.mdImages {
		if !wanted[path] {
			if job.img != nil {
				job.img.close()
			}
			delete(a.mdImages, path)
			// a preview drawn before (another tab's) still refers to the
			// freed image; draw it again so it says the image is loading
			a.previewGen++
		}
	}
	return batch(cmds...)
}

func (a *App) handleMDImageDone(m mdImageDoneMsg) {
	job := a.mdImages[m.path]
	if job == nil || job.stamp != m.stamp {
		return // dropped, or the file changed again while it was read
	}
	if job.img != nil {
		job.img.close()
	}
	job.done, job.img, job.err = true, nil, ""
	a.previewGen++
	if err := m.img.err; err != nil {
		// the path is shown already; keep only the reason
		if pe, ok := errors.AsType[*fs.PathError](err); ok {
			err = pe.Err
		}
		job.err = err.Error()
		return
	}
	job.img = m.img
}

// mdImageLines makes the rows of an image that has a line of its own in the
// preview of the Markdown file mdPath, the same way as a mermaid diagram.
// Images that are not local files are left to be shown as text.
func (a *App) mdImageLines(mdPath string) func(alt, src string, w int) *mdMedia {
	return func(alt, src string, w int) *mdMedia {
		path := mdImagePath(mdPath, src)
		if path == "" {
			return nil
		}
		label := func() []screen.Line {
			return []screen.Line{mdRow(fitSpans(mdSpans(inlineCells("🖼 "+alt, mdStyle{fg: screen.StyleMdImage})), w)...)}
		}
		return jobLines(a.mdImages[path], blobFile+path, w, i18n.T("preview.loadingimage"), src+": ", label)
	}
}

// What a screen.ImageRef.Blob of the preview names: a local image file, or a
// mermaid diagram by the key of its source.
const (
	blobFile    = "file:"
	blobMermaid = "mermaid:"
)

// previewImage is the image that a Blob of the preview names, nil when it is
// gone.
func (a *App) previewImage(blob string) *imageView {
	if path, ok := strings.CutPrefix(blob, blobFile); ok {
		if job := a.mdImages[path]; job != nil {
			return job.img
		}
	}
	if key, ok := strings.CutPrefix(blob, blobMermaid); ok {
		if job := a.mermaid[key]; job != nil {
			return job.img
		}
	}
	return nil
}

// jobLines makes the rows of a mermaid diagram or an image. When it is ready
// they are empty rows the image fills, named by blob. Until then, or when it
// failed, they are above (the source, or the image's alt text) with a line
// that says it is loading or what went wrong.
func jobLines(job *previewJob, blob string, w int, loading, errPrefix string, above func() []screen.Line) *mdMedia {
	note := func(text string, err bool) *mdMedia {
		st := screen.StyleLineNo
		if err {
			st = screen.StyleError
		}
		return &mdMedia{Lines: append(above(), mdRow(screen.Span{Text: fit(" "+text, w), Style: st}))}
	}
	switch {
	case job == nil || !job.done:
		return note(loading, false)
	case job.err != "" || job.img == nil:
		return note(errPrefix+job.err, true)
	}
	b := job.img.img.Bounds()
	rows := make([]screen.Line, clamp(job.img.rowsFor(w), 3, 28))
	for i := range rows {
		rows[i] = mdRow()
	}
	return &mdMedia{Lines: rows, Tight: job.img.rowsFor(w) < 3, Image: &screen.ImageRef{Blob: blob, Width: b.Dx(), Height: b.Dy()}}
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// mermaidLines makes the rows of one diagram (or its placeholder) for the preview.
func (a *App) mermaidLines(code string, w int) *mdMedia {
	source := func() []screen.Line { return mdCodeBlock(strings.Split(code, "\n"), "mermaid", w) }
	key := a.mermaidKey(code)
	return jobLines(a.mermaid[key], blobMermaid+key, w, i18n.T("preview.rendering"), "mermaid: ", source)
}

// buildPreviewPane makes the Pane of the Markdown preview of t, w columns
// wide and h rows high: the part of the document in view, scrolled so the part
// around the editor cursor is visible. It changes nothing but the cache of the
// document rendered at width w (prevCache), which is made again when the
// width, the buffer or an image changed.
func (a *App) buildPreviewPane(t *tab, w, h int) screen.Pane {
	if t.prevCache == nil || t.prevW != w || t.prevVer != t.buf.Version || t.prevGen != a.previewGen {
		doc := renderMarkdown(t.buf.Text(), w, mdOpts{mermaid: a.mermaidLines, image: a.mdImageLines(t.buf.Path), frontMatter: true})
		t.prevCache = &doc
		t.prevW, t.prevVer, t.prevGen = w, t.buf.Version, a.previewGen
	}
	all := t.prevCache.Lines
	top := 0
	if span := len(all) - h; span > 0 {
		n := max(1, len(t.buf.Lines)-1)
		top = clamp(t.buf.Cur.Row*span/n, 0, span)
	}
	end := min(len(all), top+max(0, h))
	p := screen.Pane{
		ID:       t.path() + "#preview",
		Kind:     screen.KindPreview,
		Title:    screen.Title{Text: t.title(), Dirty: t.dirty()},
		Viewport: screen.Viewport{Top: top, Rows: h, Cols: w, Total: len(all)},
		CurLine:  t.buf.Cur.Row,
		Preview:  &screen.PreviewInfo{Top: top, Rows: len(all), Images: []screen.PreviewImage{}},
	}
	for i, l := range all[top:end] {
		l.No = top + i
		p.Lines = append(p.Lines, l)
	}
	for _, im := range t.prevCache.Images {
		if im.Row < end && im.Row+im.Rows > top {
			p.Preview.Images = append(p.Preview.Images, im)
		}
	}
	return p
}

// wantedMermaid returns the mermaid blocks on screen: in the current tab's
// preview, and in an open review comment or comment editor.
func (a *App) wantedMermaid() []string {
	var out []string
	if t := a.curText(); t != nil && t.preview != previewOff {
		out = t.mermaidBlocks()
	}
	return append(append([]string(nil), out...), a.reviewMermaid()...)
}

// mermaidBlocks returns the tab's mermaid fences.
func (t *tab) mermaidBlocks() []string {
	t.scan()
	return t.scanBlocks
}

// images returns the sources of the tab's images that have a line of their own.
func (t *tab) images() []string {
	t.scan()
	return t.scanImages
}

// scan finds the mermaid fences and images again, only after the buffer changed.
func (t *tab) scan() {
	if !t.scanned || t.scanVer != t.buf.Version {
		t.scanBlocks, t.scanImages = previewSources(t.buf.Text())
		t.scanVer, t.scanned = t.buf.Version, true
	}
}

// fullPreview reports whether t is showing the Markdown preview alone. In
// diff mode the diff takes the whole area instead, so the buffer stays
// visible and editable.
func (a *App) fullPreview(t *tab) bool {
	return t != nil && t.preview == previewFull && a.diffFile(t) == nil
}
