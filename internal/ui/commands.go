package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/config"
)

type command struct {
	name string
	key  string
	run  func(a *App) tea.Cmd
}

func defaultCommands() []command {
	return []command{
		{"ファイル: 保存", "Ctrl+S / :w", func(a *App) tea.Cmd { return a.save() }},
		{"ファイル: 名前を付けて保存", "", func(a *App) tea.Cmd { return a.saveAs() }},
		{"ファイル: すべて保存", ":wa", func(a *App) tea.Cmd { return a.saveAll() }},
		{"ファイル: 新しいファイル", "Ctrl+N", func(a *App) tea.Cmd { return a.newFile() }},
		{"ファイル: ファイルを開く", "Ctrl+P / Space f", func(a *App) tea.Cmd { return a.pal.show(a.root, "") }},
		{"ファイル: 相対パスをコピー", "y（ファイルツリー）", func(a *App) tea.Cmd { a.copyPath(a.activePath(), false); return nil }},
		{"ファイル: 絶対パスをコピー", "Y（ファイルツリー）", func(a *App) tea.Cmd { a.copyPath(a.activePath(), true); return nil }},
		{"ファイル: ディスクから再読み込み", ":reload", func(a *App) tea.Cmd { return a.execCommand("reload") }},
		{"表示: タブを閉じる", "Ctrl+W / :bc", func(a *App) tea.Cmd { a.closeTab(false); return nil }},
		{"表示: 開いているバッファ一覧", "Space b", func(a *App) tea.Cmd { return a.bufferPicker() }},
		{"表示: サイドバー（ファイルツリー）の表示切替", "Ctrl+B", func(a *App) tea.Cmd { a.toggleSide(); return nil }},
		{"表示: ファイルツリーへ移動", "Ctrl+E / Space e", func(a *App) tea.Cmd { a.showSide = true; return a.setFocus(focusExplorer) }},
		{"表示: Markdown プレビューの切替（並べて表示 → 全幅 → 閉じる）", "Space m / :preview", func(a *App) tea.Cmd { return a.togglePreview() }},
		{"表示: ターミナルの表示切替", "Ctrl+J", func(a *App) tea.Cmd { return a.toggleTerm() }},
		{"表示: ペインのリサイズ（サイドバー・ターミナル・プレビュー）", "Alt+R", func(a *App) tea.Cmd { return a.startResize() }},
		{"表示: ペインの大きさを初期値に戻す", "リサイズ中に =", func(a *App) tea.Cmd { a.resetPanes(); return nil }},
		{"表示: 次のタブ", "Ctrl+PgDn / gn", func(a *App) tea.Cmd { a.switchTab(1); return nil }},
		{"表示: 前のタブ", "Ctrl+PgUp / gp", func(a *App) tea.Cmd { a.switchTab(-1); return nil }},
		{"編集: 検索", "Ctrl+F / /", func(a *App) tea.Cmd {
			if a.helix {
				return a.openCmdline("/")
			}
			return a.openFind()
		}},
		{"編集: 行へ移動", ":<行番号>", func(a *App) tea.Cmd { return a.openCmdline(":") }},
		{"編集: 元に戻す", "u / Ctrl+Z", func(a *App) tea.Cmd { a.withBuf(func(b *buffer.Buffer) { b.Undo() }); return nil }},
		{"編集: やり直し", "U / Ctrl+Y", func(a *App) tea.Cmd { a.withBuf(func(b *buffer.Buffer) { b.Redo() }); return nil }},
		{"編集: 行コメントの切替", "Ctrl+C / Ctrl+/", func(a *App) tea.Cmd { a.toggleComment(); return nil }},
		{"検索: プロジェクト全体を検索 / 置換", "Space /", func(a *App) tea.Cmd { return a.openSearch(fQuery) }},
		{"エージェント: 現在の文脈をターミナルへ送る", "Space a / Alt+A", func(a *App) tea.Cmd { return a.sendContext() }},
		{"LSP: ホバー", "Space k", func(a *App) tea.Cmd { return a.hover() }},
		{"LSP: 定義へ移動", "gd / F12", func(a *App) tea.Cmd { return a.gotoDefinition() }},
		{"LSP: 参照を表示", "gr / Shift+F12", func(a *App) tea.Cmd { return a.references() }},
		{"LSP: 名前の変更", "Space r / F2", func(a *App) tea.Cmd { return a.rename() }},
		{"LSP: 整形", ":fmt", func(a *App) tea.Cmd { return a.format(false) }},
		{"LSP: 診断の一覧", "Space d", func(a *App) tea.Cmd { return a.diagnosticPicker() }},
		{"LSP: 言語サーバーを再起動", ":lsp-restart", func(a *App) tea.Cmd { return a.execCommand("lsp-restart") }},
		{"レビュー: プルリクエスト一覧", "Space g / :pr", func(a *App) tea.Cmd { return a.prPicker() }},
		{"git-diff: モードの切替", "Ctrl+G / Space G / :gitdiff", func(a *App) tea.Cmd { return a.toggleGitDiff() }},
		{"git-diff: 分割表示 / 統合表示の切替", ":gitdiff split|unified", func(a *App) tea.Cmd { a.toggleSplit(); return nil }},
		{"git-diff: 比較元を HEAD / PR の base で切替", "b / :gitdiff base", func(a *App) tea.Cmd { return a.cycleGitDiffBase() }},
		{"git-diff: 変更一覧へ移動", "", func(a *App) tea.Cmd { return a.focusChangesList() }},
		{"ファイルツリー: 最新の情報に更新", "R", func(a *App) tea.Cmd { a.explorer.refresh(); return nil }},
		{"ファイルツリー: 新しいファイル", "a", func(a *App) tea.Cmd { return a.newFileIn(a.explorer.targetDir()) }},
		{"ファイルツリー: 新しいフォルダー", "A", func(a *App) tea.Cmd { return a.newFolder() }},
		{"ファイルツリー: 名前の変更", "r", func(a *App) tea.Cmd { return a.renameFile() }},
		{"ファイルツリー: 削除", "d", func(a *App) tea.Cmd { return a.deleteFile() }},
		{"表示: Web ページをブラウザタブで開く", ":browser <URL>", func(a *App) tea.Cmd { return a.openCmdline(":browser ") }},
		{"編集: カーソル位置の URL を外部ブラウザで開く", "Space o / :open-url", func(a *App) tea.Cmd { return a.openURL("") }},
		{"ブラウザ: ポート転送の一覧", ":browser list", func(a *App) tea.Cmd { return a.execCommand("browser list") }},
		{"設定: テーマを切り替え", ":theme <名前>", func(a *App) tea.Cmd { return a.themePicker() }},
		{"設定: キー操作を Helix / VS Code で切り替え", ":keymap", func(a *App) tea.Cmd { return a.execCommand("keymap") }},
		{"設定: 設定ファイルを再読み込み", ":config-reload", func(a *App) tea.Cmd { return a.execCommand("config-reload") }},
		{"終了", "Ctrl+Q / :q", func(a *App) tea.Cmd { return a.quit() }},
	}
}

