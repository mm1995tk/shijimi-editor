package core

import (
	"path/filepath"
	"strings"
)

// The kinds of file a UI can show in a view of its own (see PaneOpener). The
// names are the Kind of ctl.PaneRequest "open". To add a kind, add it here and
// to mediaKinds.
const (
	MediaVideo = "video"
	MediaImage = "image"
	MediaPDF   = "pdf"
)

// mediaKinds says which extensions (lower-case, with the dot) are of which
// kind. The video list is the one kisui-terminal's web view can play. The image
// list is the images the editor itself shows (imageExts) that kisui-terminal
// also shows, so ".tif" and ".tiff" stay in the editor, and ".avif" is added
// (the editor cannot decode it).
var mediaKinds = map[string]map[string]bool{
	MediaVideo: {".mp4": true, ".m4v": true, ".mov": true, ".webm": true, ".ogv": true, ".mkv": true},
	MediaImage: {".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".webp": true, ".avif": true},
	MediaPDF:   {".pdf": true},
}

// MediaKind is the kind of the file at path by its extension (MediaVideo,
// MediaImage or MediaPDF), or "" when it is none of them.
func MediaKind(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	for kind, exts := range mediaKinds {
		if exts[ext] {
			return kind
		}
	}
	return ""
}

// PaneOpener is an optional part of a Port: a UI that can show a video, an
// image or a PDF in a view of its own (the TUI of the primary terminal of a GUI
// workspace asks the GUI for a pane). OpenFile hands such a UI the files of
// these kinds it is asked to open, instead of reading them as text or showing
// an image in a tab. ok is false when the UI has no such view, and the file is
// opened as before; err says why the view could not be opened.
type PaneOpener interface {
	OpenMedia(kind, path string) (ok bool, err error)
}
