package ui

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

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
