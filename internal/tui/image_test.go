package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestFitImage(t *testing.T) {
	cellW, cellH = 10, 20
	// a large image shrinks to the box and keeps its aspect ratio
	pw, ph, cols, rows := fitImage(2000, 1000, 50, 40)
	if pw != 500 || ph != 250 || cols != 50 || rows != 13 {
		t.Errorf("large: %d×%d px on %d×%d cells", pw, ph, cols, rows)
	}
	// a small image is never enlarged
	pw, ph, cols, rows = fitImage(32, 16, 50, 40)
	if pw != 32 || ph != 16 || cols != 4 || rows != 1 {
		t.Errorf("small: %d×%d px on %d×%d cells", pw, ph, cols, rows)
	}
}

func TestImageViewSendsOncePerSize(t *testing.T) {
	cellW, cellH = 10, 20
	takeGraphics()
	v := &imageView{path: "a.png", img: image.NewRGBA(image.Rect(0, 0, 400, 400)), format: "png", id: 7}
	a := appOf(&Core{tabs: []*tab{{kind: tabImage, img: v}}})
	lines := a.imageLines(a.tabs[0], 30, 12)
	if len(lines) != 12 {
		t.Fatalf("got %d lines", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 30 {
			t.Errorf("line %d width = %d", i, w)
		}
	}
	g := takeGraphics()
	if !strings.Contains(g, "\x1b_Ga=T,U=1,f=100,i=7,c=22,r=11,") {
		t.Errorf("transmit not queued: %q", g[:min(len(g), 60)])
	}
	themeGen++ // rebuild the lines without changing the size
	a.imageLines(a.tabs[0], 30, 12)
	if g := takeGraphics(); g != "" {
		t.Errorf("same size sent again")
	}
	v.close()
	if g := takeGraphics(); g != "\x1b_Ga=d,d=I,i=7,q=2\x1b\\" {
		t.Errorf("close = %q", g)
	}
}

func TestImageViewFromData(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	v := newImageViewFromData("page.png", buf.Bytes())
	if v.err != nil || v.format != "png" || v.name() != "page.png" {
		t.Fatalf("view = %+v", v)
	}
	if got := len((&App{}).drawImagePane(screen.Pane{Caption: v.info()}, 10, 5)); got != 5 {
		t.Fatalf("render lines = %d", got)
	}
}

func TestBuildImagePane(t *testing.T) {
	cellW, cellH = 10, 20
	v := &imageView{path: "/x/a.png", img: image.NewRGBA(image.Rect(0, 0, 400, 300)), format: "png", size: 2048, id: 9}
	a := appOf(&Core{tabs: []*tab{{kind: tabImage, img: v}}})
	p := a.buildImagePane(a.tabs[0], 30, 12)
	want := screen.ImageRef{Blob: blobFile + "/x/a.png", Width: 400, Height: 300, Format: "png", Size: 2048}
	if p.Kind != screen.KindImage || p.Title.Text != "a.png" || p.Image == nil || *p.Image != want || p.Caption != "PNG  400×300  2.0 KB" {
		t.Fatalf("pane = %+v", p)
	}
	if p.Viewport.Cols != 30 || p.Viewport.Rows != 12 {
		t.Fatalf("viewport = %+v", p.Viewport)
	}
	if a.blobImage(p.Image.Blob) != v {
		t.Fatal("the Blob does not name the image")
	}
	if _, err := json.Marshal(p); err != nil {
		t.Fatal(err)
	}

	// unreadable: no Image, the reason is the caption, and the area is blank
	bad := &imageView{path: "/x/b.png", err: errors.New("boom")}
	a.tabs = append(a.tabs, &tab{kind: tabImage, img: bad})
	p = a.buildImagePane(a.tabs[1], 30, 4)
	if p.Image != nil || !strings.Contains(p.Caption, "boom") {
		t.Fatalf("pane = %+v", p)
	}
	lines := a.drawImagePane(p, 30, 4)
	if len(lines) != 4 {
		t.Fatalf("got %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) != 30 {
			t.Errorf("width = %d", ansi.StringWidth(l))
		}
	}
}