// ---- command line ----

func (a *App) openCmdline(kind string) tea.Cmd {
	a.cmdOpen = true
	a.cmdKind = kind[:1]
	a.cmdInput.SetValue(kind[1:])
	a.cmdInput.CursorEnd()
	if t := a.curText(); t != nil {
		a.cmdFrom = t.buf.Cur
	}
	return a.cmdInput.Focus()
}

func (a *App) closeCmdline() {
	a.cmdOpen = false
	a.cmdInput.Blur()
}

func (a *App) handleCmdlineKey(msg tea.KeyMsg) tea.Cmd {
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
			a.searchNext(false)
		}
		return nil
	case "backspace":
		if a.cmdInput.Value() == "" {
			a.closeCmdline()
			return nil
		}
	}
	var cmd tea.Cmd
	a.cmdInput, cmd = a.cmdInput.Update(msg)
	return cmd
}

func (a *App) renderCmdline() string {
	prefix := a.cmdKind
	a.cmdInput.Width = max(1, a.w-3)
	return stPanel.Render(prefix) + fitStyled(a.cmdInput.View(), a.w-1, stPanel)
}

// execCommand runs a Helix-style ":" command.
func (a *App) execCommand(line string) tea.Cmd {
	if line == "" {
		return nil
	}
	if n, err := strconv.Atoi(line); err == nil {
		a.withBuf(func(b *buffer.Buffer) { b.GotoLine(n) })
		return a.setFocus(focusEditor)
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
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
			a.fail(fmt.Errorf("未保存のファイルが %d 個あります（:q! で破棄して終了、:wq で保存して終了）", d))
			return nil
		}
		return tea.Quit
	case "q!", "quit!", "qa!", "quit-all!":
		return tea.Quit
	case "wq", "x", "write-quit", "x!", "wq!":
		a.save()
		if a.msgErr {
			return nil
		}
		if d := a.dirtyCount(); d > 0 && !strings.HasSuffix(name, "!") {
			a.fail(fmt.Errorf("他にも未保存のファイルが %d 個あります", d))
			return nil
		}
		return tea.Quit
	case "wqa", "xa", "write-quit-all":
		a.saveAll()
		if a.msgErr {
			return nil
		}
		return tea.Quit
	case "o", "open", "e", "edit":
		if arg == "" {
			return a.pal.show(a.root, "")
		}
		p := arg
		if !filepath.IsAbs(p) {
			p = filepath.Join(a.root, p)
		}
		if a.openFile(p) {
			return a.setFocus(focusEditor)
		}
	case "n", "new":
		return a.newFile()
	case "bc", "bclose", "buffer-close":
		a.closeTab(false)
	case "bc!", "bclose!", "buffer-close!":
		a.closeTab(true)
	case "fmt", "format":
		return a.format(false)
	case "reload", "rl":
		if t := a.curText(); t != nil && t.buf.Path != "" {
			if err := t.buf.Reload(); err != nil {
				a.fail(err)
			} else {
				a.info("再読み込みしました")
			}
		}
	case "sh", "run", "!":
		if arg == "" {
			return a.toggleTerm()
		}
		a.showTerm = true
		cmd := a.setFocus(focusTerminal)
		return tea.Batch(cmd, a.term.run(arg))
	case "preview", "md":
		return a.togglePreview()
	case "gitdiff", "gd", "review":
		return a.gitDiffCommand(arg)
	case "pr":
		if n, err := strconv.Atoi(strings.TrimPrefix(arg, "#")); err == nil {
			return a.openPR(n)
		}
		return a.prPicker()
	case "grep", "rg":
		return a.openSearchArgs(arg, false)
	case "browser":
		return a.browser(arg)
	case "replace", "rep":
		return a.openSearchArgs(arg, true)
	case "lsp-restart":
		a.lsp.restart()
		for _, t := range a.tabs {
			if t.kind == tabText {
				a.attachLSP(t)
			}
		}
		a.info("言語サーバーを再起動しました")
	case "config-reload":
		a.cfg = config.Load(a.root)
		a.lsp.cfg = a.cfg
		a.explorer.icons = a.cfg.Editor.FileIcons
		a.helix = a.cfg.Editor.Keymap != "vscode"
		// No theme (or an unknown one) falls back to dark-plus, as at startup.
		if name := a.cfg.Editor.Theme; name == "" || !a.setTheme(name) {
			a.setTheme(darkPlus.Name)
			if name != "" {
				a.fail(fmt.Errorf("不明なテーマ: %s（dark-plus を使います）", name))
				return nil
			}
		}
		a.info("設定を再読み込みしました: %s", strings.Join(a.cfg.Sources, ", "))
	case "theme":
		if arg == "" {
			return a.themePicker()
		}
		if !a.setTheme(arg) {
			a.fail(fmt.Errorf("不明なテーマ: %s（選べるテーマ: %s）", arg, strings.Join(themeNames(), ", ")))
			return nil
		}
		a.info("テーマ: %s", curTheme.Name)
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
			a.info("キー操作: Helix")
		} else {
			a.info("キー操作: VS Code")
		}
	case "open-url", "browse":
		return a.openURL(arg)
	case "tree", "explorer":
		a.showSide = true
		return a.setFocus(focusExplorer)
	default:
		a.fail(fmt.Errorf("不明なコマンド: %s", name))
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

