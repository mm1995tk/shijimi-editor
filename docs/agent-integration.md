# AI エージェント連携の機能案

Issue #13 の検討結果です。機能を 5 つに分け、それぞれ個別の Issue にしました。

## 方針

- エージェントは shijimi の中のターミナルパネル、または外のターミナルで動きます。どちらでも使えるようにします。
- shijimi からエージェントへ渡す方法は「ターミナルへ貼り付け」から始めます。特定のエージェントに依存しません。
- エージェントから shijimi を操作する入口は、Claude Code の IDE 連携（WebSocket の MCP サーバー、#64）を主にします。IDE 連携に無い操作（git-diff モードにする）だけ、Unix ソケットと CLI（環境変数 `SHIJIMI_SOCKET`）に残します。

## 機能一覧

| # | 機能 | 優先度 | 依存 |
|---|------|--------|------|
| A | 現在の文脈をエージェントへ送る | 高 | なし |
| B | 制御用ソケットと `shj` サブコマンド（#64 で `diff` のみに縮小） | 高 | なし |
| C | レビューコメントをエージェントへ返す | 中 | A |
| D | エージェント用スキルの配布 | 中 | B |
| E | エージェントの変更の追従（外部変更の再読み込みと差分表示） | 低 | なし |
| F | Claude Code の IDE 連携（#64） | 高 | なし |

### A. 現在の文脈をエージェントへ送る

- 選択範囲、ファイルパス、行番号、カーソル位置の診断を `path:行-行` と本文の形式に整えます。
- 送り先はターミナルパネルで動いているプロセスの標準入力です。送る前にプレビューを出し、確認してから貼り付けます。
- コマンドパレットとキーに割り当てます。

### B. 制御用ソケットと CLI

- `shj diff [path]` を用意します（当初は `open` と `status` もありましたが、#64 の IDE 連携の `openFile` と `getOpenEditors` と `getCurrentSelection` に重なるため削除しました）。
- 通信は改行区切りの JSON です。サブコマンドが無い起動は今までどおりです。
- 第 1 引数がサブコマンド名（`diff` / `exec` / `skill`）なら、サブコマンドとして扱います。同じ名前のファイルやディレクトリを開くときは `./diff` のようにパスで書きます。
- 受け付けるのは同じユーザーのソケットだけです（パーミッション 0600）。

### C. レビューコメントの返送

- PR レビュー画面のコメントを、Markdown のまま A と同じ経路でエージェントへ送ります。
- 現在は Markdown をクリップボードへコピーする機能のみです。

### D. スキルの配布

- `shj skill install` で、B の CLI（`diff`）の使い方と IDE 連携との使い分けを書いたスキルを `~/.claude/skills/shijimi/` に書き出します。
- 書き出し先は引数で変えられます。

### E. エージェントの変更の追従

- ディスク上の変更を検知し、開いているバッファへ反映します（未保存の編集があるときは確認します）。
- 変更後に git-diff モードで差分を見るショートカットを用意します。`Ctrl+G` です。
- ソケットや CLI は使わないので、B を待たずに実装できます。既存の `:reload` を自動で呼ぶ形です。

## 決めうちした点

- 最初の送り先はターミナルパネルのみとし、エージェント固有の API は使いません。
- ソケットの場所は環境変数で渡します。固定のパスは使いません。
- 優先順は A → B → C → D → E です。E は依存が無いので、順番は入れ替えられます。
- サブコマンド名は、同名のパスより優先します。

## A の実装で決めうちした点

- 選択が無いときは、カーソル行の `パス:行` と、その行の診断だけを送ります（本文は送りません）。
- 選択が 200 行を超えるときは、200 行で切って「残り N 行を省略」と書きます（`maxContextLines`）。
- 貼り付けは末尾に Enter を付けません。送り先のプログラムが括弧付き貼り付け（`ESC[?2004h`）を有効にしているときは、それで包みます。
- 送り先は、ターミナルパネルで選んでいるタブです。タブが無いときは新しく開きます。
- キーは `Space a` と `Alt+A` です。キーの割り当ては `commands.go` と `helix.go` / `app.go` の 1 行ずつで変えられます。

## C の実装で決めうちした点

