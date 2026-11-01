package tui

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
	run  func(a *App) []Effect
}

func defaultCommands() []command {
	return []command{
		{i18n.T("cmd.save"), "Ctrl+S / :w", func(a *App) []Effect { return a.save() }},
		{i18n.T("cmd.saveas"), "", func(a *App) []Effect { return a.saveAs() }},
		{i18n.T("cmd.saveall"), ":wa", func(a *App) []Effect { return a.saveAll() }},
		{i18n.T("cmd.newfile"), "Ctrl+N", func(a *App) []Effect { return a.newFile() }},
		{i18n.T("cmd.openfile"), "Ctrl+P / Space f", func(a *App) []Effect { return a.pal.show(a.root, "") }},
		{i18n.T("cmd.copyrel"), i18n.T("cmd.copyrel.key"), func(a *App) []Effect { a.copyPath(a.activePath(), false); return nil }},
		{i18n.T("cmd.copyabs"), i18n.T("cmd.copyabs.key"), func(a *App) []Effect { a.copyPath(a.activePath(), true); return nil }},
		{i18n.T("cmd.reload"), ":reload", func(a *App) []Effect { return a.execCommand("reload") }},
		{i18n.T("cmd.closetab"), "Ctrl+W / :bc", func(a *App) []Effect { return a.closeTab(false) }},
		{i18n.T("cmd.buffers"), "Space b", func(a *App) []Effect { return a.bufferPicker() }},
		{i18n.T("cmd.sidebar"), "Ctrl+B", func(a *App) []Effect { a.toggleSide(); return nil }},
		{i18n.T("cmd.focustree"), "Ctrl+E / Space e", func(a *App) []Effect { a.showSide = true; return a.setFocus(focusExplorer) }},
		{i18n.T("cmd.preview"), "Space m / :preview", func(a *App) []Effect { return cmdEffects(a.togglePreview()) }},
		{i18n.T("cmd.terminal"), "Ctrl+`", func(a *App) []Effect { return a.toggleTerm() }},
		{i18n.T("cmd.resize"), "Alt+R", func(a *App) []Effect { return a.startResize() }},
		{i18n.T("cmd.resetsizes"), i18n.T("cmd.resetsizes.key"), func(a *App) []Effect { a.resetPanes(); return nil }},
		{i18n.T("cmd.lastfile"), "ga / Ctrl+Tab", func(a *App) []Effect { a.switchLast(); return nil }},
		{i18n.T("cmd.split"), "Ctrl+\\ / Space w v / :vsplit", func(a *App) []Effect { return a.splitOpen() }},
		{i18n.T("cmd.splitfocus"), "Ctrl+1 Ctrl+2 / Space w h l w", func(a *App) []Effect { return a.focusOtherPane() }},
		{i18n.T("cmd.splitclose"), "Ctrl+\\ / Space w q o / :only", func(a *App) []Effect { a.closeSplit(true); return nil }},
		{i18n.T("cmd.find"), "Ctrl+F / /", func(a *App) []Effect {
			if a.helix {
				return a.openCmdline("/")
			}
			return a.openFind()
		}},
		{i18n.T("cmd.goto"), i18n.T("cmd.goto.key"), func(a *App) []Effect { return a.openCmdline(":") }},
		{i18n.T("cmd.back"), "Ctrl+O / Alt+←", func(a *App) []Effect { return a.jumpMove(-1) }},
		{i18n.T("cmd.forward"), "Ctrl+I / Alt+→", func(a *App) []Effect { return a.jumpMove(1) }},
		{i18n.T("cmd.undo"), "u / Ctrl+Z", func(a *App) []Effect { a.withBuf(func(b *buffer.Buffer) { b.Undo() }); return nil }},
		{i18n.T("cmd.redo"), "U / Ctrl+Y", func(a *App) []Effect { a.withBuf(func(b *buffer.Buffer) { b.Redo() }); return nil }},
		{i18n.T("cmd.togglecomment"), "Ctrl+C / Ctrl+/", func(a *App) []Effect { a.toggleComment(); return nil }},
		{i18n.T("cmd.grep"), "Space /", func(a *App) []Effect { return cmdEffects(a.openSearch(fQuery)) }},
		{i18n.T("cmd.sendcontext"), "Space a / Alt+A", func(a *App) []Effect { return cmdEffects(a.sendContext()) }},
		{i18n.T("cmd.agentpanel"), "Space i / Alt+I / :agent", func(a *App) []Effect { return cmdEffects(a.toggleAgentPanel()) }},
		{i18n.T("cmd.agentcancel"), ":agent cancel", func(a *App) []Effect { a.cancelAgent(); return nil }},
		{i18n.T("cmd.agentstop"), ":agent stop", func(a *App) []Effect { return cmdEffects(a.agentCommand("stop")) }},
		{i18n.T("cmd.signature"), ":signature", func(a *App) []Effect { return cmdEffects(a.signatureHelp(true)) }},
		{i18n.T("cmd.hover"), "Space k", func(a *App) []Effect { return cmdEffects(a.hover()) }},
		{i18n.T("cmd.definition"), "gd / F12", func(a *App) []Effect { return cmdEffects(a.gotoDefinition()) }},
		{i18n.T("cmd.references"), "gr / Shift+F12", func(a *App) []Effect { return cmdEffects(a.references()) }},
		{i18n.T("cmd.rename"), "Space r / F2", func(a *App) []Effect { return cmdEffects(a.rename()) }},
		{i18n.T("cmd.codeaction"), "Space . / :action", func(a *App) []Effect { return cmdEffects(a.codeAction()) }},
		{i18n.T("cmd.symbols"), "Space s / :symbol", func(a *App) []Effect { return cmdEffects(a.documentSymbols()) }},
		{i18n.T("cmd.wsymbols"), "Space S / :symbol-all", func(a *App) []Effect { return cmdEffects(a.workspaceSymbols()) }},
		{i18n.T("cmd.format"), ":fmt", func(a *App) []Effect { return cmdEffects(a.format(false)) }},
		{i18n.T("cmd.diagnostics"), "Space d", func(a *App) []Effect { return cmdEffects(a.diagnosticPicker()) }},
		{i18n.T("cmd.lsprestart"), ":lsp-restart", func(a *App) []Effect { return a.execCommand("lsp-restart") }},
		{i18n.T("cmd.diffhead"), "Ctrl+G / Space G / :gitdiff", func(a *App) []Effect { return cmdEffects(a.openDiff(baseHead)) }},
		{i18n.T("cmd.diffpr"), "Space g", func(a *App) []Effect { return cmdEffects(a.openDiff(basePR)) }},
		{i18n.T("cmd.diffprpick"), i18n.T("cmd.diffprpick.key"), func(a *App) []Effect { return cmdEffects(a.prPicker()) }},
		{i18n.T("cmd.diffsplit"), ":gitdiff split|unified", func(a *App) []Effect { a.toggleSplit(); return nil }},
		{i18n.T("cmd.diffchanges"), "", func(a *App) []Effect { return cmdEffects(a.focusChangesList()) }},
		{i18n.T("cmd.rvcomment"), "Space v c", func(a *App) []Effect { return cmdEffects(a.reviewCommand("c")) }},
		{i18n.T("cmd.rvfilecomment"), "Space v C", func(a *App) []Effect { return cmdEffects(a.reviewCommand("C")) }},
		{i18n.T("cmd.rvthread"), "Space v t", func(a *App) []Effect { return cmdEffects(a.reviewCommand("t")) }},
		{i18n.T("cmd.rvviewed"), "Space v v", func(a *App) []Effect { return cmdEffects(a.reviewCommand("v")) }},
		{i18n.T("cmd.rvapprove"), "Space v A", func(a *App) []Effect { return cmdEffects(a.reviewCommand("A")) }},
		{i18n.T("cmd.rvrequest"), "Space v R", func(a *App) []Effect { return cmdEffects(a.reviewCommand("R")) }},
		{i18n.T("cmd.rvcommentonly"), "Space v S", func(a *App) []Effect { return cmdEffects(a.reviewCommand("S")) }},
		{i18n.T("cmd.rvcopyurl"), "Space v y", func(a *App) []Effect { return cmdEffects(a.reviewCommand("y")) }},
		{i18n.T("cmd.rvopenurl"), "Space v o", func(a *App) []Effect { return cmdEffects(a.reviewCommand("o")) }},
		{i18n.T("cmd.treerefresh"), "R", func(a *App) []Effect { a.explorer.refresh(); return nil }},
		{i18n.T("cmd.treenewfile"), "a", func(a *App) []Effect { return a.newFileIn(a.explorer.targetDir()) }},
		{i18n.T("cmd.treenewfolder"), "A", func(a *App) []Effect { return a.newFolder() }},
		{i18n.T("cmd.treerename"), "r", func(a *App) []Effect { return a.renameFile() }},
		{i18n.T("cmd.treedelete"), "d", func(a *App) []Effect { return a.deleteFile() }},
		{i18n.T("cmd.openurl"), "Space o / :open-url", func(a *App) []Effect { return cmdEffects(a.openURL("")) }},
		{i18n.T("cmd.browserlist"), ":browser list", func(a *App) []Effect { return a.execCommand("browser list") }},
		{i18n.T("cmd.theme"), i18n.T("cmd.theme.key"), func(a *App) []Effect { return cmdEffects(a.themePicker()) }},
		{i18n.T("cmd.keymap"), ":keymap", func(a *App) []Effect { return a.execCommand("keymap") }},
		{i18n.T("cmd.reloadconfig"), ":config-reload", func(a *App) []Effect { return a.execCommand("config-reload") }},
		{i18n.T("cmd.quit"), "Ctrl+Q / :q", func(a *App) []Effect { return a.quit() }},
	}
}

