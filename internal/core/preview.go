package core

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
	Err  string
	Img  *ImageView

	// stamp is the size and modification time of an image file when it was
	// last read; a change makes it read again.
	Stamp fileStamp
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

// MermaidStartMsg asks to render the Mermaid diagram Code, cached under Key.
type MermaidStartMsg struct{ Key, Code string }

// MermaidDoneMsg is a rendered Mermaid diagram, or Err.
type MermaidDoneMsg struct {
	key string
	png string
	Err error
}

// MermaidBlocks returns the source of every mermaid fence in a document.
// It runs the renderer itself so the blocks found are exactly those the
// preview asks to draw.
func MermaidBlocks(text string) []string {
	blocks, _ := previewSources(text)
	return blocks
}

// previewSources returns the mermaid fences and the sources of the images
// shown on their own line, as the preview finds them.
func previewSources(text string) (blocks, images []string) {
	RenderMarkdown(text, 80, MdOpts{
		Mermaid: func(code string, _ int) *MdMedia {
			blocks = append(blocks, code)
			return nil
		},
		image: func(_, src string, _ int) *MdMedia {
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

// MdImageDoneMsg is an image of the Markdown preview, loaded from Path.
type MdImageDoneMsg struct {
	Path  string
	Stamp fileStamp
	Img   *ImageView
}

// MermaidKey is the cache key of the diagram code.
func (a *Core) MermaidKey(code string) string {
	sum := sha1.Sum([]byte(a.Cfg.Editor.MermaidCommand + "\x00" + code))
	return hex.EncodeToString(sum[:])
}

func mermaidCacheDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "shijimi", "mermaid")
}

// PreviewMode is how a Markdown tab shows its preview.
type PreviewMode int

const (
	PreviewOff   PreviewMode = iota
	PreviewSplit             // beside the buffer
	PreviewFull              // the preview alone, using the whole editor area
)

// TogglePreview cycles off → beside the buffer → full width → off.
func (a *Core) TogglePreview() []Effect {
	t := a.CurText()
	if t == nil || !isMarkdown(t.Buf.Path) {
		a.Fail(errors.New(i18n.T("preview.needmd")))
		return nil
	}
	t.Preview = (t.Preview + 1) % (PreviewFull + 1)
	t.scanned = false
	// the editor width changed: follow the cursor again from the left edge,
	// or a column scrolled to in the narrower view stays scrolled when it widens
	t.Buf.ScrollCol, a.Follow = 0, true
	a.Comp, a.HoverText = nil, "" // popups are anchored to the hidden buffer
	if t.Preview == PreviewFull && a.Mode == ModeInsert {
		a.Mode = ModeNormal // keystrokes must not edit a buffer that is not visible
	}
	return nil
}

// SyncPreview starts rendering for mermaid blocks and loading the images of
// the visible preview. The lists are cached per buffer version, so each call
// only does a few map lookups and a stat of each image.
func (a *Core) SyncPreview() []Effect {
	cmds := [][]Effect{a.syncMDImages()}
	blocks := a.wantedMermaid()
	if len(blocks) == 0 {
		return Batch(cmds...)
	}
	if a.Mermaid == nil {
		a.Mermaid = map[string]*previewJob{}
	}
	for _, code := range blocks {
		key := a.MermaidKey(code)
		if _, ok := a.Mermaid[key]; ok {
			continue
		}
		job := &previewJob{}
		a.Mermaid[key] = job
		png := filepath.Join(mermaidCacheDir(), key+".png")
		if _, err := os.Stat(png); err == nil {
			job.done, job.Img = true, NewImageView(png)
			continue
		}
		if strings.TrimSpace(a.Cfg.Editor.MermaidCommand) == "" {
			job.done, job.Err = true, i18n.T("preview.nomermaid")
			continue
		}
		code := code
		cmds = append(cmds, []Effect{StartTimer{After: mermaidDelay, Fire: MermaidStartMsg{key, code}}})
	}
	return Batch(cmds...)
}

// HandleMermaidStart renders a diagram, unless it left the file meanwhile.
func (a *Core) HandleMermaidStart(m MermaidStartMsg) []Effect {
	// drop the job if the user kept typing and the block no longer exists
	wanted := false
	for _, code := range a.wantedMermaid() {
		if a.MermaidKey(code) == m.Key {
			wanted = true
		}
	}
	if !wanted {
		delete(a.Mermaid, m.Key)
		return nil
	}
	command := a.Cfg.Editor.MermaidCommand
	return async(func() Event {
		dir := mermaidCacheDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return MermaidDoneMsg{key: m.Key, Err: err}
		}
		in := filepath.Join(dir, m.Key+".mmd")
		out := filepath.Join(dir, m.Key+".png")
		if err := os.WriteFile(in, []byte(m.Code), 0o644); err != nil {
			return MermaidDoneMsg{key: m.Key, Err: err}
		}
		defer os.Remove(in)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		line := strings.NewReplacer("{in}", ShellQuote(in), "{out}", ShellQuote(out)).Replace(command)
		if b, err := exec.CommandContext(ctx, "sh", "-c", line).CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(b))
			if msg == "" {
				msg = err.Error()
			}
			os.Remove(out)
			return MermaidDoneMsg{key: m.Key, Err: fmt.Errorf("%s", strings.SplitN(msg, "\n", 2)[0])}
		}
		return MermaidDoneMsg{key: m.Key, png: out}
	})
}

