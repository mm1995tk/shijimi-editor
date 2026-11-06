package tui

// SetFileReporter sets what is told the file of the focused tab: the
// absolute path of a text or an image (also while a diff or a preview of it is
// shown), or "" when there is no tab. It is called
// once with the file at the start and then each time that changes. The GUI's
// file tree lights the row of that file (the shj in the primary terminal of a
// GUI workspace reports it to kisui serve).
func (a *App) SetFileReporter(f func(file string)) {
	a.fileReport = f
	a.reportedFile, a.reported = "", false
}

// reportFile tells the reporter the shown file when it is not the one it was
// last told.
func (a *App) reportFile() {
	if a.fileReport == nil {
		return
	}
	file := ""
	if t := a.Cur(); t != nil {
		file = t.Path()
	}
	if a.reported && file == a.reportedFile {
		return
	}
	a.reported, a.reportedFile = true, file
	a.fileReport(file)
}