- レビュー画面のキーは `a` です（`y` はこれまでどおりコピー）。`commands.go` の `case "a"` で変えられます。
- 送る内容は、未送信のコメントの Markdown の前に「次のレビューコメントに対応してください。」の 1 文を付けたものです。文は `reviewPrompt` で変えられます。
- コメントのパスは、リポジトリのトップ基準から shijimi のルート基準（ターミナルの cwd）に直して送ります。
- PR レビュー画面でも使えます（ヘルプ行にも載せています）。
- 経路は A と同じです（プレビュー → Enter で貼り付け、Enter キーは押しません）。
- 送ったあともコメントは消しません。

## F. Claude Code の IDE 連携（#64）

VS Code 拡張や claudecode.nvim と同じプロトコルで、shijimi を Claude Code の IDE として使えるようにします。

### 置き換えたもの・残したもの

| 既存機能 | 扱い |
|----------|------|
| `shj open`（B） | 削除。`openFile` が同じことをします |
| `shj status`（B） | 削除。`getCurrentSelection` / `getOpenEditors` / `getWorkspaceFolders` が同じ情報を返します |
| `shj diff`（B） | 残す。git-diff モードにする操作は IDE 連携に無いためです |
| スキル（D） | 残す。`diff` と IDE 連携との使い分けだけを書きます |
| 現在の文脈を送る（A） | 残す。IDE 連携に接続中は貼り付けの代わりに `at_mentioned` を送ります。接続していないとき、Claude Code 以外のエージェントのときは貼り付けです |

### プロトコル

- `127.0.0.1` の空きポートで WebSocket を待ち受けます（ライブラリは `github.com/gorilla/websocket`）。サブプロトコルは `mcp`、メッセージは MCP の JSON-RPC 2.0 です（`initialize` / `ping` / `tools/list` / `tools/call`）。
- 接続先をロックファイル `<Claude の設定ディレクトリ>/ide/<port>.lock` に書きます。設定ディレクトリは `CLAUDE_CONFIG_DIR`、なければ `~/.claude` です。内容は `{pid, workspaceFolders, ideName: "shijimi", transport: "ws", authToken}` で、パーミッションは 0600 です。終了時に消します。起動時には、pid が存在しない shijimi のロックファイルを掃除します。
- 接続には HTTP ヘッダー `x-claude-code-ide-authorization: <authToken>` が必要です。トークンは起動ごとに乱数で作ります。`Origin` ヘッダー付き（ブラウザ）の接続は拒否します。
- ターミナルパネルのプロセスには `CLAUDE_CODE_SSE_PORT=<port>`・`ENABLE_IDE_INTEGRATION=true`・`FORCE_CODE_TERMINAL=true`（claudecode.nvim と同じ 3 つ）を渡し、中で起動した Claude Code が自動で接続するようにします。

### 道具（`tools/call`）

`getCurrentSelection`、`getLatestSelection`、`getOpenEditors`、`getWorkspaceFolders`、`getDiagnostics`、`openFile`、`openDiff`、`closeAllDiffTabs`、`close_tab`、`checkDocumentDirty`、`saveDocument`。一覧は `internal/ui/ide.go` の `IDETools` です。

- `openFile` はフォーカスを動かしません（エージェントを使っているターミナルに残ります）。存在しないファイルはエラーにします。
- `getDiagnostics` は LSP の診断です。`severity` は `Error` / `Warning` / `Information` / `Hint` です。位置は 0 始まりで、`character` は LSP の値をそのまま返します。
- `openDiff` は人の判断が出るまで返事を保留します。差分はモーダルで表示し、Enter で `FILE_SAVED` と新しい内容を返し（ファイルは Claude Code が書き込みます。claudecode.nvim と同じ）、Esc で `DIFF_REJECTED` を返します。複数の差分は順に出します。接続が切れたら保留を破棄します。差分の編集（承認前に右側を直す）は未対応です。
- 通知は `selection_changed`（カーソルか選択が動いたとき。同じ値は送りません）と `at_mentioned`（`Space a` / `Alt+A`。行は 0 始まり）です。

### 決めうちした点

- 差分の表示はタブではなくモーダルです。承認の操作を 1 つのキーに絞るためです。
- shijimi はファイルへ書き込みません。承認すると Claude Code が書き込み、開いているバッファは既存のファイル監視（`watch-files`）が読み込み直します。
- `close_tab` は、差分がすでに無くても `TAB_CLOSED` を返します（Claude Code が決定後の後始末に呼ぶため）。
- 起動に失敗しても（ホームが書けないなど）エディターは動きます。制御用ソケットと同じ扱いです。
- Windows は対象外です（pid の生死確認に `kill(pid, 0)` を使います）。
