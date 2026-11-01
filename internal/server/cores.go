package server

import (
	"path/filepath"
	"strings"
)

// Cores is the editing state of the workspaces: a core per workspace root
// (internal/core.Workspaces, handed over by internal/cli). The server decides
// which workspaces there are; the cores keep the files each one has open, also
// while another workspace has the focus (docs/design/gui-protocol.md, 3 章
// 「ワークスペース」). Roots are normalized paths, as in wsInfo.Root.
type Cores interface {
	// Open makes sure root has a core. workspace.focus calls it.
	Open(root string)
	// Files is the absolute paths of the files open in root's core.
	Files(root string) []string
	// Kept is the roots whose cores have any file open, named or not. Their
	// workspaces stay while no shell is in them.
	Kept() []string
	// CloseFiles closes root's files that have no unsaved edits.
	CloseFiles(root string)
	// Close stops root's core when its workspace is gone.
	Close(root string)
}

// SetCores sets where the workspaces' files live. Without it the workspaces
// have no files.
func (s *Server) SetCores(c Cores) {
	s.mu.Lock()
	s.cores = c
	s.mu.Unlock()
}

func (s *Server) coresOf() Cores {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cores
}

// relFiles makes the paths relative to root ("/"-separated) when inside it.
func relFiles(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if r, err := filepath.Rel(root, p); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			p = filepath.ToSlash(r)
		}
		out = append(out, p)
	}
	return out
}
