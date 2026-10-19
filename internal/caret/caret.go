// Package caret keeps the terminal's own cursor hidden while bubbletea moves
// it to where the user is typing.
//
// The editor draws its cursors itself. An input method (IME), however, shows
// the text being composed at the terminal's cursor, so the app hands bubbletea
// the typing position as the view's cursor. bubbletea then also shows that
// cursor; Writer drops the "show cursor" sequence so it stays hidden.
package caret

import (
	"bytes"
	"io"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// File is what bubbletea needs from its output to treat it as a terminal.
type File interface {
	io.ReadWriteCloser
	Fd() uintptr
}

// Writer wraps the terminal output. It forwards Read, Close and Fd so that
// bubbletea still recognises the output as a terminal.
type Writer struct {
	f  File
	mu sync.Mutex // keeps Send from landing in the middle of a frame
}

// NewWriter wraps f.
func NewWriter(f File) *Writer { return &Writer{f: f} }

// Send writes p to the terminal as is, between frames. It is for sequences
// that draw nothing, such as setting the clipboard (OSC 52).
func (w *Writer) Send(p []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.f.Write(p)
	return err
}

var showCursor = []byte(ansi.SetModeTextCursorEnable)

// Write writes p without any "show cursor" sequence. Call ShowCursor once the
// program has exited to give the shell its cursor back.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.f.Write(bytes.ReplaceAll(p, showCursor, nil)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ShowCursor makes the terminal cursor visible again.
func (w *Writer) ShowCursor() error { return w.Send(showCursor) }

func (w *Writer) Read(p []byte) (int, error) { return w.f.Read(p) }
func (w *Writer) Close() error               { return w.f.Close() }
func (w *Writer) Fd() uintptr                { return w.f.Fd() }
