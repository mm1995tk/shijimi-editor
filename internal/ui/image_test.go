package ui

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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
	lines := v.render(30, 12)
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
	v.render(30, 12)
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
	if got := len(v.render(10, 5)); got != 5 {
		t.Fatalf("render lines = %d", got)
	}
}
