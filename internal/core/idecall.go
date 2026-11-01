package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"


	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
)


// SetIDE attaches the MCP server internal/cli made for the program, so the
// editor can push events. The server is not the core's: a GUI server makes its own.
func (a *Core) SetIDE(s *ide.Server) { a.ide = s }

func (a *Core) ideAbs(p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(a.root, p)
	}
	return filepath.Clean(p)
}

// ideFile resolves a path or file:// URI argument.
func (a *Core) ideFile(s string) string {
	if strings.HasPrefix(s, "file://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Path
		}
	}
	return a.ideAbs(s)
}

// currentSelection describes the active text tab's selection; ok is false
// when no named file is open.
func (a *Core) currentSelection() (Selection, bool) {
	t := a.curText()
	if t == nil {
		return Selection{}, false
	}
	return SelectionOf(t.buf)
}

// notifySelection pushes selection_changed when the cursor or selection of
// the active file moved since the last push.
func (a *Core) notifySelection() {
	if a.ide == nil || !a.ide.Connected() {
		return
	}
	sel, ok := a.currentSelection()
	if !ok {
		return
	}
	if a.selTrack.Observe(sel) {
		a.ide.Notify("selection_changed", sel)
	}
}

// mentionContext tells the connected agent about the current selection (or
// cursor line) so it can refer to it; it returns false when no agent is
// connected.
func (a *Core) mentionContext() bool {
	if a.ide == nil || !a.ide.Connected() {
		return false
	}
	sel, ok := a.currentSelection()
	if !ok {
		return false
	}
	start, end := sel.MentionRange()
	a.ide.Notify("at_mentioned", map[string]any{
		"filePath":  sel.FilePath,
		"lineStart": start,
		"lineEnd":   end,
	})
	a.info(i18n.T("ide.sent"), a.rel(sel.FilePath))
	return true
}

func (a *Core) tabFor(path string) *tab {
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
func (a *Core) handleIDE(m IDECall) []Effect {
	res, cmd, pending := a.ideCall(m)
	var effs []Effect
	if !pending {
		effs = append(effs, ReplyIDE{To: m.Reply, Reply: res})
	}
	return append(effs, cmd...)
}

// ideCall runs a tool. pending is true when the reply is deferred.
func (a *Core) ideCall(m IDECall) (res IDEReply, cmd []Effect, pending bool) {
	ok := func(v any) (IDEReply, []Effect, bool) {
		return IDEReply{Content: []ide.Content{ide.JSON(v)}}, nil, false
	}
	fail := func(format string, args ...any) (IDEReply, []Effect, bool) {
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
		if last := a.selTrack.Last(); m.Tool == "getLatestSelection" && last != nil {
			return ok(map[string]any{"success": true, "text": last.Text, "filePath": last.FilePath,
				"fileUrl": last.FileURL, "selection": last.Selection})
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
				"uri": FileURL(t.buf.Path), "isActive": i == a.active, "label": filepath.Base(t.buf.Path),
				"languageId": lang, "isDirty": t.dirty(),
			})
		}
		return ok(map[string]any{"tabs": tabs})
	case "getWorkspaceFolders":
		return ok(map[string]any{"success": true, "rootPath": a.root, "folders": []map[string]any{
			{"name": filepath.Base(a.root), "uri": FileURL(a.root), "path": a.root},
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
						"start": IDEPos{Line: d.Range.Start.Line, Character: d.Range.Start.Character},
						"end":   IDEPos{Line: d.Range.End.Line, Character: d.Range.End.Character},
					},
				})
			}
			if len(ds) > 0 || args.URI != "" {
				out = append(out, map[string]any{"uri": FileURL(p), "diagnostics": ds})
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
			SelectText(t.buf, args.StartText, args.EndText)
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
		a.props.Add(NewProposal(args.TabName, oldPath, newPath, string(old), args.NewText, m.Reply))
		return IDEReply{}, nil, true
	case "closeAllDiffTabs":
		n := a.props.Len()
		for _, p := range a.props.All() {
			a.closeProposal(p)
		}
		return IDEReply{Content: []ide.Content{ide.Text(fmt.Sprintf("CLOSED_%d_DIFF_TABS", n))}}, nil, false
	case "close_tab":
		if named := a.props.Named(args.TabName); len(named) > 0 {
			a.closeProposal(named[0])
			return IDEReply{Content: []ide.Content{ide.Text("TAB_CLOSED")}}, nil, false
		}
		// Claude Code calls close_tab to clean up after a decision, when the
		// diff is already gone.
		return IDEReply{Content: []ide.Content{ide.Text("TAB_CLOSED")}}, nil, false
	}
	return fail(i18n.T("ide.unknowntool"), m.Tool)
}
