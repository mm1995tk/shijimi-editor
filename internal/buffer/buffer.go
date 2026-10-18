// Package buffer implements the text model used by the editor: lines of
// runes, a cursor, an optional selection and undo/redo history.
package buffer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
)

const TabWidth = 4

// Pos is a position in the buffer. Col is a rune index, not a screen column.
type Pos struct{ Row, Col int }

// Less reports whether p comes before q.
func (p Pos) Less(q Pos) bool {
	return p.Row < q.Row || (p.Row == q.Row && p.Col < q.Col)
}

type snapshot struct {
	lines [][]rune
	cur   Pos
}

type editKind int

const (
	editOther editKind = iota
	editTyping
	editDeleting
)

// Buffer holds the contents of one open file.
//
// Invariant: a line's rune slice is never mutated in place once stored in
// Lines, so snapshots only need to copy the outer slice.
// lineSelection records the Anchor/Cur pair SelectLine produced, and the
// content version it was made at.
type lineSelection struct {
	anchor, cur Pos
	ver         int
}

type Buffer struct {
	Path  string
	Lines [][]rune
	Dirty bool

	Cur    Pos
	Anchor *Pos // selection anchor; nil when nothing is selected

	// lineSel is the Anchor/Cur pair left by SelectLine; it identifies a
	// selection as "whole lines" only while neither end has moved.
	lineSel *lineSelection

	ScrollRow, ScrollCol int

	// Version increments on every content change (used by highlight caches).
	Version int

	UseTabs    bool
	indentUnit string
	crlf       bool
	stamp      diskStamp // file state at the last Open/Save/Reload
	wantVis    int       // remembered screen column for vertical movement

	undo, redo []snapshot
	lastEdit   editKind

	multi // secondary cursors (see multi.go)

	marks []*Mark // positions that follow edits (see marks.go)
}

// New returns an empty, unnamed buffer.
func New() *Buffer {
	return &Buffer{Lines: [][]rune{{}}}
}

// diskStamp identifies a file's on-disk state well enough to notice that
// another process rewrote it.
type diskStamp struct {
	mod    time.Time
	size   int64
	exists bool
}

func statStamp(path string) diskStamp {
	st, err := os.Stat(path)
	if err != nil {
		return diskStamp{}
	}
	return diskStamp{mod: st.ModTime(), size: st.Size(), exists: true}
}

// DiskChanged reports whether the file was modified on disk since it was last
// read or written by this buffer. A file that no longer exists is not reported.
func (b *Buffer) DiskChanged() bool {
	if b.Path == "" {
		return false
	}
	now := statStamp(b.Path)
	// a vanished file is not a change to follow: reloading would empty the buffer
	return now.exists && now != b.stamp
}

// Open reads a file into a new buffer. A missing file yields an empty buffer
// bound to that path.
func Open(path string) (*Buffer, error) {
	b := New()
	b.Path = path
	b.UseTabs = prefersTabs(path)
	b.stamp = statStamp(path)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	probe := data
	if len(probe) > 8000 {
		probe = probe[:8000]
	}
	if bytes.IndexByte(probe, 0) >= 0 {
		return nil, errors.New("バイナリファイルは開けません")
	}
	text := string(data)
	if strings.Contains(text, "\r\n") {
		b.crlf = true
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	b.Lines = splitLines([]rune(text))
	if !b.UseTabs {
		for _, l := range b.Lines {
			if len(l) > 0 && l[0] == '\t' {
				b.UseTabs = true
				break
			}
		}
	}
	return b, nil
}

func prefersTabs(path string) bool {
	base := filepath.Base(path)
	return filepath.Ext(path) == ".go" || base == "Makefile" || base == "makefile" || base == "go.mod"
}

// Name is the label shown in tabs.
func (b *Buffer) Name() string {
	if b.Path == "" {
		return "Untitled"
	}
	return filepath.Base(b.Path)
}

// Text returns the whole buffer as a string using "\n" line endings.
func (b *Buffer) Text() string {
	var sb strings.Builder
	for i, l := range b.Lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(string(l))
	}
	return sb.String()
}

