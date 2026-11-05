package core

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/core/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// Command is an entry of the command palette: its name, its key and what it
// does.
type Command struct {
	Name msg.Msg
	Key  msg.Msg
	run  func(a *Core) []Effect
}

func defaultCommands() []Command {
	return []Command{
		{msg.T("cmd.save"), msg.S("Ctrl+S / :w"), func(a *Core) []Effect { return a.save() }},
		{msg.T("cmd.saveas"), msg.S(""), func(a *Core) []Effect { return a.saveAs() }},
		{msg.T("cmd.saveall"), msg.S(":wa"), func(a *Core) []Effect { return a.saveAll() }},
		{msg.T("cmd.newfile"), msg.S("Ctrl+N"), func(a *Core) []Effect { return a.newFile() }},
		{msg.T("cmd.openfile"), msg.S("Ctrl+P / Space f"), func(a *Core) []Effect { return a.Pal.Show(a.Root, "") }},
		{msg.T("cmd.copyrel"), msg.T("cmd.copyrel.key"), func(a *Core) []Effect { a.copyPath(a.activePath(), false); return nil }},
		{msg.T("cmd.copyabs"), msg.T("cmd.copyabs.key"), func(a *Core) []Effect { a.copyPath(a.activePath(), true); return nil }},
		{msg.T("cmd.reload"), msg.S(":reload"), func(a *Core) []Effect { return a.ExecCommand("reload") }},
		{msg.T("cmd.closetab"), msg.S("Ctrl+W / :bc"), func(a *Core) []Effect { return a.CloseTab(false) }},
		{msg.T("cmd.buffers"), msg.S("Space b"), func(a *Core) []Effect { return a.BufferPicker() }},
		{msg.T("cmd.sidebar"), msg.S("Ctrl+B"), func(a *Core) []Effect { a.ToggleSide(); return nil }},
		{msg.T("cmd.focustree"), msg.S("Ctrl+E / Space e"), func(a *Core) []Effect { a.SetSide(true); return a.SetFocus(FocusExplorer) }},
		{msg.T("cmd.preview"), msg.S("Space m / :preview"), func(a *Core) []Effect { return a.TogglePreview() }},
		{msg.T("cmd.terminal"), msg.S("Ctrl+`"), func(a *Core) []Effect { return a.ToggleTerm() }},
		{msg.T("cmd.resize"), msg.S("Alt+R"), func(a *Core) []Effect { return a.StartResize() }},
		{msg.T("cmd.resetsizes"), msg.T("cmd.resetsizes.key"), func(a *Core) []Effect { a.ResetPanes(); return nil }},
		{msg.T("cmd.lastfile"), msg.S("ga / Ctrl+Tab"), func(a *Core) []Effect { a.SwitchLast(); return nil }},
		{msg.T("cmd.split"), msg.S("Ctrl+\\ / Space w v / :vsplit"), func(a *Core) []Effect { return a.SplitOpen() }},
		{msg.T("cmd.splitfocus"), msg.S("Ctrl+1 Ctrl+2 / Space w h l w"), func(a *Core) []Effect { return a.focusOtherPane() }},
		{msg.T("cmd.splitclose"), msg.S("Ctrl+\\ / Space w q o / :only"), func(a *Core) []Effect { a.closeSplit(true); return nil }},
		{msg.T("cmd.find"), msg.S("Ctrl+F / /"), func(a *Core) []Effect {
			if a.Helix {
				return a.openCmdline("/")
			}
			return a.OpenFind()
		}},
		{msg.T("cmd.goto"), msg.T("cmd.goto.key"), func(a *Core) []Effect { return a.openCmdline(":") }},
		{msg.T("cmd.back"), msg.S("Ctrl+O / Alt+←"), func(a *Core) []Effect { return a.jumpMove(-1) }},
		{msg.T("cmd.forward"), msg.S("Ctrl+I / Alt+→"), func(a *Core) []Effect { return a.jumpMove(1) }},
		{msg.T("cmd.undo"), msg.S("u / Ctrl+Z"), func(a *Core) []Effect { a.withBuf(func(b *buffer.Buffer) { b.Undo() }); return nil }},
		{msg.T("cmd.redo"), msg.S("U / Ctrl+Y"), func(a *Core) []Effect { a.withBuf(func(b *buffer.Buffer) { b.Redo() }); return nil }},
		{msg.T("cmd.togglecomment"), msg.S("Ctrl+C / Ctrl+/"), func(a *Core) []Effect { a.toggleComment(); return nil }},
		{msg.T("cmd.grep"), msg.S("Space /"), func(a *Core) []Effect { return a.OpenSearch(FQuery) }},
		{msg.T("cmd.sendcontext"), msg.S("Space a / Alt+A"), func(a *Core) []Effect { return a.SendContext() }},
		{msg.T("cmd.agentpanel"), msg.S("Space i / Alt+I / :agent"), func(a *Core) []Effect { return a.toggleAgentPanel() }},
		{msg.T("cmd.agentcancel"), msg.S(":agent cancel"), func(a *Core) []Effect { a.cancelAgent(); return nil }},
		{msg.T("cmd.agentstop"), msg.S(":agent stop"), func(a *Core) []Effect { return a.agentCommand("stop") }},
		{msg.T("cmd.signature"), msg.S(":signature"), func(a *Core) []Effect { return a.signatureHelp(true) }},
		{msg.T("cmd.hover"), msg.S("Space k"), func(a *Core) []Effect { return a.hover() }},
		{msg.T("cmd.definition"), msg.S("gd / F12"), func(a *Core) []Effect { return a.gotoDefinition() }},
		{msg.T("cmd.references"), msg.S("gr / Shift+F12"), func(a *Core) []Effect { return a.references() }},
		{msg.T("cmd.rename"), msg.S("Space r / F2"), func(a *Core) []Effect { return a.rename() }},
		{msg.T("cmd.codeaction"), msg.S("Space . / :action"), func(a *Core) []Effect { return a.codeAction() }},
		{msg.T("cmd.symbols"), msg.S("Space s / :symbol"), func(a *Core) []Effect { return a.documentSymbols() }},
		{msg.T("cmd.wsymbols"), msg.S("Space S / :symbol-all"), func(a *Core) []Effect { return a.workspaceSymbols() }},
		{msg.T("cmd.format"), msg.S(":fmt"), func(a *Core) []Effect { return a.Format(false) }},
		{msg.T("cmd.diagnostics"), msg.S("Space d"), func(a *Core) []Effect { return a.diagnosticPicker() }},
		{msg.T("cmd.lsprestart"), msg.S(":lsp-restart"), func(a *Core) []Effect { return a.ExecCommand("lsp-restart") }},
		{msg.T("cmd.diffhead"), msg.S("Ctrl+G / Space G / :gitdiff"), func(a *Core) []Effect { return a.OpenDiff(BaseHead) }},
		{msg.T("cmd.diffpr"), msg.S("Space g"), func(a *Core) []Effect { return a.OpenDiff(BasePR) }},
		{msg.T("cmd.diffprpick"), msg.T("cmd.diffprpick.key"), func(a *Core) []Effect { return a.prPicker() }},
		{msg.T("cmd.diffsplit"), msg.S(":gitdiff split|unified"), func(a *Core) []Effect { a.ToggleSplit(); return nil }},
		{msg.T("cmd.diffchanges"), msg.S(""), func(a *Core) []Effect { return a.focusChangesList() }},
		{msg.T("cmd.rvcomment"), msg.S("Space v c"), func(a *Core) []Effect { return a.ReviewCommand("c") }},
		{msg.T("cmd.rvfilecomment"), msg.S("Space v C"), func(a *Core) []Effect { return a.ReviewCommand("C") }},
		{msg.T("cmd.rvthread"), msg.S("Space v t"), func(a *Core) []Effect { return a.ReviewCommand("t") }},
		{msg.T("cmd.rvviewed"), msg.S("Space v v"), func(a *Core) []Effect { return a.ReviewCommand("v") }},
		{msg.T("cmd.rvapprove"), msg.S("Space v A"), func(a *Core) []Effect { return a.ReviewCommand("A") }},
		{msg.T("cmd.rvrequest"), msg.S("Space v R"), func(a *Core) []Effect { return a.ReviewCommand("R") }},
		{msg.T("cmd.rvcommentonly"), msg.S("Space v S"), func(a *Core) []Effect { return a.ReviewCommand("S") }},
		{msg.T("cmd.rvcopyurl"), msg.S("Space v y"), func(a *Core) []Effect { return a.ReviewCommand("y") }},
		{msg.T("cmd.rvopenurl"), msg.S("Space v o"), func(a *Core) []Effect { return a.ReviewCommand("o") }},
		{msg.T("cmd.treerefresh"), msg.S("R"), func(a *Core) []Effect { a.Explorer.Refresh(); return nil }},
		{msg.T("cmd.treenewfile"), msg.S("a"), func(a *Core) []Effect { return a.newFileIn(a.Explorer.targetDir()) }},
		{msg.T("cmd.treenewfolder"), msg.S("A"), func(a *Core) []Effect { return a.newFolder() }},
		{msg.T("cmd.treerename"), msg.S("r"), func(a *Core) []Effect { return a.renameFile() }},
		{msg.T("cmd.treedelete"), msg.S("d"), func(a *Core) []Effect { return a.deleteFile() }},
		{msg.T("cmd.openurl"), msg.S("Space o / :open-url"), func(a *Core) []Effect { return a.openURL("") }},
		{msg.T("cmd.browserlist"), msg.S(":browser list"), func(a *Core) []Effect { return a.ExecCommand("browser list") }},
		{msg.T("cmd.theme"), msg.T("cmd.theme.key"), func(a *Core) []Effect { return a.themePicker() }},
		{msg.T("cmd.keymap"), msg.S(":keymap"), func(a *Core) []Effect { return a.ExecCommand("keymap") }},
		{msg.T("cmd.reloadconfig"), msg.S(":config-reload"), func(a *Core) []Effect { return a.ExecCommand("config-reload") }},
		{msg.T("cmd.quit"), msg.S("Ctrl+Q / :q"), func(a *Core) []Effect { return a.Quit() }},
	}
}

