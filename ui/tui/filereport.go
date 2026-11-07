package tui

import (
	"reflect"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// SetFileReporter sets what is told the state of this shj: the file of the
// focused tab (the absolute path of a text or an image, also while a diff or a
// preview of it is shown, or "" when there is no tab) and, when the GUI shows
// the CHANGES of the diff mode (SetChangesInGUI), the list of changes. It is
// called once at the start and then each time that changes. The GUI's file
// tree lights the row of the file and its sidebar lists the changes (the shj in
// the primary terminal of a GUI workspace reports it to kisui serve).
func (a *App) SetFileReporter(f func(ctl.PrimaryFile)) {
	a.fileReport = f
	a.reportedFile, a.reported = ctl.PrimaryFile{}, false
}

// reportFile tells the reporter the state when it is not the one it was last
// told. While the changes are loading it waits, so that the GUI does not
// show an empty list for a moment.
func (a *App) reportFile() {
	if a.fileReport == nil {
		return
	}
	pf := ctl.PrimaryFile{}
	if t := a.Cur(); t != nil {
		pf.File = t.Path()
	}
	d, loading := a.ChangesReport()
	if loading {
		return
	}
	pf.Diff = d
	if a.reported && reflect.DeepEqual(pf, a.reportedFile) {
		return
	}
	a.reported, a.reportedFile = true, pf
	a.fileReport(pf)
}
