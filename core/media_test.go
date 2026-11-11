package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type mediaFront struct {
	*testFront
	kinds, got []string
	ok         bool
	err        error
}

func (f *mediaFront) OpenMedia(kind, p string) (bool, error) {
	f.kinds = append(f.kinds, kind)
	f.got = append(f.got, p)
	return f.ok, f.err
}

func TestMediaKind(t *testing.T) {
	for p, want := range map[string]string{
		"a.mp4": MediaVideo, "/x/A.MOV": MediaVideo, "a.webm": MediaVideo,
		"a.png": MediaImage, "a.JPG": MediaImage, "a.avif": MediaImage,
		"a.pdf": MediaPDF, "/x/A.PDF": MediaPDF,
		"a.tiff": "", "a.txt": "", "a": "", "a.mp4.txt": "", "a.pdf.bak": "",
	} {
		if got := MediaKind(p); got != want {
			t.Errorf("MediaKind(%q) = %q, want %q", p, got, want)
		}
	}
}

// A UI that has a pane view takes a video, an image and a PDF; one that has none (or a Port that is not a PaneOpener) opens them as before.
func TestOpenFileMedia(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"video": "a.mp4", "image": "a.png", "pdf": "a.pdf"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, f := newTestCore(t, dir, "", 80, 24)
	mf := &mediaFront{testFront: f, ok: true}
	c.SetPort(mf)
	n := len(c.Tabs)
	for kind, name := range files {
		p := filepath.Join(dir, name)
		before := len(mf.got)
		if !c.OpenFile(p) || len(c.Tabs) != n || len(mf.got) != before+1 || mf.got[before] != p || mf.kinds[before] != kind {
			t.Fatalf("the UI takes the %s: %d tabs, %v %v", kind, len(c.Tabs), mf.kinds, mf.got)
		}
	}
	mp4 := filepath.Join(dir, "a.mp4")
	mf.err = errors.New("no room")
	if c.OpenFile(mp4) || !c.MsgErr {
		t.Fatal("a refusal is an error")
	}
	mf.ok, mf.err = false, nil
	if !c.OpenFile(mp4) || len(c.Tabs) != n+1 {
		t.Fatal("a UI without a video view opens the file as text")
	}
	if !c.OpenFile(filepath.Join(dir, "a.png")) || len(c.Tabs) != n+2 || c.Tabs[len(c.Tabs)-1].Kind != TabImage {
		t.Fatal("a UI without an image view shows the image in a tab")
	}
}