// ---- review ----

func (a *App) openPR(n int) tea.Cmd {
	for i, t := range a.tabs {
		if t.kind == tabReview && t.rev.prNumber == n {
			a.active = i
			a.focus = focusEditor
			return nil
		}
	}
	r := newReview(a.root, n)
	a.addTab(&tab{kind: tabReview, rev: r})
	a.focus = focusEditor
	return r.load()
}

func (a *App) prPicker() tea.Cmd {
	a.info("プルリクエストを取得しています…")
	return listPRs(a.root)
}

func (a *App) showPRList(m prListMsg) tea.Cmd {
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	if len(m.prs) == 0 {
		a.info("オープンなプルリクエストはありません")
		return nil
	}
	items := make([]paletteItem, len(m.prs))
	for i, p := range m.prs {
		draft := ""
		if p.IsDraft {
			draft = " [draft]"
		}
		items[i] = paletteItem{label: fmt.Sprintf("#%d %s%s", p.Number, p.Title, draft), detail: "@" + p.Author.Login + "  " + p.HeadRefName}
	}
	return a.pal.pick("プルリクエスト", items, func(i int) tea.Cmd { return a.openPR(m.prs[i].Number) })
}

func (a *App) handleSubmitted(m reviewSubmittedMsg) tea.Cmd {
	for _, t := range a.tabs {
		if t.kind != tabReview || t.rev.id != m.id {
			continue
		}
		if m.err != nil {
			a.fail(m.err)
			return nil
		}
		var kept []*reviewComment
		for _, c := range t.rev.comments {
			if !c.Pending {
				kept = append(kept, c)
			}
		}
		t.rev.comments = kept
		t.rev.rowsFor = ""
		names := map[string]string{"APPROVE": "承認", "REQUEST_CHANGES": "変更を要求", "COMMENT": "コメント"}
		a.info("レビューを送信しました（%s）", names[m.event])
		t.rev.loading = true
		return t.rev.load()
	}
	return nil
}