// HandleMermaidDone keeps a rendered diagram for the preview.
func (a *Core) HandleMermaidDone(m MermaidDoneMsg) {
	job := a.Mermaid[m.key]
	if job == nil {
		return
	}
	job.done = true
	a.PreviewGen++
	if m.Err != nil {
		job.Err = m.Err.Error()
		return
	}
	job.Img = NewImageView(m.png)
	if job.Img.Err != nil {
		job.Err = job.Img.Err.Error()
		job.Img = nil
	}
}

// syncMDImages starts loading the images of the visible preview, and loads
// an image again when its file changed. Decoding runs off the UI loop, so a
// large image does not stall typing. Images no longer in the preview are
// freed.
func (a *Core) syncMDImages() []Effect {
	t := a.CurText()
	if t == nil || t.Preview == PreviewOff {
		return nil
	}
	wanted := map[string]bool{}
	var cmds [][]Effect
	for _, src := range t.images() {
		path := mdImagePath(t.Buf.Path, src)
		if path == "" || wanted[path] {
			continue
		}
		wanted[path] = true
		stamp := statStamp(path)
		job, ok := a.MdImages[path]
		if ok && job.Stamp == stamp {
			continue
		}
		if !ok {
			if a.MdImages == nil {
				a.MdImages = map[string]*previewJob{}
			}
			job = &previewJob{}
			a.MdImages[path] = job
		}
		// a job being read again keeps showing the old result until the new one arrives
		job.Stamp = stamp
		cmds = append(cmds, async(func() Event { return MdImageDoneMsg{path, stamp, NewImageView(path)} }))
	}
	for path, job := range a.MdImages {
		if !wanted[path] {
			if job.Img != nil {
				job.Img.Close()
			}
			delete(a.MdImages, path)
			// a preview drawn before (another tab's) still refers to the
			// freed image; draw it again so it says the image is loading
			a.PreviewGen++
		}
	}
	return Batch(cmds...)
}

// HandleMDImageDone keeps a loaded image for the preview, unless the file
// changed meanwhile.
func (a *Core) HandleMDImageDone(m MdImageDoneMsg) {
	job := a.MdImages[m.Path]
	if job == nil || job.Stamp != m.Stamp {
		return // dropped, or the file changed again while it was read
	}
	if job.Img != nil {
		job.Img.Close()
	}
	job.done, job.Img, job.Err = true, nil, ""
	a.PreviewGen++
	if err := m.Img.Err; err != nil {
		// the path is shown already; keep only the reason
		if pe, ok := errors.AsType[*fs.PathError](err); ok {
			err = pe.Err
		}
		job.Err = err.Error()
		return
	}
	job.Img = m.Img
}

// mdImageLines makes the rows of an image that has a line of its own in the
// preview of the Markdown file mdPath, the same way as a mermaid diagram.
// Images that are not local files are left to be shown as text.
func (a *Core) mdImageLines(mdPath string) func(alt, src string, w int) *MdMedia {
	return func(alt, src string, w int) *MdMedia {
		path := mdImagePath(mdPath, src)
		if path == "" {
			return nil
		}
		label := func() []screen.Line {
			return []screen.Line{MdRow(fitSpans(mdSpans(inlineCells("🖼 "+alt, mdStyle{fg: screen.StyleMdImage})), w)...)}
		}
		return jobLines(a.MdImages[path], BlobFile+path, w, i18n.T("preview.loadingimage"), src+": ", label)
	}
}

// What a screen.ImageRef.Blob of the preview names: a local image file, or a
// mermaid diagram by the key of its source.
const (
	BlobFile    = "file:"
	blobMermaid = "mermaid:"
)