// ---- command line ----

func (a *App) openCmdline(kind string) []Effect {
	a.cmdOpen = true
	a.cmdKind = kind[:1]
	a.cmdInput.SetValue(kind[1:])
	a.cmdInput.CursorEnd()
	if t := a.curText(); t != nil {
		a.cmdFrom = t.buf.Cur
	}
	a.cmdInput.Focus()
	return nil
}

func (a *App) closeCmdline() {
	a.cmdOpen = false
	a.cmdInput.Blur()
}

func (a *App) handleCmdlineKey(msg Key) []Effect {
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
			if t := a.curText(); t != nil {
				t.buf.Cur = a.cmdFrom
				t.buf.Anchor = nil
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
func (a *App) cmdlineInput(msg Input) []Effect {
	a.lineInputUpdate(&a.cmdInput, msg)
	return nil
}

func (a *App) renderCmdline() string {
	return drawCmdline(*a.cmdlineState(), a.cmdInput.view(inputStyleOn(cEditorBg)), a.w)
}

// execCommand runs a Helix-style ":" command.
func (a *App) execCommand(line string) []Effect {
	if line == "" {
		return nil
	}
	if n, err := strconv.Atoi(line); err == nil {
		a.pushJump()
		a.withBuf(func(b *buffer.Buffer) { b.GotoLine(n) })
		return a.setFocus(focusEditor)
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if a.single && singleBlockedCommand(name) {
		a.fail(fmt.Errorf(i18n.T("cmd.nosubst"), name))
		return nil
	}
	switch name {
	case "w", "write", "w!", "write!":
		if arg != "" {
			if t := a.curText(); t != nil {
				return a.saveTo(t, arg)
			}
			return nil
		}
		return a.save()
	case "wa", "write-all":
		return a.saveAll()
	case "q", "quit", "qa", "quit-all":
		if d := a.dirtyCount(); d > 0 {
			a.fail(fmt.Errorf(i18n.T("cmd.quitunsaved"), d))
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
			a.fail(fmt.Errorf(i18n.T("cmd.moreunsaved"), d))
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
			return a.setFocus(focusEditor)
		}
	case "n", "new":
		return a.newFile()
	case "bc", "bclose", "buffer-close":
		return a.closeTab(false)
	case "bc!", "bclose!", "buffer-close!":
		return a.closeTab(true)
	case "fmt", "format":
		return cmdEffects(a.format(false))
	case "signature", "sig":
		return cmdEffects(a.signatureHelp(true))
	case "action", "code-action", "ca":
		return cmdEffects(a.codeAction())
	case "symbol", "sym":
		return cmdEffects(a.documentSymbols())
	case "symbol-all", "sym-all":
		return cmdEffects(a.workspaceSymbols())
	case "reload", "rl":
		if t := a.curText(); t != nil && t.buf.Path != "" {
			if err := t.reload(); err != nil {
				a.fail(err)
			} else {
				a.infoText(i18n.T("cmd.reloaded"))
			}
		}
	case "sh", "run", "!":
		if a.muxOn() {
			switch arg {
			case "":
				return cmdEffects(a.muxToggle())
			case "hide":
				return cmdEffects(a.muxResult(a.muxHide()))
			}
			return cmdEffects(a.muxSend(arg, true))
		}
		if arg == "" {
			return a.toggleTerm()
		}
		a.showTerm = true
		cmd := a.setFocus(focusTerminal)
		return batch(cmd, cmdEffects(a.term.run(arg)))
	case "preview", "md":
		return cmdEffects(a.togglePreview())
	case "vsplit", "vs", "split", "sp":
		return a.splitOpen()
	case "only", "on":
		a.closeSplit(true)
		return nil
	case "gitdiff", "gd", "review":
		return cmdEffects(a.gitDiffCommand(arg))
	case "pr":
		if n, err := strconv.Atoi(strings.TrimPrefix(arg, "#")); err == nil {
			return cmdEffects(a.findPR(n))
		}
		return cmdEffects(a.prPicker())
	case "grep", "rg":
		return cmdEffects(a.openSearchArgs(arg, false))
	case "browser":
		return cmdEffects(a.browser(arg))
	case "agent":
		return cmdEffects(a.agentCommand(arg))
	case "replace", "rep":
		return cmdEffects(a.openSearchArgs(arg, true))
	case "lsp-restart":
		a.lsp.restart()
		for _, t := range a.tabs {
			if t.kind == tabText {
				a.attachLSP(t)
			}
		}
		a.infoText(i18n.T("cmd.lsprestarted"))
	case "config-reload":
		a.cfg = config.Load(a.root)
		a.mux.resolved = false // [terminal] multiplexer may have changed
		i18n.SetLang(a.cfg.Editor.UILanguage)
		a.commands = defaultCommands()
		a.findInput.Placeholder = i18n.T("app.find")
		a.lsp.cfg = a.cfg
		a.explorer.icons = a.cfg.Editor.FileIcons
		a.helix = a.cfg.Editor.Keymap != "vscode"
		// No theme (or an unknown one) falls back to the default theme, as at startup.
		if name := a.cfg.Editor.Theme; name == "" || !a.setTheme(name) {
			a.setTheme(defaultTheme.Name)
			if name != "" {
				a.fail(fmt.Errorf(i18n.T("cmd.unknowntheme.default"), name))
				return nil
			}
		}
		a.info(i18n.T("cmd.configreloaded"), strings.Join(a.cfg.Sources, ", "))
		a.loadKeymap()
	case "theme":
		if arg == "" {
			return cmdEffects(a.themePicker())
		}
		if !a.setTheme(arg) {
			a.fail(fmt.Errorf(i18n.T("cmd.unknowntheme"), arg, strings.Join(themeNames(), ", ")))
			return nil
		}
		a.info(i18n.T("cmd.themeis"), curTheme.Name)
	case "keymap":
		switch arg {
		case "helix":
			a.helix = true
		case "vscode":
			a.helix = false
		default:
			a.helix = !a.helix
		}
		a.mode = modeNormal
		if a.helix {
			a.infoText(i18n.T("cmd.keymap.helix"))
		} else {
			a.infoText(i18n.T("cmd.keymap.vscode"))
		}
	case "open-url", "browse":
		return cmdEffects(a.openURL(arg))
	case "tree", "explorer":
		a.showSide = true
		return a.setFocus(focusExplorer)
	default:
		a.fail(fmt.Errorf(i18n.T("cmd.unknowncmd"), name))
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
