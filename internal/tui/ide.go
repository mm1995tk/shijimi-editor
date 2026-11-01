package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
)

// IDEHandler adapts the MCP server to a running program: each call is delivered
// to Update as an IDECall and the reply is awaited. Most calls are answered at
// once; openDiff is answered when the user decides.
func IDEHandler(p *tea.Program) ide.Handler {
	return func(ctx context.Context, tool string, args json.RawMessage) ([]ide.Content, error) {
		reply := make(chan IDEReply, 1)
		p.Send(IDECall{Tool: tool, Args: args, Reply: reply})
		select {
		case r := <-reply:
			return r.Content, r.Err
		case <-ctx.Done():
			p.Send(IDECancel{Reply: reply})
			return nil, ctx.Err()
		}
	}
}

// SetIDE attaches the running MCP server so the editor can push events.
func (a *App) SetIDE(s *ide.Server) { a.ide = s }

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

type idePos struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type ideRange struct {
	Start   idePos `json:"start"`
	End     idePos `json:"end"`
	IsEmpty bool   `json:"isEmpty"`
}

// ideSelection is the editor state reported by getCurrentSelection and the
// selection_changed notification.
type ideSelection struct {
	Text      string   `json:"text"`
	FilePath  string   `json:"filePath"`
	FileURL   string   `json:"fileUrl"`
	Selection ideRange `json:"selection"`
}

func fileURL(p string) string { return (&url.URL{Scheme: "file", Path: p}).String() }

func (a *App) ideAbs(p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(a.root, p)
	}
	return filepath.Clean(p)
}

// ideFile resolves a path or file:// URI argument.
func (a *App) ideFile(s string) string {
	if strings.HasPrefix(s, "file://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Path
		}
	}
	return a.ideAbs(s)
}

// currentSelection describes the active text tab's selection; ok is false
// when no named file is open.
func (a *App) currentSelection() (ideSelection, bool) {
	t := a.curText()
	if t == nil || t.buf.Path == "" {
		return ideSelection{}, false
	}
	b := t.buf
	s, e, ok := b.Selection()
	if !ok {
		s, e = b.Cur, b.Cur
	}
	sel := ideSelection{
		FilePath: b.Path,
		FileURL:  fileURL(b.Path),
		Selection: ideRange{
			Start:   idePos{s.Row, s.Col},
			End:     idePos{e.Row, e.Col},
			IsEmpty: !ok,
		},
	}
	if ok {
		sel.Text = b.SelectedText()
	}
	return sel, true
}

// notifySelection pushes selection_changed when the cursor or selection of
// the active file moved since the last push.
func (a *App) notifySelection() {
	if a.ide == nil || !a.ide.Connected() {
		return
	}
	sel, ok := a.currentSelection()
	if !ok {
		return
	}
	a.lastSel = &sel
	if a.sentSel != nil && a.sentSel.FilePath == sel.FilePath && a.sentSel.Selection == sel.Selection {
		return
	}
	cp := sel
	a.sentSel = &cp
	a.ide.Notify("selection_changed", sel)
}

// mentionContext tells the connected agent about the current selection (or
// cursor line) so it can refer to it; it returns false when no agent is
// connected.
func (a *App) mentionContext() bool {
	if a.ide == nil || !a.ide.Connected() {
		return false
	}
	sel, ok := a.currentSelection()
	if !ok {
		return false
	}
	end := sel.Selection.End.Line
	if !sel.Selection.IsEmpty && sel.Selection.End.Character == 0 && end > sel.Selection.Start.Line {
		end--
	}
	a.ide.Notify("at_mentioned", map[string]any{
		"filePath":  sel.FilePath,
		"lineStart": sel.Selection.Start.Line,
		"lineEnd":   end,
	})
	a.info(i18n.T("ide.sent"), a.rel(sel.FilePath))
	return true
}

func (a *App) tabFor(path string) *tab {
	for _, t := range a.tabs {
		if t.kind == tabText && t.buf.Path == path {
			return t
		}
	}
	return nil
}

var lspSeverity = map[int]string{1: "Error", 2: "Warning", 3: "Information", 4: "Hint"}

// handleIDE runs an IDE tool call. An answer that is ready goes out as a
// ReplyIDE Effect; openDiff keeps the channel in its proposal and answers when
// the user decides.
func (a *App) handleIDE(m IDECall) []Effect {
	res, cmd, pending := a.ideCall(m)
	var effs []Effect
	if !pending {
		effs = append(effs, ReplyIDE{To: m.Reply, Reply: res})
	}
	return append(effs, cmdEffects(cmd)...)
}

