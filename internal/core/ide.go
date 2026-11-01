package core

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
)

// The part of the Claude Code connection that is the editor's content: the
// tools offered, the selection reported, and the changes waiting for the
// user's decision. The server (ide.Server, internal/ide) is not here: internal/cli
// makes one for the whole program and hands it to the front end, which
// delivers its calls as IDECall Events. See docs/design/gui-protocol.md.

func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

// IDETools lists the tools offered to Claude Code.
func IDETools() []ide.Tool {
	return []ide.Tool{
		{Name: "getCurrentSelection", Description: "Get the text selection (or cursor position) in the active editor.", InputSchema: obj(map[string]any{})},
		{Name: "getLatestSelection", Description: "Get the most recent text selection, even if the editor is not focused.", InputSchema: obj(map[string]any{})},
		{Name: "getOpenEditors", Description: "List the files open in the editor.", InputSchema: obj(map[string]any{})},
		{Name: "getWorkspaceFolders", Description: "Get the workspace folders.", InputSchema: obj(map[string]any{})},
		{Name: "getDiagnostics", Description: "Get language diagnostics (LSP) for a file, or for all open files.", InputSchema: obj(map[string]any{
			"uri": str("file:// URI of the file; omit for all files with diagnostics"),
		})},
		{Name: "openFile", Description: "Open a file in the editor, optionally selecting text.", InputSchema: obj(map[string]any{
			"filePath":  str("Path of the file to open"),
			"startText": str("Select from the first line containing this text"),
			"endText":   str("Select up to the end of the first following occurrence of this text"),
		}, "filePath")},
		{Name: "openDiff", Description: "Show a proposed change as a diff and wait for the user to accept or reject it. Accepting writes the file.", InputSchema: obj(map[string]any{
			"old_file_path":     str("Path of the original file"),
			"new_file_path":     str("Path the new contents are written to"),
			"new_file_contents": str("Proposed contents of the file"),
			"tab_name":          str("Name of the diff"),
		}, "old_file_path", "new_file_path", "new_file_contents", "tab_name")},
		{Name: "closeAllDiffTabs", Description: "Reject and close all pending diffs.", InputSchema: obj(map[string]any{})},
		{Name: "close_tab", Description: "Reject and close a pending diff by name.", InputSchema: obj(map[string]any{
			"tab_name": str("Name given to openDiff"),
		}, "tab_name")},
		{Name: "checkDocumentDirty", Description: "Check whether a file has unsaved changes.", InputSchema: obj(map[string]any{
			"filePath": str("Path of the file"),
		}, "filePath")},
		{Name: "saveDocument", Description: "Save a file that has unsaved changes.", InputSchema: obj(map[string]any{
			"filePath": str("Path of the file"),
		}, "filePath")},
	}
}

// IDEReply is the result of an IDE tool call.
type IDEReply struct {
	Content []ide.Content
	Err     error
}

// IDEPos, IDERange and Selection are the editor state reported by
// getCurrentSelection and the selection_changed notification.
type IDEPos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type IDERange struct {
	Start   IDEPos `json:"start"`
	End     IDEPos `json:"end"`
	IsEmpty bool   `json:"isEmpty"`
}

type Selection struct {
	Text      string   `json:"text"`
	FilePath  string   `json:"filePath"`
	FileURL   string   `json:"fileUrl"`
	Selection IDERange `json:"selection"`
}

// FileURL is the file:// URI of an absolute path.
func FileURL(p string) string { return (&url.URL{Scheme: "file", Path: p}).String() }

// SelectionOf describes the selection of b (or its cursor, when nothing is
// selected); ok is false when b has no file name.
func SelectionOf(b *buffer.Buffer) (Selection, bool) {
	if b == nil || b.Path == "" {
		return Selection{}, false
	}
	s, e, ok := b.Selection()
	if !ok {
		s, e = b.Cur, b.Cur
	}
	sel := Selection{
		FilePath: b.Path,
		FileURL:  FileURL(b.Path),
		Selection: IDERange{
			Start:   IDEPos{s.Row, s.Col},
			End:     IDEPos{e.Row, e.Col},
			IsEmpty: !ok,
		},
	}
	if ok {
		sel.Text = b.SelectedText()
	}
	return sel, true
}

// SelectionTracker remembers the latest selection seen (getLatestSelection
// answers with it when no editor is active) and the one last pushed to the
// client, so that an unchanged selection is not pushed twice.
type SelectionTracker struct{ last, sent *Selection }

// Last is the latest selection seen, or nil.
func (t *SelectionTracker) Last() *Selection { return t.last }

// Observe records sel as the latest selection and reports whether it differs
// from the one last pushed, in which case it counts as pushed.
func (t *SelectionTracker) Observe(sel Selection) (push bool) {
	t.last = &sel
	if t.sent != nil && t.sent.FilePath == sel.FilePath && t.sent.Selection == sel.Selection {
		return false
	}
	cp := sel
	t.sent = &cp
	return true
}

// MentionRange is the lines an at_mentioned notification names for sel: a
// selection that ends at the start of a line does not include that line.
func (s Selection) MentionRange() (start, end int) {
	end = s.Selection.End.Line
	if !s.Selection.IsEmpty && s.Selection.End.Character == 0 && end > s.Selection.Start.Line {
		end--
	}
	return s.Selection.Start.Line, end
}