// Save writes the buffer to its path.
func (b *Buffer) Save() error {
	if b.Path == "" {
		return errors.New("ファイル名がありません")
	}
	text := b.Text()
	if b.crlf {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(b.Path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(b.Path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(b.Path, []byte(text), mode); err != nil {
		return err
	}
	b.Dirty = false
	b.stamp = statStamp(b.Path)
	return nil
}

// ---- history ----

func (b *Buffer) snap() snapshot {
	return snapshot{lines: append([][]rune(nil), b.Lines...), cur: b.Cur}
}

// begin records an undo point. Consecutive edits of the same kind
// (typing, deleting) are grouped into one undo step.
func (b *Buffer) begin(kind editKind) {
	if b.looping {
		// one undo step for an edit applied to every cursor
		if b.batchBegun {
			b.Dirty = true
			b.Version++
			return
		}
		b.batchBegun = true
	} else {
		b.extra = nil // an edit we cannot apply to every cursor
	}
	if kind == editOther || kind != b.lastEdit {
		b.undo = append(b.undo, b.snap())
		if len(b.undo) > 1000 {
			b.undo = b.undo[1:]
		}
	}
	b.redo = nil
	b.lastEdit = kind
	b.Dirty = true
	b.Version++
}

// Undo reverts the last edit group.
func (b *Buffer) Undo() bool {
	if len(b.undo) == 0 {
		return false
	}
	b.redo = append(b.redo, b.snap())
	s := b.undo[len(b.undo)-1]
	b.undo = b.undo[:len(b.undo)-1]
	b.restore(s)
	return true
}

// Redo re-applies the last undone edit group.
func (b *Buffer) Redo() bool {
	if len(b.redo) == 0 {
		return false
	}
	b.undo = append(b.undo, b.snap())
	s := b.redo[len(b.redo)-1]
	b.redo = b.redo[:len(b.redo)-1]
	b.restore(s)
	return true
}

func (b *Buffer) restore(s snapshot) {
	b.Lines = s.lines
	b.Cur = s.cur
	b.Anchor = nil
	b.extra = nil
	b.marks = nil // the old text is gone, so the positions mean nothing
	b.lastEdit = editOther
	b.Dirty = true
	b.Version++
	b.clamp()
}

// ---- selection ----

// Selection returns the ordered selection range and whether one exists.
func (b *Buffer) Selection() (start, end Pos, ok bool) {
	if b.Anchor == nil || *b.Anchor == b.Cur {
		return Pos{}, Pos{}, false
	}
	a, c := *b.Anchor, b.Cur
	if c.Less(a) {
		return c, a, true
	}
	return a, c, true
}

// SelectedText returns the selected text, or "" when nothing is selected.
func (b *Buffer) SelectedText() string {
	s, e, ok := b.Selection()
	if !ok {
		return ""
	}
	return b.textRange(s, e)
}

func (b *Buffer) textRange(s, e Pos) string {
	if s.Row == e.Row {
		return string(b.Lines[s.Row][s.Col:e.Col])
	}
	var sb strings.Builder
	sb.WriteString(string(b.Lines[s.Row][s.Col:]))
	for r := s.Row + 1; r < e.Row; r++ {
		sb.WriteByte('\n')
		sb.WriteString(string(b.Lines[r]))
	}
	sb.WriteByte('\n')
	sb.WriteString(string(b.Lines[e.Row][:e.Col]))
	return sb.String()
}

// SelectAll selects the whole buffer.
func (b *Buffer) SelectAll() {
	b.lineSel = nil
	b.extra = nil
	b.Anchor = &Pos{}
	last := len(b.Lines) - 1
	b.Cur = Pos{last, len(b.Lines[last])}
}

// ClearSelection drops the selection.
func (b *Buffer) ClearSelection() {
	b.Anchor = nil
	b.lineSel = nil
}

// startSelect sets the anchor at the cursor if extend is true and there is
// none yet; clears it otherwise.
func (b *Buffer) startSelect(extend bool) {
	b.lineSel = nil
	if !extend {
		b.Anchor = nil
		return
	}
	if b.Anchor == nil {
		a := b.Cur
		b.Anchor = &a
	}
}

// ---- editing ----

// Insert inserts text at the cursor, replacing any selection.
func (b *Buffer) Insert(text string) {
	kind := editTyping
	if strings.ContainsRune(text, '\n') {
		kind = editOther
	}
	if _, _, ok := b.Selection(); ok {
		kind = editOther
	}
	b.insertWith(kind, text)
}

// InsertBlock inserts text (e.g. a paste) as its own undo step.
func (b *Buffer) InsertBlock(text string) {
	b.insertWith(editOther, text)
}

func (b *Buffer) insertWith(kind editKind, text string) {
	b.begin(kind)
	b.deleteSelection()
	b.insertRaw([]rune(strings.ReplaceAll(text, "\r\n", "\n")))
}

func (b *Buffer) insertRaw(text []rune) {
	start := b.Cur
	defer func() {
		f := func(p Pos) Pos { return shiftInsert(p, start, b.Cur) }
		b.shiftOthers(f)
		b.shiftMarks(f, func(p Pos) bool { return p == start })
	}()
	parts := splitLines(text)
	line := b.Lines[b.Cur.Row]
	before, after := line[:b.Cur.Col], line[b.Cur.Col:]
	if len(parts) == 1 {
		b.Lines[b.Cur.Row] = concat(before, parts[0], after)
		b.Cur.Col += len(parts[0])
	} else {
		n := len(parts)
		repl := make([][]rune, n)
		repl[0] = concat(before, parts[0])
		for i := 1; i < n-1; i++ {
			repl[i] = parts[i]
		}
		repl[n-1] = concat(parts[n-1], after)
		lines := make([][]rune, 0, len(b.Lines)+n-1)
		lines = append(lines, b.Lines[:b.Cur.Row]...)
		lines = append(lines, repl...)
		lines = append(lines, b.Lines[b.Cur.Row+1:]...)
		b.Lines = lines
		b.Cur = Pos{b.Cur.Row + n - 1, len(parts[n-1])}
	}
	b.Anchor = nil
	b.syncWant()
}

// Newline inserts a line break, copying the current line's indentation.
func (b *Buffer) Newline() {
	line := b.Lines[b.Cur.Row]
	indent := leadingSpace(line)
	if len(indent) > b.Cur.Col {
		indent = indent[:b.Cur.Col]
	}
	extra := ""
	if b.Cur.Col > 0 && strings.ContainsRune("{[(:", line[b.Cur.Col-1]) {
		extra = b.IndentUnit()
	}
	b.Insert("\n" + string(indent) + extra)
}

// IndentUnit is the string inserted for one level of indentation.
func (b *Buffer) IndentUnit() string {
	if b.indentUnit != "" {
		return b.indentUnit
	}
	if b.UseTabs {
		return "\t"
	}
	return strings.Repeat(" ", TabWidth)
}

// Indent inserts indentation; with a multi-line selection it indents every
// selected line.
func (b *Buffer) Indent() {
	s, e, ok := b.Selection()
	if !ok || s.Row == e.Row {
		unit := b.IndentUnit()
		if unit == "\t" {
			b.Insert("\t")
			return
		}
		vis := VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)
		b.Insert(strings.Repeat(" ", len(unit)-vis%len(unit)))
		return
	}
	b.begin(editOther)
	unit := []rune(b.IndentUnit())
	for r := s.Row; r <= e.Row; r++ {
		if r == e.Row && e.Col == 0 {
			break
		}
		b.Lines[r] = concat(unit, b.Lines[r])
	}
	b.Anchor = &Pos{s.Row, 0}
	b.Cur = Pos{e.Row, len(b.Lines[e.Row])}
}

// Outdent removes one level of indentation from the current or selected lines.
func (b *Buffer) Outdent() {
	s, e, ok := b.Selection()
	if !ok {
		s, e = b.Cur, b.Cur
	}
	b.begin(editOther)
	for r := s.Row; r <= e.Row; r++ {
		l := b.Lines[r]
		n := 0
		if len(l) > 0 && l[0] == '\t' {
			n = 1
		} else {
			for n < len(l) && n < TabWidth && l[n] == ' ' {
				n++
			}
		}
		b.Lines[r] = concat(l[n:])
		if r == b.Cur.Row {
			b.Cur.Col = max(0, b.Cur.Col-n)
		}
		if b.Anchor != nil && r == b.Anchor.Row {
			b.Anchor.Col = max(0, b.Anchor.Col-n)
		}
	}
}

// Backspace deletes the selection or the rune before the cursor.
func (b *Buffer) Backspace() {
	if _, _, ok := b.Selection(); ok {
		b.begin(editOther)
		b.deleteSelection()
		return
	}
	if b.Cur.Col == 0 && b.Cur.Row == 0 {
		return
	}
	b.begin(editDeleting)
	start := b.Cur
	if start.Col > 0 {
		start.Col--
	} else {
		start.Row--
		start.Col = len(b.Lines[start.Row])
	}
	b.deleteRange(start, b.Cur)
}

// Delete deletes the selection or the rune after the cursor.
func (b *Buffer) Delete() {
	if _, _, ok := b.Selection(); ok {
		b.begin(editOther)
		b.deleteSelection()
		return
	}
	end := b.Cur
	if end.Col < len(b.Lines[end.Row]) {
		end.Col++
	} else if end.Row < len(b.Lines)-1 {
		end = Pos{end.Row + 1, 0}
	} else {
		return
	}
	b.begin(editDeleting)
	b.deleteRange(b.Cur, end)
}

// DeleteSelection removes the selected text, if any.
func (b *Buffer) DeleteSelection() {
	if _, _, ok := b.Selection(); ok {
		b.begin(editOther)
		b.deleteSelection()
	}
}

// CutLine removes the current line and returns it (with trailing newline).
func (b *Buffer) CutLine() string {
	b.begin(editOther)
	r := b.Cur.Row
	text := string(b.Lines[r]) + "\n"
	if len(b.Lines) == 1 {
		b.Lines = [][]rune{{}}
	} else {
		b.Lines = append(b.Lines[:r:r], b.Lines[r+1:]...)
	}
	b.Anchor = nil
	b.clamp()
	return text
}

// MoveLine swaps the current line with the one above (dir=-1) or below (dir=1).
func (b *Buffer) MoveLine(dir int) {
	r := b.Cur.Row
	t := r + dir
	if t < 0 || t >= len(b.Lines) {
		return
	}
	b.begin(editOther)
	b.Lines[r], b.Lines[t] = b.Lines[t], b.Lines[r]
	b.Cur.Row = t
	b.Anchor = nil
}

// ToggleComment adds or removes a line comment prefix on the current or
// selected lines.
func (b *Buffer) ToggleComment(prefix string) {
	if prefix == "" {
		return
	}
	s, e, ok := b.Selection()
	if !ok {
		s, e = b.Cur, b.Cur
	}
	if ok && e.Col == 0 && e.Row > s.Row {
		e.Row--
	}
	p := []rune(prefix + " ")
	allCommented := true
	for r := s.Row; r <= e.Row; r++ {
		t := strings.TrimLeftFunc(string(b.Lines[r]), unicode.IsSpace)
		if t != "" && !strings.HasPrefix(t, prefix) {
			allCommented = false
		}
	}
	b.begin(editOther)
	for r := s.Row; r <= e.Row; r++ {
		l := b.Lines[r]
		ind := len(leadingSpace(l))
		rest := string(l[ind:])
		if rest == "" {
			continue
		}
		if allCommented {
			rest = strings.TrimPrefix(rest, prefix)
			rest = strings.TrimPrefix(rest, " ")
			b.Lines[r] = concat(l[:ind], []rune(rest))
		} else {
			b.Lines[r] = concat(l[:ind], p, l[ind:])
		}
	}
	b.Anchor = nil
	b.clamp()
}

func (b *Buffer) deleteSelection() {
	s, e, ok := b.Selection()
	if !ok {
		return
	}
	b.deleteRange(s, e)
}

func (b *Buffer) deleteRange(s, e Pos) {
	b.shiftOthers(func(p Pos) Pos { return shiftDelete(p, s, e) })
	b.shiftMarks(func(p Pos) Pos { return shiftDelete(p, s, e) }, func(Pos) bool { return false })
	merged := concat(b.Lines[s.Row][:s.Col], b.Lines[e.Row][e.Col:])
	lines := make([][]rune, 0, len(b.Lines)-(e.Row-s.Row))
	lines = append(lines, b.Lines[:s.Row]...)
	lines = append(lines, merged)
	lines = append(lines, b.Lines[e.Row+1:]...)
	b.Lines = lines
	b.Cur = s
	b.Anchor = nil
	b.syncWant()
}

// ---- movement ----

// Move describes a cursor motion.
type Move int

const (
	Left Move = iota
	Right
	Up
	Down
	Home
	End
	WordLeft
	WordRight
	DocStart
	DocEnd
)

// MoveCursor moves the cursor; extend keeps or starts a selection.
func (b *Buffer) MoveCursor(m Move, extend bool) {
	_, _, hadSel := b.Selection()
	if !extend && hadSel && (m == Left || m == Right) {
		s, e, _ := b.Selection()
		if m == Left {
			b.Cur = s
		} else {
			b.Cur = e
		}
		b.Anchor = nil
		b.syncWant()
		b.lastEdit = editOther
		return
	}
	b.startSelect(extend)
	b.lastEdit = editOther
	switch m {
	case Left:
		if b.Cur.Col > 0 {
			b.Cur.Col--
		} else if b.Cur.Row > 0 {
			b.Cur.Row--
			b.Cur.Col = len(b.Lines[b.Cur.Row])
		}
	case Right:
		if b.Cur.Col < len(b.Lines[b.Cur.Row]) {
			b.Cur.Col++
		} else if b.Cur.Row < len(b.Lines)-1 {
			b.Cur = Pos{b.Cur.Row + 1, 0}
		}
	case Up:
		if b.Cur.Row == 0 {
			b.Cur.Col = 0
		} else {
			b.Cur.Row--
			b.Cur.Col = ColFromVisual(b.Lines[b.Cur.Row], b.wantVis)
		}
		return
	case Down:
		if b.Cur.Row == len(b.Lines)-1 {
			b.Cur.Col = len(b.Lines[b.Cur.Row])
		} else {
			b.Cur.Row++
			b.Cur.Col = ColFromVisual(b.Lines[b.Cur.Row], b.wantVis)
		}
		return
	case Home:
		// Smart home: first non-blank, then column 0.
		ind := len(leadingSpace(b.Lines[b.Cur.Row]))
		if b.Cur.Col == ind {
			b.Cur.Col = 0
		} else {
			b.Cur.Col = ind
		}
	case End:
		b.Cur.Col = len(b.Lines[b.Cur.Row])
	case WordLeft:
		b.wordLeft()
	case WordRight:
		b.wordRight()
	case DocStart:
		b.Cur = Pos{}
	case DocEnd:
		last := len(b.Lines) - 1
		b.Cur = Pos{last, len(b.Lines[last])}
	}
	b.syncWant()
}

// MoveLines moves the cursor n lines (negative = up), used for paging.
func (b *Buffer) MoveLines(n int, extend bool) {
	b.startSelect(extend)
	b.lastEdit = editOther
	b.Cur.Row = clampInt(b.Cur.Row+n, 0, len(b.Lines)-1)
	b.Cur.Col = ColFromVisual(b.Lines[b.Cur.Row], b.wantVis)
}

// SetCursor places the cursor at a row and screen column.
func (b *Buffer) SetCursor(row, vis int, extend bool) {
	if !b.looping {
		b.extra = nil
	}
	b.startSelect(extend)
	b.lastEdit = editOther
	b.Cur.Row = clampInt(row, 0, len(b.Lines)-1)
	b.Cur.Col = ColFromVisual(b.Lines[b.Cur.Row], vis)
	b.syncWant()
}

// GotoLine moves the cursor to a 1-based line number.
func (b *Buffer) GotoLine(n int) {
	b.extra = nil
	b.Anchor = nil
	b.Cur = Pos{clampInt(n-1, 0, len(b.Lines)-1), 0}
	b.syncWant()
}

// SelectRange selects [s, e) and puts the cursor at e.
func (b *Buffer) SelectRange(s, e Pos) {
	a := s
	b.Anchor = &a
	b.Cur = e
	b.syncWant()
}

func (b *Buffer) wordLeft() {
	if b.Cur.Col == 0 {
		b.MoveCursor(Left, b.Anchor != nil)
		return
	}
	l := b.Lines[b.Cur.Row]
	c := b.Cur.Col
	for c > 0 && unicode.IsSpace(l[c-1]) {
		c--
	}
	if c > 0 && isWord(l[c-1]) {
		for c > 0 && isWord(l[c-1]) {
			c--
		}
	} else if c > 0 {
		c--
	}
	b.Cur.Col = c
}

func (b *Buffer) wordRight() {
	l := b.Lines[b.Cur.Row]
	if b.Cur.Col == len(l) {
		b.MoveCursor(Right, b.Anchor != nil)
		return
	}
	c := b.Cur.Col
	for c < len(l) && unicode.IsSpace(l[c]) {
		c++
	}
	if c < len(l) && isWord(l[c]) {
		for c < len(l) && isWord(l[c]) {
			c++
		}
	} else if c < len(l) {
		c++
	}
	b.Cur.Col = c
}

func (b *Buffer) syncWant() {
	b.wantVis = VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)
}

func (b *Buffer) clamp() {
	b.Cur.Row = clampInt(b.Cur.Row, 0, len(b.Lines)-1)
	b.Cur.Col = clampInt(b.Cur.Col, 0, len(b.Lines[b.Cur.Row]))
	b.syncWant()
}

// ---- search ----

// Find returns the next match of query at or after from (wrapping around).
// backward searches before from instead.
func (b *Buffer) Find(query string, from Pos, backward bool) (Pos, bool) {
	q := []rune(strings.ToLower(query))
	if len(q) == 0 {
		return Pos{}, false
	}
	n := len(b.Lines)
	for i := 0; i <= n; i++ {
		var r int
		if backward {
			r = ((from.Row-i)%n + n) % n
		} else {
			r = (from.Row + i) % n
		}
		cols := MatchCols(b.Lines[r], q)
		if backward {
			for j := len(cols) - 1; j >= 0; j-- {
				if i > 0 || cols[j] < from.Col {
					return Pos{r, cols[j]}, true
				}
			}
		} else {
			for _, c := range cols {
				if i > 0 || c >= from.Col {
					return Pos{r, c}, true
				}
			}
		}
	}
	return Pos{}, false
}

// MatchCols returns the start columns of case-insensitive matches of q
// (already lower-cased) in line.
func MatchCols(line, q []rune) []int {
	if len(q) == 0 || len(q) > len(line) {
		return nil
	}
	var out []int
	for i := 0; i+len(q) <= len(line); i++ {
		ok := true
		for j, qr := range q {
			if unicode.ToLower(line[i+j]) != qr {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
			i += len(q) - 1
		}
	}
	return out
}

// ---- helpers ----

// RuneWidth is the screen width of r at screen column vis.
func RuneWidth(r rune, vis int) int {
	if r == '\t' {
		return TabWidth - vis%TabWidth
	}
	w := runewidth.RuneWidth(r)
	if w == 0 {
		return 1
	}
	return w
}

// VisualCol converts a rune index into a screen column.
func VisualCol(line []rune, col int) int {
	vis := 0
	for i := 0; i < col && i < len(line); i++ {
		vis += RuneWidth(line[i], vis)
	}
	return vis
}

// ColFromVisual converts a screen column into the nearest rune index.
func ColFromVisual(line []rune, target int) int {
	vis := 0
	for i, r := range line {
		w := RuneWidth(r, vis)
		if vis+w > target {
			if target-vis > w/2 {
				return i + 1
			}
			return i
		}
		vis += w
	}
	return len(line)
}

func splitLines(text []rune) [][]rune {
	var out [][]rune
	start := 0
	for i, r := range text {
		if r == '\n' {
			out = append(out, concat(text[start:i]))
			start = i + 1
		}
	}
	return append(out, concat(text[start:]))
}

// concat always allocates, so stored lines never share backing arrays.
func concat(parts ...[]rune) []rune {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]rune, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func leadingSpace(l []rune) []rune {
	i := 0
	for i < len(l) && (l[i] == ' ' || l[i] == '\t') {
		i++
	}
	return l[:i]
}

// WordRange returns the half-open column range of the word around col. A
// character that is not part of a word is a range of its own.
func WordRange(line []rune, col int) (start, end int) {
	if col < 0 || col >= len(line) {
		return col, col
	}
	if !isWord(line[col]) {
		return col, col + 1
	}
	start, end = col, col+1
	for start > 0 && isWord(line[start-1]) {
		start--
	}
	for end < len(line) && isWord(line[end]) {
		end++
	}
	return start, end
}

func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- modal (Helix-style) helpers ----

// Edit replaces the text between Start and End.
type Edit struct {
	Start, End Pos
	Text       string
}

// ApplyEdits applies non-overlapping edits as one undo step.
func (b *Buffer) ApplyEdits(edits []Edit) {
	if len(edits) == 0 {
		return
	}
	b.begin(editOther)
	sorted := append([]Edit(nil), edits...)
	// apply from the end so earlier positions stay valid
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1].Start.Less(sorted[j].Start); j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	cur := b.Cur
	for _, e := range sorted {
		s, en := b.ClampPos(e.Start), b.ClampPos(e.End)
		if en.Less(s) {
			s, en = en, s
		}
		b.deleteRange(s, en)
		b.insertRaw([]rune(e.Text))
	}
	b.Cur = cur
	b.Anchor = nil
	b.clamp()
}

// ClampPos returns p limited to valid buffer positions.
func (b *Buffer) ClampPos(p Pos) Pos {
	p.Row = clampInt(p.Row, 0, len(b.Lines)-1)
	p.Col = clampInt(p.Col, 0, len(b.Lines[p.Row]))
	return p
}

// WordEnd moves the cursor just past the end of the next word.
func (b *Buffer) WordEnd(extend bool) {
	b.startSelect(extend)
	b.lastEdit = editOther
	l := b.Lines[b.Cur.Row]
	c := b.Cur.Col
	if c >= len(l) && b.Cur.Row < len(b.Lines)-1 {
		b.Cur = Pos{b.Cur.Row + 1, 0}
		l, c = b.Lines[b.Cur.Row], 0
	}
	for c < len(l) && unicode.IsSpace(l[c]) {
		c++
	}
	if c < len(l) && isWord(l[c]) {
		for c < len(l) && isWord(l[c]) {
			c++
		}
	} else {
		for c < len(l) && !isWord(l[c]) && !unicode.IsSpace(l[c]) {
			c++
		}
	}
	b.Cur.Col = c
	b.syncWant()
}

// SelectWord selects from the cursor to the start of the next word (Helix
// "w") or to the start of the previous word (Helix "b").
func (b *Buffer) SelectWord(forward, extend bool) {
	b.lineSel = nil
	if !extend {
		b.Anchor = nil
	}
	if forward {
		b.startSelect(true)
		b.wordRight()
		l := b.Lines[b.Cur.Row]
		for b.Cur.Col < len(l) && unicode.IsSpace(l[b.Cur.Col]) {
			b.Cur.Col++
		}
	} else {
		b.startSelect(true)
		b.wordLeft()
	}
	b.lastEdit = editOther
	b.syncWant()
}

// SelectLine selects the current line (Helix "x"); repeated calls extend
// the selection by one line.
func (b *Buffer) SelectLine() {
	s, e, ok := b.Selection()
	last := len(b.Lines) - 1
	if ok && s.Col == 0 && e.Row > s.Row && e.Col == 0 {
		b.Anchor = &Pos{s.Row, 0}
		b.Cur = b.lineEndPos(e.Row)
	} else if ok && s.Col == 0 && e.Row > s.Row && e == (Pos{last, len(b.Lines[last])}) {
		// already extended to the end of the last line
	} else {
		b.Anchor = &Pos{b.Cur.Row, 0}
		b.Cur = b.lineEndPos(b.Cur.Row)
	}
	b.lineSel = &lineSelection{*b.Anchor, b.Cur, b.Version}
	b.lastEdit = editOther
	b.syncWant()
}

// lineEndPos returns the position just after row's newline (or its end
// for the last line).
func (b *Buffer) lineEndPos(row int) Pos {
	if row+1 < len(b.Lines) {
		return Pos{row + 1, 0}
	}
	return Pos{row, len(b.Lines[row])}
}

// LineDeleteRange returns the range "d" should delete. It is the selection,
// except when the selection was made by SelectLine, covers the last line
// (possibly empty) and a previous line exists: the newline before it is
// included so the line disappears, and widened is true. Falls back to
// SelectionOrChar.
func (b *Buffer) LineDeleteRange() (s, e Pos, widened bool) {
	if b.Anchor != nil && b.lineSel != nil && b.lineSel.ver == b.Version && *b.Anchor == b.lineSel.anchor && b.Cur == b.lineSel.cur {
		s, e = *b.Anchor, b.Cur
		last := len(b.Lines) - 1
		if s.Col == 0 && s.Row > 0 && s.Row <= last && e == (Pos{last, len(b.Lines[last])}) {
			return Pos{s.Row - 1, len(b.Lines[s.Row-1])}, e, true
		}
	}
	s, e = b.SelectionOrChar()
	return s, e, false
}

// SelectionOrChar returns the selection, or the single character under the
// cursor when nothing is selected.
func (b *Buffer) SelectionOrChar() (Pos, Pos) {
	if s, e, ok := b.Selection(); ok {
		return s, e
	}
	s := b.Cur
	e := s
	if e.Col < len(b.Lines[e.Row]) {
		e.Col++
	} else if e.Row+1 < len(b.Lines) {
		e = Pos{e.Row + 1, 0}
	}
	return s, e
}

// DeleteRange deletes [s, e) as one undo step and returns the removed text.
func (b *Buffer) DeleteRange(s, e Pos) string {
	if !s.Less(e) {
		return ""
	}
	text := b.textRange(s, e)
	b.begin(editOther)
	b.deleteRange(s, e)
	return text
}

// TextRange returns the text in [s, e).
func (b *Buffer) TextRange(s, e Pos) string { return b.textRange(s, e) }

// InsertAt inserts text at p as one undo step, leaving the cursor after it.
func (b *Buffer) InsertAt(p Pos, text string) {
	b.Anchor = nil
	b.Cur = b.ClampPos(p)
	b.InsertBlock(text)
}

// OpenLine inserts an empty line below (or above) the cursor, keeping the
// indentation, and moves the cursor onto it.
func (b *Buffer) OpenLine(below bool) {
	row := b.Cur.Row
	indent := string(leadingSpace(b.Lines[row]))
	b.Anchor = nil
	if below {
		b.Cur = Pos{row, len(b.Lines[row])}
		b.InsertBlock("\n" + indent)
	} else {
		b.Cur = Pos{row, 0}
		b.InsertBlock(indent + "\n")
		b.Cur = Pos{row, len([]rune(indent))}
	}
	b.syncWant()
}

// ReplaceChars replaces every character of the selection (or the one under
// the cursor) with r.
func (b *Buffer) ReplaceChars(r rune) {
	s, e := b.SelectionOrChar()
	if !s.Less(e) {
		return
	}
	old := []rune(b.textRange(s, e))
	for i, c := range old {
		if c != '\n' {
			old[i] = r
		}
	}
	anchor := b.Anchor
	b.begin(editOther)
	b.deleteRange(s, e)
	b.insertRaw(old)
	if anchor != nil {
		b.Anchor = &s
	} else {
		b.Cur = s
	}
}

// FindChar moves to the next/previous occurrence of r on the current line.
// till stops one character before it. Returns false when not found.
func (b *Buffer) FindChar(r rune, forward, till, extend bool) bool {
	l := b.Lines[b.Cur.Row]
	c := b.Cur.Col
	if forward {
		for i := c + 1; i < len(l); i++ {
			if l[i] == r {
				b.startSelect(extend)
				if till {
					b.Cur.Col = i
				} else {
					b.Cur.Col = i + 1
				}
				b.syncWant()
				return true
			}
		}
		return false
	}
	for i := c - 1; i >= 0; i-- {
		if l[i] == r {
			b.startSelect(extend)
			if till {
				b.Cur.Col = i + 1
			} else {
				b.Cur.Col = i
			}
			b.syncWant()
			return true
		}
	}
	return false
}

// MatchBracket jumps to the bracket matching the one under the cursor.
func (b *Buffer) MatchBracket(extend bool) {
	pairs := map[rune]rune{'(': ')', '[': ']', '{': '}', ')': '(', ']': '[', '}': '{'}
	l := b.Lines[b.Cur.Row]
	if b.Cur.Col >= len(l) {
		return
	}
	open := l[b.Cur.Col]
	close, ok := pairs[open]
	if !ok {
		return
	}
	forward := strings.ContainsRune("([{", open)
	depth := 0
	p := b.Cur
	for {
		if forward {
			p.Col++
			for p.Row < len(b.Lines) && p.Col >= len(b.Lines[p.Row]) {
				p.Row++
				p.Col = 0
				if p.Row < len(b.Lines) && len(b.Lines[p.Row]) > 0 {
					break
				}
			}
			if p.Row >= len(b.Lines) {
				return
			}
		} else {
			p.Col--
			for p.Col < 0 {
				p.Row--
				if p.Row < 0 {
					return
				}
				p.Col = len(b.Lines[p.Row]) - 1
			}
		}
		if p.Col >= len(b.Lines[p.Row]) {
			continue
		}
		c := b.Lines[p.Row][p.Col]
		if c == open {
			depth++
		} else if c == close {
			if depth == 0 {
				b.startSelect(extend)
				b.Cur = p
				b.syncWant()
				return
			}
			depth--
		}
	}
}

// JoinLines joins the current line (or selected lines) with the next.
func (b *Buffer) JoinLines() {
	s, e, ok := b.Selection()
	if !ok {
		s, e = b.Cur, b.Cur
	}
	last := max(e.Row, s.Row+1)
	if e.Col == 0 && e.Row > s.Row+1 {
		last = e.Row - 1
	}
	if last >= len(b.Lines) {
		return
	}
	b.begin(editOther)
	joined := b.Lines[s.Row]
	for r := s.Row + 1; r <= last; r++ {
		next := []rune(strings.TrimLeft(string(b.Lines[r]), " \t"))
		joined = concat([]rune(strings.TrimRight(string(joined), " \t")), []rune(" "), next)
	}
	lines := make([][]rune, 0, len(b.Lines))
	lines = append(lines, b.Lines[:s.Row]...)
	lines = append(lines, joined)
	lines = append(lines, b.Lines[last+1:]...)
	b.Lines = lines
	b.Anchor = nil
	b.clamp()
}

// SetIndentUnit overrides the indentation inserted by Tab.
func (b *Buffer) SetIndentUnit(unit string) {
	b.indentUnit = unit
	b.UseTabs = unit == "\t"
}

// Reload re-reads the file from disk, discarding changes.
func (b *Buffer) Reload() error {
	nb, err := Open(b.Path)
	if err != nil {
		return err
	}
	b.begin(editOther)
	b.Lines = nb.Lines
	b.crlf = nb.crlf
	b.stamp = nb.stamp
	b.Dirty = false
	b.Anchor = nil
	b.extra = nil
	b.clamp()
	return nil
}
