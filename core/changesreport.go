package core

import (
	"path/filepath"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// ChangesReport is the CHANGES list for the GUI that shows it in its sidebar
// (SetChangesInGUI): non-nil while the diff mode against HEAD is on, nil
// otherwise (nothing to tell, or the shj shows the list itself). loading is
// At most ctl.MaxDiffFiles files are told; the rest is cut.
// true while the list is being read after the diff mode was opened; there is
// nothing to tell yet.
func (a *Core) ChangesReport() (d *ctl.PrimaryDiff, loading bool) {
	g := &a.Gd
	if !a.changesInGUI || !g.On || g.Base != BaseHead {
		return nil, false
	}
	if g.Loading {
		return nil, true
	}
	top := a.GitTopDir()
	d = &ctl.PrimaryDiff{Files: make([]ctl.DiffFile, 0, min(len(g.Files), ctl.MaxDiffFiles))}
	for _, f := range g.Files[:min(len(g.Files), ctl.MaxDiffFiles)] {
		d.Files = append(d.Files, ctl.DiffFile{
			Path:    filepath.Join(top, f.Path()),
			Status:  f.Status,
			Added:   f.Added,
			Deleted: f.Deleted,
		})
	}
	if t := a.CurText(); t != nil && a.DiffFile(t) != nil {
		d.Selected = t.Buf.Path
	}
	return d, false
}
