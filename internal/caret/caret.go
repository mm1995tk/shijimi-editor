// Package caret parks the terminal's own cursor where the user is typing.
//
// bubbletea hides the terminal cursor and leaves it at the bottom-left corner
// after every frame; the editor draws its cursors itself. An input method
// (IME), however, shows the text being composed at the terminal's cursor, so
// Japanese input appeared in the corner instead of where it goes. Writer
// moves the (still hidden) cursor to the typing position after each write.
package caret

import (
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

// File is what bubbletea needs from its output to treat it as a terminal.
type File interface {
	io.ReadWriteCloser
	Fd() uintptr
}

// Writer wraps the terminal output. It forwards Read, Close and Fd so that
// bubbletea still recognises the output as a terminal.
type Writer struct {
	f   File
	mu  sync.Mutex    // keeps Send from landing in the middle of a frame
	pos atomic.Uint64 // (y+1)<<32 | (x+1); 0 = leave the cursor alone
}

// Send writes p to the terminal as is, between frames. It is for sequences
// that draw nothing, such as setting the clipboard (OSC 52).
func (w *Writer) Send(p []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.f.Write(p)
	return err
}

// NewWriter wraps f.
func NewWriter(f File) *Writer { return &Writer{f: f} }

// Set makes later writes park the cursor at column x, row y (0-based).
func (w *Writer) Set(x, y int) {
	if x < 0 || y < 0 {
		w.Clear()
		return
	}
	w.pos.Store(uint64(y+1)<<32 | uint64(x+1))
}

// Clear makes later writes leave the cursor where bubbletea put it.
func (w *Writer) Clear() { w.pos.Store(0) }

// Write writes p, followed by a move to the parked position in the same write.
// bubbletea starts every frame on the alternate screen from the home
// position, so moving the cursor afterwards does not disturb the next frame.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	pos := w.pos.Load()
	if pos == 0 {
		return w.f.Write(p)
	}
	buf := make([]byte, 0, len(p)+16)
	buf = append(buf, p...)
	buf = append(buf, "\x1b["...)
	buf = strconv.AppendUint(buf, pos>>32, 10)
	buf = append(buf, ';')
	buf = strconv.AppendUint(buf, pos&0xffffffff, 10)
	buf = append(buf, 'H')
	if _, err := w.f.Write(buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *Writer) Read(p []byte) (int, error) { return w.f.Read(p) }
func (w *Writer) Close() error               { return w.f.Close() }
func (w *Writer) Fd() uintptr                { return w.f.Fd() }
