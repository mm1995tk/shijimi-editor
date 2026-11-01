package core

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

type command struct {
	name string
	key  string
	run  func(a *Core) []Effect
}

func defaultCommands() []command {
	return []command{
		{i18n.T("cmd.save"), "Ctrl+S / :w", func(a *Core) []Effect { return a.save() }},
		{i18n.T("cmd.saveas"), "", func(a *Core) []Effect { return a.saveAs() }},
		{i18n.T("cmd.saveall"), ":wa", func(a *Core) []Effect { return a.saveAll() }},
		{i18n.T("cmd.newfile"), "Ctrl+N", func(a *Core) []Effect { return a.newFile() }},
		{i18n.T("cmd.openfile"), "Ctrl+P / Space f", func(a *Core) []Effect { return a.pal.show(a.root, "") }},
		{i18n.T("cmd.copyrel"), i18n.T("cmd.copyrel.key"), func(a *Core) []Effect { a.copyPath(a.activePath(), false); return nil }},
		{i18n.T("cmd.copyabs"), i18n.T("cmd.copyabs.key"), func(a *Core) []Effect { a.copyPath(a.activePath(), true); return nil }},
		{i18n.T("cmd.reload"), ":reload", func(a *Core) []Effect { return a.execCommand("reload") }},
		{i18n.T("cmd.closetab"), "Ctrl+W / :bc", func(a *Core) []Effect { return a.closeTab(false) }},
		{i18n.T("cmd.buffers"), "Space b", func(a *Core) []Effect { return a.bufferPicker() }},
		{i18n.T("cmd.sidebar"), "Ctrl+B", func(a *Core) []Effect { a.toggleSide(); return nil }},
		{i18n.T("cmd.focustree"), "Ctrl+E / Space e", func(a *Core) []Effect { a.SetSide(true); return a.SetFocus(FocusExplorer) }},
		{i18n.T("cmd.preview"), "Space m / :preview", func(a *Core) []Effect { return a.togglePreview() }},
		{i18n.T("cmd.terminal"), "Ctrl+`", func(a *Core) []Effect { return a.toggleTerm() }},
		{i18n.T("cmd.resize"), "Alt+R", func(a *Core) []Effect { return a.StartResize() }},
		{i18n.T("cmd.resetsizes"), i18n.T("cmd.resetsizes.key"), func(a *Core) []Effect { a.ResetPanes(); return nil }},
		{i18n.T("cmd.lastfile"), "ga / Ctrl+Tab", func(a *Core) []Effect { a.switchLast(); return nil }},
		{i18n.T("cmd.split"), "Ctrl+\\ / Space w v / :vsplit", func(a *Core) []Effect { return a.splitOpen() }},
		{i18n.T("cmd.splitfocus"), "Ctrl+1 Ctrl+2 / Space w h l w", func(a *Core) []Effect { return a.focusOtherPane() }},
		{i18n.T("cmd.splitclose"), "Ctrl+\\ / Space w q o / :only", func(a *Core) []Effect { a.closeSplit(true); return nil }},
		{i18n.T("cmd.find"), "Ctrl+F / /", func(a *Core) []Effect {
			if a.helix {
				return a.openCmdline("/")
			}
			return a.openFind()
		}},
		{i18n.T("cmd.goto"), i18n.T("cmd.goto.key"), func(a *Core) []Effect { return a.openCmdline(":") }},
		{i18n.T("cmd.back"), "Ctrl+O / Alt+←", func(a *Core) []Effect { return a.jumpMove(-1) }},
		{i18n.T("cmd.forward"), "Ctrl+I / Alt+→", func(a *Core) []Effect { return a.jumpMove(1) }},
		{i18n.T("cmd.undo"), "u / Ctrl+Z", func(a *Core) []Effect { a.withBuf(func(b *buffer.Buffer) { b.Undo() }); return nil }},
		{i18n.T("cmd.redo"), "U / Ctrl+Y", func(a *Core) []Effect { a.withBuf(func(b *buffer.Buffer) { b.Redo() }); return nil }},
		{i18n.T("cmd.togglecomment"), "Ctrl+C / Ctrl+/", func(a *Core) []Effect { a.toggleComment(); return nil }},
		{i18n.T("cmd.grep"), "Space /", func(a *Core) []Effect { return a.openSearch(FQuery) }},
		{i18n.T("cmd.sendcontext"), "Space a / Alt+A", func(a *Core) []Effect { return a.sendContext() }},
		{i18n.T("cmd.agentpanel"), "Space i / Alt+I / :agent", func(a *Core) []Effect { return a.toggleAgentPanel() }},
		{i18n.T("cmd.agentcancel"), ":agent cancel", func(a *Core) []Effect { a.cancelAgent(); return nil }},
		{i18n.T("cmd.agentstop"), ":agent stop", func(a *Core) []Effect { return a.agentCommand("stop") }},
		{i18n.T("cmd.signature"), ":signature", func(a *Core) []Effect { return a.signatureHelp(true) }},
		{i18n.T("cmd.hover"), "Space k", func(a *Core) []Effect { return a.hover() }},
		{i18n.T("cmd.definition"), "gd / F12", func(a *Core) []Effect { return a.gotoDefinition() }},
		{i18n.T("cmd.references"), "gr / Shift+F12", func(a *Core) []Effect { return a.references() }},
		{i18n.T("cmd.rename"), "Space r / F2", func(a *Core) []Effect { return a.rename() }},
		{i18n.T("cmd.codeaction"), "Space . / :action", func(a *Core) []Effect { return a.codeAction() }},
		{i18n.T("cmd.symbols"), "Space s / :symbol", func(a *Core) []Effect { return a.documentSymbols() }},
		{i18n.T("cmd.wsymbols"), "Space S / :symbol-all", func(a *Core) []Effect { return a.workspaceSymbols() }},
		{i18n.T("cmd.format"), ":fmt", func(a *Core) []Effect { return a.format(false) }},
		{i18n.T("cmd.diagnostics"), "Space d", func(a *Core) []Effect { return a.diagnosticPicker() }},
		{i18n.T("cmd.lsprestart"), ":lsp-restart", func(a *Core) []Effect { return a.execCommand("lsp-restart") }},
		{i18n.T("cmd.diffhead"), "Ctrl+G / Space G / :gitdiff", func(a *Core) []Effect { return a.openDiff(baseHead) }},
		{i18n.T("cmd.diffpr"), "Space g", func(a *Core) []Effect { return a.openDiff(basePR) }},
		{i18n.T("cmd.diffprpick"), i18n.T("cmd.diffprpick.key"), func(a *Core) []Effect { return a.prPicker() }},
		{i18n.T("cmd.diffsplit"), ":gitdiff split|unified", func(a *Core) []Effect { a.ToggleSplit(); return nil }},
		{i18n.T("cmd.diffchanges"), "", func(a *Core) []Effect { return a.focusChangesList() }},
		{i18n.T("cmd.rvcomment"), "Space v c", func(a *Core) []Effect { return a.reviewCommand("c") }},
		{i18n.T("cmd.rvfilecomment"), "Space v C", func(a *Core) []Effect { return a.reviewCommand("C") }},
		{i18n.T("cmd.rvthread"), "Space v t", func(a *Core) []Effect { return a.reviewCommand("t") }},
		{i18n.T("cmd.rvviewed"), "Space v v", func(a *Core) []Effect { return a.reviewCommand("v") }},
		{i18n.T("cmd.rvapprove"), "Space v A", func(a *Core) []Effect { return a.reviewCommand("A") }},
		{i18n.T("cmd.rvrequest"), "Space v R", func(a *Core) []Effect { return a.reviewCommand("R") }},
		{i18n.T("cmd.rvcommentonly"), "Space v S", func(a *Core) []Effect { return a.reviewCommand("S") }},
		{i18n.T("cmd.rvcopyurl"), "Space v y", func(a *Core) []Effect { return a.reviewCommand("y") }},
		{i18n.T("cmd.rvopenurl"), "Space v o", func(a *Core) []Effect { return a.reviewCommand("o") }},
		{i18n.T("cmd.treerefresh"), "R", func(a *Core) []Effect { a.explorer.refresh(); return nil }},
		{i18n.T("cmd.treenewfile"), "a", func(a *Core) []Effect { return a.newFileIn(a.explorer.targetDir()) }},
		{i18n.T("cmd.treenewfolder"), "A", func(a *Core) []Effect { return a.newFolder() }},
		{i18n.T("cmd.treerename"), "r", func(a *Core) []Effect { return a.renameFile() }},
		{i18n.T("cmd.treedelete"), "d", func(a *Core) []Effect { return a.deleteFile() }},
		{i18n.T("cmd.openurl"), "Space o / :open-url", func(a *Core) []Effect { return a.openURL("") }},
		{i18n.T("cmd.browserlist"), ":browser list", func(a *Core) []Effect { return a.execCommand("browser list") }},
		{i18n.T("cmd.theme"), i18n.T("cmd.theme.key"), func(a *Core) []Effect { return a.themePicker() }},
		{i18n.T("cmd.keymap"), ":keymap", func(a *Core) []Effect { return a.execCommand("keymap") }},
		{i18n.T("cmd.reloadconfig"), ":config-reload", func(a *Core) []Effect { return a.execCommand("config-reload") }},
		{i18n.T("cmd.quit"), "Ctrl+Q / :q", func(a *Core) []Effect { return a.quit() }},
	}
}

