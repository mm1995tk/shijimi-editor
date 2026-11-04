package buffer

import (
	"path/filepath"
	"sync"
)

// Pool keeps the open files of a process so that opening a file that is
// already open gives another view of the same Doc instead of a second copy
// of its text. The cores of `shj serve` share one.
type Pool struct {
	mu   sync.Mutex
	docs map[string]*Doc // by fileKey
}

// NewPool makes an empty Pool.
func NewPool() *Pool { return &Pool{docs: map[string]*Doc{}} }

// fileKey names the file path points at, so that two spellings of one file
// (a symbolic link, a path with ".." in it) are one key. A file that is not
// there yet is named by its folder.
func fileKey(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	if real, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(real, filepath.Base(abs))
	}
	return abs
}

// Open is a new view of the file at path: of the Doc that is open, or else of
// one read from the disk (see the package-level Open). A nil Pool opens the
// file for this view alone.
func (p *Pool) Open(path string) (*Buffer, error) {
	if p == nil {
		return Open(path)
	}
	key := fileKey(path)
	p.mu.Lock()
	defer p.mu.Unlock()
	if d := p.docs[key]; d != nil {
		return d.NewView(), nil
	}
	b, err := Open(path)
	if err != nil {
		return nil, err
	}
	b.pool, b.key = p, key
	p.docs[key] = b.Doc
	return b, nil
}

// Len is how many files are open.
func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.docs)
}