// PreviewImage is the image that a Blob of the preview names, nil when it is
// gone.
func (a *Core) PreviewImage(blob string) *ImageView {
	if path, ok := strings.CutPrefix(blob, BlobFile); ok {
		if job := a.MdImages[path]; job != nil {
			return job.Img
		}
	}
	if key, ok := strings.CutPrefix(blob, blobMermaid); ok {
		if job := a.Mermaid[key]; job != nil {
			return job.Img
		}
	}
	return nil
}

// jobLines makes the rows of a mermaid diagram or an image. When it is ready
// they are empty rows the image fills, named by blob. Until then, or when it
// failed, they are above (the source, or the image's alt text) with a line
// that says it is loading or what went wrong.
func jobLines(job *previewJob, blob string, w int, loading, errPrefix string, above func() []screen.Line) *MdMedia {
	note := func(text string, err bool) *MdMedia {
		st := screen.StyleLineNo
		if err {
			st = screen.StyleError
		}
		return &MdMedia{Lines: append(above(), MdRow(screen.Span{Text: Fit(" "+text, w), Style: st}))}
	}
	switch {
	case job == nil || !job.done:
		return note(loading, false)
	case job.Err != "" || job.Img == nil:
		return note(errPrefix+job.Err, true)
	}
	b := job.Img.Img.Bounds()
	rows := make([]screen.Line, Clamp(job.Img.RowsFor(w), 3, 28))
	for i := range rows {
		rows[i] = MdRow()
	}
	return &MdMedia{Lines: rows, Tight: job.Img.RowsFor(w) < 3, Image: &screen.ImageRef{Blob: blob, Width: b.Dx(), Height: b.Dy()}}
}

// ShellQuote is s quoted for sh.
func ShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// MermaidLines makes the rows of one diagram (or its placeholder) for the preview.
func (a *Core) MermaidLines(code string, w int) *MdMedia {
	source := func() []screen.Line { return mdCodeBlock(strings.Split(code, "\n"), "mermaid", w) }
	key := a.MermaidKey(code)
	return jobLines(a.Mermaid[key], blobMermaid+key, w, i18n.T("preview.rendering"), "mermaid: ", source)
}

// BuildPreviewPane makes the Pane of the Markdown preview of t, w columns
// wide and h rows high: the part of the document in view, scrolled so the part
// around the editor cursor is visible. It changes nothing but the cache of the
// document rendered at width w (prevCache), which is made again when the
// width, the buffer or an image changed.
func (a *Core) BuildPreviewPane(t *Tab, w, h int) screen.Pane {
	if t.prevCache == nil || t.PrevW != w || t.prevVer != t.Buf.Version || t.prevGen != a.PreviewGen {
		doc := RenderMarkdown(t.Buf.Text(), w, MdOpts{Mermaid: a.MermaidLines, image: a.mdImageLines(t.Buf.Path), FrontMatter: true})
		t.prevCache = &doc
		t.PrevW, t.prevVer, t.prevGen = w, t.Buf.Version, a.PreviewGen
	}
	all := t.prevCache.Lines
	top := 0
	if span := len(all) - h; span > 0 {
		n := max(1, len(t.Buf.Lines)-1)
		top = Clamp(t.Buf.Cur.Row*span/n, 0, span)
	}
	end := min(len(all), top+max(0, h))
	p := screen.Pane{
		ID:       t.Path() + "#preview",
		Kind:     screen.KindPreview,
		Title:    screen.Title{Text: t.Title(), Dirty: t.dirty()},
		Viewport: screen.Viewport{Top: top, Rows: h, Cols: w, Total: len(all)},
		CurLine:  t.Buf.Cur.Row,
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
func (a *Core) wantedMermaid() []string {
	var out []string
	if t := a.CurText(); t != nil && t.Preview != PreviewOff {
		out = t.mermaidBlocks()
	}
	return append(append([]string(nil), out...), a.reviewMermaid()...)
}

// mermaidBlocks returns the tab's mermaid fences.
func (t *Tab) mermaidBlocks() []string {
	t.scan()
	return t.scanBlocks
}

// images returns the sources of the tab's images that have a line of their own.
func (t *Tab) images() []string {
	t.scan()
	return t.scanImages
}

// scan finds the mermaid fences and images again, only after the buffer changed.
func (t *Tab) scan() {
	if !t.scanned || t.scanVer != t.Buf.Version {
		t.scanBlocks, t.scanImages = previewSources(t.Buf.Text())
		t.scanVer, t.scanned = t.Buf.Version, true
	}
}

// FullPreview reports whether t is showing the Markdown preview alone. In
// diff mode the diff takes the whole area instead, so the buffer stays
// visible and editable.
func (a *Core) FullPreview(t *Tab) bool {
	return t != nil && t.Preview == PreviewFull && a.DiffFile(t) == nil
}