// ---- command line ----

func (a *Core) openCmdline(kind string) []Effect {
	a.CmdOpen = true
	a.cmdKind = kind[:1]
	a.cmdInput.SetValue(kind[1:])
	a.cmdInput.CursorEnd()
	if t := a.CurText(); t != nil {
		a.cmdFrom = t.Buf.Cur
	}
	a.cmdInput.Focus()
	return nil
}

func (a *Core) closeCmdline() {
	a.CmdOpen = false
	a.cmdInput.Blur()
}

func (a *Core) handleCmdlineKey(msg Key) []Effect {
	switch msg.String() {
	case "esc", "ctrl+c":
		a.closeCmdline()
		return nil
	case "enter":
		v := a.cmdInput.Value()
		kind := a.cmdKind
		a.closeCmdline()
		if kind == ":" {
			return a.ExecCommand(strings.TrimSpace(v))
		}
		if kind == "s" {
			a.selectMatches(v)
			return nil
		}
		if v != "" {
			a.lastSearch = v
			a.searchBackward = kind == "?"
			if t := a.CurText(); t != nil {
				t.Buf.Cur = a.cmdFrom
				t.Buf.Anchor = nil
			}
			a.pushJump()
			a.searchNext(false)
		}
		return nil
	case "backspace":
		if a.cmdInput.Value() == "" {
			a.closeCmdline()
			return nil
		}
	}
	return a.cmdlineInput(msg)
}