// SelectText selects from the first line containing start up to the end of
// the first following occurrence of end (or just the start match).
func SelectText(b *buffer.Buffer, start, end string) {
	find := func(from buffer.Pos, s string) (buffer.Pos, bool) {
		for r := from.Row; r < len(b.Lines); r++ {
			line := b.Lines[r]
			off := 0
			if r == from.Row {
				off = min(from.Col, len(line))
			}
			if i := strings.Index(string(line[off:]), s); i >= 0 {
				return buffer.Pos{Row: r, Col: off + len([]rune(string(line[off:])[:i]))}, true
			}
		}
		return buffer.Pos{}, false
	}
	s, found := find(buffer.Pos{}, start)
	if !found {
		return
	}
	e := buffer.Pos{Row: s.Row, Col: s.Col + len([]rune(start))}
	if end != "" {
		if p, found := find(s, end); found {
			e = buffer.Pos{Row: p.Row, Col: p.Col + len([]rune(end))}
		}
	}
	b.SelectRange(s, e)
}

// diffContext is how many unchanged lines surround each change.
const diffContext = 3

// PropLine is a line of a proposal's diff.
type PropLine struct {
	Kind byte // ' ', '+', '-' or '@'
	Text string
}

// Proposal is a change the agent asked to apply with openDiff. Accepting
// answers FILE_SAVED; Claude Code then writes the file itself, as it does for
// the VS Code extension, and the file watcher picks the change up.
type Proposal struct {
	Name             string
	OldPath, NewPath string
	NewText          string
	Lines            []PropLine
	reply            chan<- IDEReply
}

// NewProposal makes the proposal of newText for newPath, whose old contents are
// oldText. reply gets the answer when the user decides.
func NewProposal(name, oldPath, newPath, oldText, newText string, reply chan<- IDEReply) *Proposal {
	return &Proposal{
		Name: name, OldPath: oldPath, NewPath: newPath, NewText: newText,
		Lines: UnifiedDiff(oldText, newText), reply: reply,
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// UnifiedDiff renders the change from oldText to newText as hunks.
func UnifiedDiff(oldText, newText string) []PropLine {
	old, cur := splitLines(oldText), splitLines(newText)
	blocks := diff.Lines(old, cur)
	var out []PropLine
	for i := 0; i < len(blocks); {
		// group blocks whose context windows touch
		j := i
		for j+1 < len(blocks) && blocks[j+1].OldStart-blocks[j].OldEnd <= 2*diffContext {
			j++
		}
		first, last := blocks[i], blocks[j]
		oStart := max(0, first.OldStart-diffContext)
		oEnd := min(len(old), last.OldEnd+diffContext)
		nStart := first.NewStart - (first.OldStart - oStart)
		nEnd := last.NewEnd + (oEnd - last.OldEnd)
		out = append(out, PropLine{'@', fmt.Sprintf("@@ -%d,%d +%d,%d @@", oStart+1, oEnd-oStart, nStart+1, nEnd-nStart)})
		o := oStart
		for _, b := range blocks[i : j+1] {
			for ; o < b.OldStart; o++ {
				out = append(out, PropLine{' ', old[o]})
			}
			for ; o < b.OldEnd; o++ {
				out = append(out, PropLine{'-', old[o]})
			}
			for n := b.NewStart; n < b.NewEnd; n++ {
				out = append(out, PropLine{'+', cur[n]})
			}
		}
		for ; o < oEnd; o++ {
			out = append(out, PropLine{' ', old[o]})
		}
		i = j + 1
	}
	if len(out) == 0 {
		out = append(out, PropLine{' ', i18n.T("idep.nochange")})
	}
	return out
}

// Proposals are the changes waiting for the user's decision, oldest first.
// The first is the one the user is asked about.
type Proposals struct{ list []*Proposal }

func (q *Proposals) Len() int { return len(q.list) }

// First is the proposal being asked about, or nil.
func (q *Proposals) First() *Proposal {
	if len(q.list) == 0 {
		return nil
	}
	return q.list[0]
}

func (q *Proposals) Add(p *Proposal) { q.list = append(q.list, p) }

// Resolve answers the agent and drops the proposal. The editor never writes
// the file: that is left to Claude Code after FILE_SAVED.
func (q *Proposals) Resolve(p *Proposal, accept bool) {
	if accept {
		p.reply <- IDEReply{Content: []ide.Content{ide.Text("FILE_SAVED"), ide.Text(p.NewText)}}
	} else {
		p.reply <- IDEReply{Content: []ide.Content{ide.Text("DIFF_REJECTED"), ide.Text(p.Name)}}
	}
	q.drop(p)
}

// Close drops a proposal the agent closed itself (close_tab, closeAllDiffTabs),
// typically after the decision was made on the Claude Code side. The pending
// request is still answered, but the editor rejected nothing.
func (q *Proposals) Close(p *Proposal) {
	p.reply <- IDEReply{Content: []ide.Content{ide.Text("DIFF_REJECTED"), ide.Text(p.Name)}}
	q.drop(p)
}

// Named is the proposals called name, in order.
func (q *Proposals) Named(name string) []*Proposal {
	var out []*Proposal
	for _, p := range q.list {
		if p.Name == name {
			out = append(out, p)
		}
	}
	return out
}

// All is a copy of the waiting proposals.
func (q *Proposals) All() []*Proposal { return slices.Clone(q.list) }

// Cancel drops the proposal whose client disconnected.
func (q *Proposals) Cancel(reply chan<- IDEReply) {
	q.list = slices.DeleteFunc(q.list, func(p *Proposal) bool { return p.reply == reply })
}

func (q *Proposals) drop(p *Proposal) {
	q.list = slices.DeleteFunc(q.list, func(x *Proposal) bool { return x == p })
}
