package core

import (
	"bytes"
	"errors"
	"image/png"
	"net/http"
	"os"
	"strings"
)

// ErrNoBlob is BlobData's error for a blob the core does not show.
var ErrNoBlob = errors.New("no such blob")

// BlobData is the bytes of the image that a screen.ImageRef.Blob names, for
// blob.get: a local image file as it is on disk, and a mermaid diagram as PNG.
// Only blobs the core shows (BlobImage) are given, so a GUI cannot read any
// file it likes through it.
func (a *Core) BlobData(blob string) (mime string, data []byte, err error) {
	iv := a.BlobImage(blob)
	if iv == nil || iv.Img == nil {
		return "", nil, ErrNoBlob
	}
	if path, ok := strings.CutPrefix(blob, BlobFile); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		return http.DetectContentType(data), data, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, iv.Img); err != nil {
		return "", nil, err
	}
	return "image/png", buf.Bytes(), nil
}