// ideCall runs a tool. pending is true when the reply is deferred.
func (a *App) ideCall(m IDECall) (res IDEReply, cmd tea.Cmd, pending bool) {
	ok := func(v any) (IDEReply, tea.Cmd, bool) {
		return IDEReply{Content: []ide.Content{ide.JSON(v)}}, nil, false
	}
	fail := func(format string, args ...any) (IDEReply, tea.Cmd, bool) {
		return IDEReply{Err: fmt.Errorf(format, args...)}, nil, false
	}
	var args struct {
		URI       string `json:"uri"`
		FilePath  string `json:"filePath"`
		StartText string `json:"startText"`
		EndText   string `json:"endText"`
		OldPath   string `json:"old_file_path"`
		NewPath   string `json:"new_file_path"`
		NewText   string `json:"new_file_contents"`
		TabName   string `json:"tab_name"`
	}
	if len(m.Args) > 0 {
		if err := json.Unmarshal(m.Args, &args); err != nil {
			return fail(i18n.T("ide.badargs"), err)
		}
	}
	switch m.Tool {
	case "getCurrentSelection", "getLatestSelection":
		if sel, found := a.currentSelection(); found {
			return ok(map[string]any{"success": true, "text": sel.Text, "filePath": sel.FilePath,
				"fileUrl": sel.FileURL, "selection": sel.Selection})
		}
		if m.Tool == "getLatestSelection" && a.lastSel != nil {
			return ok(map[string]any{"success": true, "text": a.lastSel.Text, "filePath": a.lastSel.FilePath,
				"fileUrl": a.lastSel.FileURL, "selection": a.lastSel.Selection})
		}
		return ok(map[string]any{"success": false, "message": "No active editor found"})
	case "getOpenEditors":
		tabs := []map[string]any{}
		for i, t := range a.tabs {
			if t.kind != tabText || t.buf.Path == "" {
				continue
			}
			lang := ""
			if t.lang != nil {
				lang = t.lang.ID()
			}
			tabs = append(tabs, map[string]any{
				"uri": fileURL(t.buf.Path), "isActive": i == a.active, "label": filepath.Base(t.buf.Path),
				"languageId": lang, "isDirty": t.dirty(),
			})
		}
		return ok(map[string]any{"tabs": tabs})
	case "getWorkspaceFolders":
		return ok(map[string]any{"success": true, "rootPath": a.root, "folders": []map[string]any{
			{"name": filepath.Base(a.root), "uri": fileURL(a.root), "path": a.root},
		}})
	case "getDiagnostics":
		var paths []string
		if args.URI != "" {
			paths = []string{a.ideFile(args.URI)}
		} else {
			for p := range a.lsp.diags {
				paths = append(paths, p)
			}
			slices.Sort(paths)
		}
		out := []map[string]any{}
		for _, p := range paths {
			ds := []map[string]any{}
			for _, d := range a.lsp.diagnostics(p) {
				ds = append(ds, map[string]any{
					"message": d.Message, "severity": lspSeverity[sevRank(d.Severity)], "source": d.Source,
					"range": map[string]any{
						"start": idePos{d.Range.Start.Line, d.Range.Start.Character},
						"end":   idePos{d.Range.End.Line, d.Range.End.Character},
					},
				})
			}
			if len(ds) > 0 || args.URI != "" {
				out = append(out, map[string]any{"uri": fileURL(p), "diagnostics": ds})
			}
		}
		return ok(out)
	case "openFile":
		if args.FilePath == "" {
			return fail("%s", i18n.T("ide.nofilepath"))
		}
		p := a.ideAbs(args.FilePath)
		if _, err := os.Stat(p); err != nil {
			return fail(i18n.T("ide.nofile"), p)
		}
		if !a.openFile(p) {
			return fail(i18n.T("ide.cantopen"), p)
		}
		a.follow = true
		if t := a.curText(); t != nil && args.StartText != "" {
			selectText(t.buf, args.StartText, args.EndText)
		}
		return IDEReply{Content: []ide.Content{ide.Text("Opened file: " + p)}}, nil, false
	case "checkDocumentDirty":
		p := a.ideAbs(args.FilePath)
		t := a.tabFor(p)
		if t == nil {
			return ok(map[string]any{"success": false, "message": "Document not open: " + p})
		}
		return ok(map[string]any{"success": true, "filePath": p, "isDirty": t.dirty(), "isUntitled": false})
	case "saveDocument":
		p := a.ideAbs(args.FilePath)
		t := a.tabFor(p)
		if t == nil {
			return ok(map[string]any{"success": false, "message": "Document not open: " + p})
		}
		if t.dirty() {
			cmd = a.saveTab(t)
			if a.msgErr {
				return fail(i18n.T("ide.cantsave"), a.msg)
			}
		}
		return IDEReply{Content: []ide.Content{ide.JSON(map[string]any{"success": true, "filePath": p, "saved": true})}}, cmd, false
	case "openDiff":
		if args.NewPath == "" || args.OldPath == "" {
			return fail("%s", i18n.T("ide.needpaths"))
		}
		oldPath, newPath := a.ideAbs(args.OldPath), a.ideAbs(args.NewPath)
		old, _ := os.ReadFile(oldPath)
		a.props = append(a.props, newProposal(args.TabName, oldPath, newPath, string(old), args.NewText, m.Reply))
		return IDEReply{}, nil, true
	case "closeAllDiffTabs":
		n := len(a.props)
		for _, p := range slices.Clone(a.props) {
			a.closeProposal(p)
		}
		return IDEReply{Content: []ide.Content{ide.Text(fmt.Sprintf("CLOSED_%d_DIFF_TABS", n))}}, nil, false
	case "close_tab":
		for _, p := range slices.Clone(a.props) {
			if p.name == args.TabName {
				a.closeProposal(p)
				return IDEReply{Content: []ide.Content{ide.Text("TAB_CLOSED")}}, nil, false
			}
		}
		// Claude Code calls close_tab to clean up after a decision, when the
		// diff is already gone.
		return IDEReply{Content: []ide.Content{ide.Text("TAB_CLOSED")}}, nil, false
	}
	return fail(i18n.T("ide.unknowntool"), m.Tool)
}

// selectText selects from the first line containing start up to the end of
// the first following occurrence of end (or just the start match).
func selectText(b *buffer.Buffer, start, end string) {
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
