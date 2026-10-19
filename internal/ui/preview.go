package ui

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Markdown preview: a read-only rendering shown to the right of the buffer.
// mermaid code blocks are rendered to PNG by an external command
// (`mermaid-command` in config.toml) and drawn with half blocks.

const mermaidDelay = 300 * time.Millisecond

type mermaidJob struct {
	done bool
	err  string
	img  *imageView

	cacheW int
	cache  []string
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
	var out []string
	renderMarkdown(text, 80, mdOpts{mermaid: func(code string, _ int) []string {
		out = append(out, code)
		return nil
	}})
	return out
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
	return filepath.Join(d, "dageki", "mermaid")
}

// previewMode is how a Markdown tab shows its preview.
type previewMode int

const (
	previewOff   previewMode = iota
	previewSplit             // beside the buffer
	previewFull              // the preview alone, using the whole editor area
)

// togglePreview cycles off → beside the buffer → full width → off.
func (a *App) togglePreview() tea.Cmd {
	t := a.curText()
	if t == nil || !isMarkdown(t.buf.Path) {
		a.fail(fmt.Errorf("Markdown ファイル（.md）を開いてください"))
		return nil
	}
	t.preview = (t.preview + 1) % (previewFull + 1)
	t.scanned = false
	a.comp, a.hoverText = nil, "" // popups are anchored to the hidden buffer
	if t.preview == previewFull && a.mode == modeInsert {
		a.mode = modeNormal // keystrokes must not edit a buffer that is not visible
	}
	return nil
}

// syncPreview starts rendering for mermaid blocks of the visible preview.
// The block list is cached per buffer version, so each call only does a few map lookups.
func (a *App) syncPreview() tea.Cmd {
	t := a.curText()
	if t == nil || t.preview == previewOff {
		return nil
	}
	if a.mermaid == nil {
		a.mermaid = map[string]*mermaidJob{}
	}
	var cmds []tea.Cmd
	for _, code := range t.mermaidBlocks() {
		key := a.mermaidKey(code)
		if _, ok := a.mermaid[key]; ok {
			continue
		}
		job := &mermaidJob{}
		a.mermaid[key] = job
		png := filepath.Join(mermaidCacheDir(), key+".png")
		if _, err := os.Stat(png); err == nil {
			job.done, job.img = true, newImageView(png)
			continue
		}
		if strings.TrimSpace(a.cfg.Editor.MermaidCommand) == "" {
			job.done, job.err = true, "mermaid-command が未設定のため図を描画できません"
			continue
		}
		code := code
		cmds = append(cmds, tea.Tick(mermaidDelay, func(time.Time) tea.Msg { return mermaidStartMsg{key, code} }))
	}
	return tea.Batch(cmds...)
}

func (a *App) handleMermaidStart(m mermaidStartMsg) tea.Cmd {
	// drop the job if the user kept typing and the block no longer exists
	wanted := false
	if t := a.curText(); t != nil && t.preview != previewOff {
		for _, code := range t.mermaidBlocks() {
			if a.mermaidKey(code) == m.key {
				wanted = true
			}
		}
	}
	if !wanted {
		delete(a.mermaid, m.key)
		return nil
	}
	command := a.cfg.Editor.MermaidCommand
	return func() tea.Msg {
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
	}
}

func (a *App) handleMermaidDone(m mermaidDoneMsg) {
	job := a.mermaid[m.key]
	if job == nil {
		return
	}
	job.done, job.cache = true, nil
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

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// mermaidLines draws one diagram (or its placeholder) for the preview.
func (a *App) mermaidLines(code string, w int) []string {
	job := a.mermaid[a.mermaidKey(code)]
	src := strings.Split(code, "\n")
	note := func(text string, err bool) []string {
		out := mdCodeBlock(src, "mermaid", w)
		st := stPanelDim
		if err {
			st = bgfg(cEditorBg, cError)
		}
		return append(out, st.Render(fit(" "+text, w)))
	}
	switch {
	case job == nil || !job.done:
		return note("⏳ mermaid を描画中…", false)
	case job.err != "":
		return note("mermaid: "+job.err, true)
	}
	if job.cache != nil && job.cacheW == w {
		return job.cache
	}
	b := job.img.img.Bounds()
	rows := (b.Dy()*w/max(1, b.Dx()) + 1) / 2 // rows needed to fill w columns
	rows = clamp(rows, 3, 28)
	lines := job.img.render(w, rows+1)[1:] // drop the info line
	job.cacheW, job.cache = w, lines
	return lines
}

// previewLines renders the document in the preview pane, scrolled so the
// part around the editor cursor is visible.
func (a *App) previewLines(t *tab, w, h int) []string {
	text := t.buf.Text()
	gen := 0
	for _, j := range a.mermaid {
		if j.done {
			gen++
		}
	}
	if t.prevCache == nil || t.prevW != w || t.prevVer != t.buf.Version || t.prevGen != gen {
		t.prevCache = renderMarkdown(text, w, mdOpts{mermaid: a.mermaidLines})
		t.prevW, t.prevVer, t.prevGen = w, t.buf.Version, gen
	}
	all := t.prevCache
	top := 0
	if span := len(all) - h; span > 0 {
		n := max(1, len(t.buf.Lines)-1)
		top = clamp(t.buf.Cur.Row*span/n, 0, span)
	}
	out := make([]string, 0, h)
	for i := 0; i < h; i++ {
		if top+i < len(all) {
			out = append(out, all[top+i])
		} else {
			out = append(out, stEmpty.Render(strings.Repeat(" ", w)))
		}
	}
	return out
}

// mermaidBlocks returns the tab's mermaid fences, rescanning only after the
// buffer changed.
func (t *tab) mermaidBlocks() []string {
	if !t.scanned || t.scanVer != t.buf.Version {
		t.scanBlocks, t.scanVer, t.scanned = mermaidBlocks(t.buf.Text()), t.buf.Version, true
	}
	return t.scanBlocks
}

// fullPreview reports whether t is showing the Markdown preview alone. In
// git-diff mode the diff takes the whole area instead, so the buffer stays
// visible and editable.
func (a *App) fullPreview(t *tab) bool {
	return t != nil && t.preview == previewFull && a.diffFile(t) == nil
}
