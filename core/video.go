package core

import (
	"path/filepath"
	"strings"
)

var videoMIME = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".mov":  "video/quicktime",
	".webm": "video/webm",
	".ogv":  "video/ogg",
	".mkv":  "video/x-matroska",
}

// IsVideo reports whether path is a video file, by its extension.
func IsVideo(path string) bool {
	return VideoMIME(path) != ""
}

// VideoMIME is the media type of the video at path ("" when it is not a video).
func VideoMIME(path string) string {
	return videoMIME[strings.ToLower(filepath.Ext(path))]
}

// VideoOpener is an optional part of a Port: a UI that can play video in a view
// of its own (the TUI of the primary terminal of a GUI workspace asks the GUI
// for a pane). OpenFile hands such a UI the videos it is asked to open, instead
// of reading them as text. ok is false when the UI has no such view, and the
// file is opened as before; err says why the view could not be opened.
type VideoOpener interface {
	OpenVideo(path string) (ok bool, err error)
}