// cmdlineInput passes a key or a paste to the command line.
func (a *Core) cmdlineInput(msg Input) []Effect {
	a.lineInputUpdate(&a.cmdInput, msg)
	return nil
}

// ExecCommand runs a Helix-style ":" command.
func (a *Core) ExecCommand(line string) []Effect {
	if line == "" {
		return nil
	}
	if n, err := strconv.Atoi(line); err == nil {
		a.pushJump()
		a.withBuf(func(b *buffer.Buffer) { b.GotoLine(n) })
		return a.SetFocus(FocusEditor)
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if a.single && singleBlockedCommand(name) {
		a.Fail(msg.Err("cmd.nosubst", name))
		return nil
	}
	switch name {
	case "w", "write", "w!", "write!":
		if arg != "" {
			if t := a.CurText(); t != nil {
				return a.saveTo(t, arg)
			}
			return nil
		}
		return a.save()
	case "wa", "write-all":
		return a.saveAll()
	case "q", "quit", "qa", "quit-all":
		if d := a.dirtyCount(); d > 0 {
			a.Fail(msg.Err("cmd.quitunsaved", d))
			return nil
		}
		return a.exit()
	case "q!", "quit!", "qa!", "quit-all!":
		return a.exit()
	case "wq", "x", "write-quit", "x!", "wq!":
		a.save()
		if a.MsgErr {
			return nil
		}
		if d := a.dirtyCount(); d > 0 && !strings.HasSuffix(name, "!") {
			a.Fail(msg.Err("cmd.moreunsaved", d))
			return nil
		}
		return a.exit()
	case "wqa", "xa", "write-quit-all":
		a.saveAll()
		if a.MsgErr {
			return nil
		}
		return a.exit()
	case "o", "open", "e", "edit":
		if arg == "" {
			return a.Pal.Show(a.Root, "")
		}
		p := arg
		if !filepath.IsAbs(p) {
			p = filepath.Join(a.Root, p)
		}
		a.pushJump()
		if a.OpenFile(p) {
			return a.SetFocus(FocusEditor)
		}
	case "n", "new":
		if a.single {
			return nil
		}
		if arg == "" {
			a.Fail(msg.Err("cmd.newfile.usage"))
			return nil
		}
		return a.createFile(arg)
	case "bc", "bclose", "buffer-close":
		return a.CloseTab(false)
	case "bc!", "bclose!", "buffer-close!":
		return a.CloseTab(true)
	case "fmt", "format":
		return a.Format(false)
	case "signature", "sig":
		return a.signatureHelp(true)
	case "action", "code-action", "ca":
		return a.codeAction()
	case "symbol", "sym":
		return a.documentSymbols()
	case "symbol-all", "sym-all":
		return a.workspaceSymbols()
	case "copy-rel":
		a.copyPath(a.activePath(), false)
	case "copy-abs":
		a.copyPath(a.activePath(), true)
	case "reload", "rl":
		if t := a.CurText(); t != nil && t.Buf.Path != "" {
			if err := t.reload(); err != nil {
				a.Fail(err)
			} else {
				a.InfoText(msg.T("cmd.reloaded"))
			}
		}
	case "sh", "run", "!":
		if arg == "" {
			return a.ToggleTerm()
		}
		if !a.TermInPane() {
			a.SetTerm(true)
			a.SetFocus(FocusTerminal)
		} else if arg == "hide" {
			a.port.ShowTerm(false)
			return nil
		}
		a.port.TermRun(arg)
		return nil
	case "preview", "md":
		return a.TogglePreview()
	case "vsplit", "vs", "split", "sp":
		return a.SplitOpen()
	case "only", "on":
		a.closeSplit(true)
		return nil
	case "gitdiff", "gd", "review":
		return a.gitDiffCommand(arg)
	case "pr":
		if n, err := strconv.Atoi(strings.TrimPrefix(arg, "#")); err == nil {
			return a.findPR(n)
		}
		return a.prPicker()
	case "grep", "rg":
		return a.OpenSearchArgs(arg, false)
	case "browser":
		return a.browser(arg)
	case "agent":
		return a.agentCommand(arg)
	case "replace", "rep":
		return a.OpenSearchArgs(arg, true)
	case "lsp-restart":
		a.Lsp.restart()
		for _, t := range a.Tabs {
			if t.Kind == TabText {
				a.attachLSP(t)
			}
		}
		a.InfoText(msg.T("cmd.lsprestarted"))
	case "config-reload":
		cfg, err := config.Load(a.Root, a.build().Config)
		if err != nil {
			// Unlike at startup the editor is running: keep the settings it has
			// and say what is wrong.
			if ce, ok := errors.AsType[*config.Error](err); ok {
				a.Fail(msg.Err("config.err.reload", ce.Summary()))
			} else {
				a.Fail(err)
			}
			return nil
		}
		a.Cfg = cfg
		a.port.ConfigRead(a.Cfg)
		a.Commands = defaultCommands()
		a.FindInput.Placeholder = msg.T("app.find")
		a.Lsp.cfg = a.Cfg
		a.Explorer.Icons = a.Cfg.Editor.FileIcons
		a.Helix = a.Cfg.Editor.Keymap != "vscode"
		// No theme (or an unknown one) falls back to the default theme, as at startup.
		if name := a.Cfg.Editor.Theme; name == "" || !a.setTheme(name) {
			a.setTheme(DefaultTheme.Name)
			if name != "" {
				a.Fail(msg.Err("cmd.unknowntheme.default", name))
				return nil
			}
		}
		a.Info("cmd.configreloaded", strings.Join(a.Cfg.Sources, ", "))
		a.loadKeymap()
	case "theme":
		if arg == "" {
			return a.themePicker()
		}
		if !a.setTheme(arg) {
			a.Fail(msg.Err("cmd.unknowntheme", arg, strings.Join(themeNames(), ", ")))
			return nil
		}
		a.Info("cmd.themeis", CurTheme.Name)
	case "keymap":
		switch arg {
		case "helix":
			a.Helix = true
		case "vscode":
			a.Helix = false
		default:
			a.Helix = !a.Helix
		}
		a.Mode = ModeNormal
		if a.Helix {
			a.InfoText(msg.T("cmd.keymap.helix"))
		} else {
			a.InfoText(msg.T("cmd.keymap.vscode"))
		}
	case "open-url", "browse":
		return a.openURL(arg)
	case "tree", "explorer":
		a.SetSide(true)
		return a.SetFocus(FocusExplorer)
	default:
		a.Fail(msg.Err("cmd.unknowncmd", name))
	}
	return nil
}

// ---- project search ----

// parseGrepArgs splits `[-x globs] [-i globs] query` (also --exclude=globs /
// --include=globs). Globs are comma separated. Flags add to the [search]
// defaults from config.
func parseGrepArgs(arg string, def config.Search) (q string, include, exclude []string) {
	include = append(include, def.Include...)
	exclude = append(exclude, def.Exclude...)
	rest := strings.TrimSpace(arg)
	for strings.HasPrefix(rest, "-") {
		flag, tail, _ := strings.Cut(rest, " ")
		var val string
		if f, v, ok := strings.Cut(flag, "="); ok {
			flag, val = f, v
			rest = strings.TrimSpace(tail)
		} else {
			val, rest, _ = strings.Cut(strings.TrimSpace(tail), " ")
			rest = strings.TrimSpace(rest)
		}
		switch flag {
		case "-x", "--exclude":
			exclude = append(exclude, splitGlobs(val)...)
		case "-i", "--include":
			include = append(include, splitGlobs(val)...)
		default:
			return strings.TrimSpace(arg), include, exclude
		}
	}
	return rest, include, exclude
}

// splitGlobs splits a comma separated glob list, dropping quotes and blanks.
func splitGlobs(v string) []string {
	var out []string
	for g := range strings.SplitSeq(v, ",") {
		if g = strings.Trim(strings.TrimSpace(g), `'"`); g != "" {
			out = append(out, g)
		}
	}
	return out
}