// ---- command line ----

func (a *Core) openCmdline(kind string) []Effect {
	a.cmdOpen = true
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
	a.cmdOpen = false
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
			return a.execCommand(strings.TrimSpace(v))
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

// execCommand runs a Helix-style ":" command.
func (a *Core) execCommand(line string) []Effect {
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
		a.Fail(fmt.Errorf(i18n.T("cmd.nosubst"), name))
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
			a.Fail(fmt.Errorf(i18n.T("cmd.quitunsaved"), d))
			return nil
		}
		return a.exit()
	case "q!", "quit!", "qa!", "quit-all!":
		return a.exit()
	case "wq", "x", "write-quit", "x!", "wq!":
		a.save()
		if a.msgErr {
			return nil
		}
		if d := a.dirtyCount(); d > 0 && !strings.HasSuffix(name, "!") {
			a.Fail(fmt.Errorf(i18n.T("cmd.moreunsaved"), d))
			return nil
		}
		return a.exit()
	case "wqa", "xa", "write-quit-all":
		a.saveAll()
		if a.msgErr {
			return nil
		}
		return a.exit()
	case "o", "open", "e", "edit":
		if arg == "" {
			return a.pal.show(a.root, "")
		}
		p := arg
		if !filepath.IsAbs(p) {
			p = filepath.Join(a.root, p)
		}
		a.pushJump()
		if a.openFile(p) {
			return a.SetFocus(FocusEditor)
		}
	case "n", "new":
		return a.newFile()
	case "bc", "bclose", "buffer-close":
		return a.closeTab(false)
	case "bc!", "bclose!", "buffer-close!":
		return a.closeTab(true)
	case "fmt", "format":
		return a.format(false)
	case "signature", "sig":
		return a.signatureHelp(true)
	case "action", "code-action", "ca":
		return a.codeAction()
	case "symbol", "sym":
		return a.documentSymbols()
	case "symbol-all", "sym-all":
		return a.workspaceSymbols()
	case "reload", "rl":
		if t := a.CurText(); t != nil && t.Buf.Path != "" {
			if err := t.reload(); err != nil {
				a.Fail(err)
			} else {
				a.InfoText(i18n.T("cmd.reloaded"))
			}
		}
	case "sh", "run", "!":
		if a.MuxOn() {
			switch arg {
			case "":
				return a.MuxToggle()
			case "hide":
				return a.MuxResult(a.MuxHide())
			}
			return a.MuxSend(arg, true)
		}
		if arg == "" {
			return a.toggleTerm()
		}
		a.SetTerm(true)
		cmd := a.SetFocus(FocusTerminal)
		return Batch(cmd, a.TermRun(arg))
	case "preview", "md":
		return a.togglePreview()
	case "vsplit", "vs", "split", "sp":
		return a.splitOpen()
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
		return a.openSearchArgs(arg, false)
	case "browser":
		return a.browser(arg)
	case "agent":
		return a.agentCommand(arg)
	case "replace", "rep":
		return a.openSearchArgs(arg, true)
	case "lsp-restart":
		a.lsp.restart()
		for _, t := range a.tabs {
			if t.Kind == TabText {
				a.attachLSP(t)
			}
		}
		a.InfoText(i18n.T("cmd.lsprestarted"))
	case "config-reload":
		a.cfg = config.Load(a.root)
		a.MuxReset() // [terminal] multiplexer may have changed
		i18n.SetLang(a.cfg.Editor.UILanguage)
		a.commands = defaultCommands()
		a.findInput.Placeholder = i18n.T("app.find")
		a.lsp.cfg = a.cfg
		a.explorer.icons = a.cfg.Editor.FileIcons
		a.helix = a.cfg.Editor.Keymap != "vscode"
		// No theme (or an unknown one) falls back to the default theme, as at startup.
		if name := a.cfg.Editor.Theme; name == "" || !a.setTheme(name) {
			a.setTheme(DefaultTheme.Name)
			if name != "" {
				a.Fail(fmt.Errorf(i18n.T("cmd.unknowntheme.default"), name))
				return nil
			}
		}
		a.Info(i18n.T("cmd.configreloaded"), strings.Join(a.cfg.Sources, ", "))
		a.loadKeymap()
	case "theme":
		if arg == "" {
			return a.themePicker()
		}
		if !a.setTheme(arg) {
			a.Fail(fmt.Errorf(i18n.T("cmd.unknowntheme"), arg, strings.Join(themeNames(), ", ")))
			return nil
		}
		a.Info(i18n.T("cmd.themeis"), CurTheme.Name)
	case "keymap":
		switch arg {
		case "helix":
			a.helix = true
		case "vscode":
			a.helix = false
		default:
			a.helix = !a.helix
		}
		a.mode = ModeNormal
		if a.helix {
			a.InfoText(i18n.T("cmd.keymap.helix"))
		} else {
			a.InfoText(i18n.T("cmd.keymap.vscode"))
		}
	case "open-url", "browse":
		return a.openURL(arg)
	case "tree", "explorer":
		a.SetSide(true)
		return a.SetFocus(FocusExplorer)
	default:
		a.Fail(fmt.Errorf(i18n.T("cmd.unknowncmd"), name))
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
