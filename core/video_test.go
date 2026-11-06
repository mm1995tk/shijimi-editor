package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type videoFront struct {
	*testFront
	got []string
	ok  bool
	err error
}

func (f *videoFront) OpenVideo(p string) (bool, error) {
	f.got = append(f.got, p)
	return f.ok, f.err
}

func TestIsVideo(t *testing.T) {
	for p, want := range map[string]bool{"a.mp4": true, "/x/A.MOV": true, "a.webm": true, "a.png": false, "a": false, "a.mp4.txt": false} {
		if IsVideo(p) != want {
			t.Errorf("IsVideo(%q) != %v", p, want)
		}
	}
	if VideoMIME("a.mkv") != "video/x-matroska" || VideoMIME("a.txt") != "" {
		t.Error("VideoMIME")
	}
}

// A UI that has a video view takes the video; one that has none (or a Port that is not a VideoOpener) opens it as before.
func TestOpenFileVideo(t *testing.T) {
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "a.mp4")
	if err := os.WriteFile(mp4, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, f := newTestCore(t, dir, "", 80, 24)
	vf := &videoFront{testFront: f, ok: true}
	c.SetPort(vf)
	n := len(c.Tabs)
	if !c.OpenFile(mp4) || len(c.Tabs) != n || len(vf.got) != 1 || vf.got[0] != mp4 {
		t.Fatalf("the UI takes the video: %d tabs, %v", len(c.Tabs), vf.got)
	}
	vf.err = errors.New("no room")
	if c.OpenFile(mp4) || !c.MsgErr {
		t.Fatal("a refusal is an error")
	}
	vf.ok, vf.err = false, nil
	if !c.OpenFile(mp4) || len(c.Tabs) != n+1 {
		t.Fatal("a UI without a video view opens the file as text")
	}
}
