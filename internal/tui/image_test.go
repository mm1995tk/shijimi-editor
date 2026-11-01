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

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestImageViewSendsOncePerSize(t *testing.T) {
	core.CellW, core.CellH = 10, 20
	core.TakeGraphics()
	v := &core.ImageView{Path: "a.png", Img: image.NewRGBA(image.Rect(0, 0, 400, 400)), Format: "png", Id: 7}
	a := appOf(&core.Core{Tabs: []*core.Tab{{Kind: core.TabImage, Img: v}}})
	lines := a.imageLines(a.Tabs[0], 30, 12)
	if len(lines) != 12 {
		t.Fatalf("got %d lines", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 30 {
			t.Errorf("line %d width = %d", i, w)
		}
	}
	g := core.TakeGraphics()
	if !strings.Contains(g, "\x1b_Ga=T,U=1,f=100,i=7,c=22,r=11,") {
		t.Errorf("transmit not queued: %q", g[:min(len(g), 60)])
	}
	core.ThemeGen++ // rebuild the lines without changing the size
	a.imageLines(a.Tabs[0], 30, 12)
	if g := core.TakeGraphics(); g != "" {
		t.Errorf("same size sent again")
	}
	v.Close()
	if g := core.TakeGraphics(); g != "\x1b_Ga=d,d=I,i=7,q=2\x1b\\" {
		t.Errorf("close = %q", g)
	}
}

func TestImageViewFromData(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	v := core.NewImageViewFromData("page.png", buf.Bytes())
	if v.Err != nil || v.Format != "png" || v.Name() != "page.png" {
		t.Fatalf("view = %+v", v)
	}
	if got := len(drawImagePane(screen.Pane{Caption: v.Info()}, nil, 10, 5)); got != 5 {
		t.Fatalf("render lines = %d", got)
	}
}

func TestBuildImagePane(t *testing.T) {
	core.CellW, core.CellH = 10, 20
	v := &core.ImageView{Path: "/x/a.png", Img: image.NewRGBA(image.Rect(0, 0, 400, 300)), Format: "png", Size: 2048, Id: 9}
	a := appOf(&core.Core{Tabs: []*core.Tab{{Kind: core.TabImage, Img: v}}})
	p := a.BuildImagePane(a.Tabs[0], 30, 12)
	want := screen.ImageRef{Blob: core.BlobFile + "/x/a.png", Width: 400, Height: 300, Format: "png", Size: 2048}
	if p.Kind != screen.KindImage || p.Title.Text != "a.png" || p.Image == nil || *p.Image != want || p.Caption != "PNG  400×300  2.0 KB" {
		t.Fatalf("pane = %+v", p)
	}
	if p.Viewport.Cols != 30 || p.Viewport.Rows != 12 {
		t.Fatalf("viewport = %+v", p.Viewport)
	}
	if a.BlobImage(p.Image.Blob) != v {
		t.Fatal("the Blob does not name the image")
	}
	if _, err := json.Marshal(p); err != nil {
		t.Fatal(err)
	}

	// unreadable: no Image, the reason is the caption, and the area is blank
	bad := &core.ImageView{Path: "/x/b.png", Err: errors.New("boom")}
	a.Tabs = append(a.Tabs, &core.Tab{Kind: core.TabImage, Img: bad})
	p = a.BuildImagePane(a.Tabs[1], 30, 4)
	if p.Image != nil || !strings.Contains(p.Caption, "boom") {
		t.Fatalf("pane = %+v", p)
	}
	lines := drawImagePane(p, nil, 30, 4)
	if len(lines) != 4 {
		t.Fatalf("got %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) != 30 {
			t.Errorf("width = %d", ansi.StringWidth(l))
		}
	}
}