func (a *App) handleReviewKey(t *tab, msg tea.KeyMsg) tea.Cmd {
	r := t.rev
	key := msg.String()
	if a.pending == " " {
		a.pending = ""
		return a.spaceMode(key)
	}
	half := max(1, r.pageH/2)
	switch key {
	case "q", "esc":
		if r.pendingCount() > 0 && !a.confirmClose {
			a.confirmClose = true
			a.msg, a.msgErr = "未送信のコメントがあります。もう一度 q で破棄して閉じます", true
			return nil
		}
		a.closeTab(true)
		return nil
	case "r":
		r.loading = true
		return r.load()
	case ":":
		return a.openCmdline(":")
	case " ":
		a.pending = " "
		return nil
	}
	if r.loading || len(r.files) == 0 {
		return nil
	}
	switch key {
	case "tab", "h", "l", "left", "right":
		r.focusFiles = key == "h" || key == "left" || (key == "tab" && !r.focusFiles)
	case "j", "down":
		if r.focusFiles {
			r.selectFile(r.fileSel + 1)
		} else {
			r.moveCursor(1)
		}
	case "k", "up":
		if r.focusFiles {
			r.selectFile(r.fileSel - 1)
		} else {
			r.moveCursor(-1)
		}
	case "J":
		r.selectFile(r.fileSel + 1)
	case "K":
		r.selectFile(r.fileSel - 1)
	case "]", "n":
		r.moveHunk(1)
	case "[", "p":
		r.moveHunk(-1)
	case "ctrl+d", "pgdown":
		r.moveCursor(half)
	case "ctrl+u", "pgup":
		r.moveCursor(-half)
	case "g", "home":
		r.cursor = 0
	case "G", "end":
		r.cursor = max(0, len(r.buildRows())-1)
	case "s":
		r.split = !r.split
		r.rowsFor = ""
		r.selectFile(r.fileSel)
	case "v":
		f := r.file()
		r.viewed[f.Path()] = !r.viewed[f.Path()]
		if r.viewed[f.Path()] && r.fileSel < len(r.files)-1 {
			r.selectFile(r.fileSel + 1)
		}
	case "c":
		if r.focusFiles || r.target() == nil {
			a.fail(errString("コメントする行に移動してください"))
			return nil
		}
		l := r.target()
		_, no := lineKey(l)
		return a.pal.ask(fmt.Sprintf("%s:%d へのコメント（\\n で改行）", r.file().Path(), no), "", func(v string) tea.Cmd {
			v = strings.TrimSpace(strings.ReplaceAll(v, `\n`, "\n"))
			if v != "" {
				r.addComment(v)
			}
			return nil
		})
	case "d", "x":
		if !r.deletePendingAtCursor() {
			a.fail(errString("未送信のコメントの上で押してください"))
		}
	case "y":
		md := r.notesMarkdown(func(p string) string { return p })
		if md == "" {
			a.fail(errString("コメントがありません（c で追加）"))
			return nil
		}
		a.copyText(md)
		a.info("%d 件のコメントを Markdown でコピーしました", r.pendingCount())
	case "a":
		md := r.notesMarkdown(a.reviewToRoot)
		if md == "" {
			a.fail(errString("コメントがありません（c で追加）"))
			return nil
		}
		a.ctx = &ctxPreview{text: reviewPrompt + md}
		return nil
	case "enter", "o":
		f := r.file()
		l := r.target()
		base := a.gitTop
		if base == "" {
			base = a.root
		}
		if f.Status == "deleted" || !a.openFile(filepath.Join(base, f.Path())) {
			return nil
		}
		if l != nil && l.NewNo > 0 {
			a.curText().buf.GotoLine(l.NewNo)
		}
		a.follow = true
		return nil
	case "A", "R", "S":
		if r.pr == nil {
			a.fail(errString("ローカルの変更はレビューを送信できません（y でコメントをコピー）"))
			return nil
		}
		events := map[string]string{"A": "APPROVE", "R": "REQUEST_CHANGES", "S": "COMMENT"}
		labels := map[string]string{"A": "承認", "R": "変更を要求", "S": "コメント"}
		ev := events[key]
		return a.pal.ask(fmt.Sprintf("%s して送信（未送信コメント %d 件）。本文", labels[key], r.pendingCount()), "", func(v string) tea.Cmd {
			v = strings.ReplaceAll(v, `\n`, "\n")
			if ev != "APPROVE" && strings.TrimSpace(v) == "" && r.pendingCount() == 0 {
				a.fail(errString("本文かコメントが必要です"))
				return nil
			}
			a.info("送信しています…")
			return r.submit(ev, v)
		})
	case "C":
		if r.pr == nil {
			return nil
		}
		a.showTerm = true
		cmd := a.setFocus(focusTerminal)
		return tea.Batch(cmd, a.term.run(fmt.Sprintf("gh pr checkout %d", r.pr.Number)))
	}
	return nil
}

func (a *App) reviewClick(r *review, l layout, x, y int) {
	if r.loading || len(r.files) == 0 {
		return
	}
	listW := clamp(l.rightW/4, 20, 40)
	row := y - l.editY - 2 // review header + pane title
	if x-l.rightX < listW {
		r.focusFiles = true
		r.selectFile(r.fileScroll + row)
		return
	}
	r.focusFiles = false
	r.cursor = clamp(r.scroll+row, 0, max(0, len(r.buildRows())-1))
}

// splitGlobs splits a comma separated glob list, dropping quotes and blanks.
func splitGlobs(v string) []string {
	var out []string
	for _, g := range strings.Split(v, ",") {
		if g = strings.Trim(strings.TrimSpace(g), `'"`); g != "" {
			out = append(out, g)
		}
	}
	return out
}
