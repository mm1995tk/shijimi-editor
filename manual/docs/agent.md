# AI エージェント連携

ターミナルパネルで動かす AI エージェント（Claude Code など）と、shijimi の間で情報をやりとりします。

## Claude Code の IDE 連携

shijimi は Claude Code の IDE 連携プロトコルに対応しています。VS Code 拡張や claudecode.nvim と同じプロトコルです。

- 起動すると、`127.0.0.1` の空きポートで WebSocket の MCP サーバーを立てます。
- ロックファイル `~/.claude/ide/<ポート>.lock`（権限 0600）に、接続先と認証トークンを書きます。`CLAUDE_CONFIG_DIR` があれば、その下の `ide/` に書きます。終了すると消します。
- shijimi のターミナルパネルで `claude` を起動すると、自動で接続します。
- 外のターミナルで動く Claude Code は、作業ディレクトリが shijimi のルート配下なら、`/ide` で選べます。
- 認証トークンの無い接続と、ブラウザ（`Origin` ヘッダー付き）からの接続は拒否します。

### エージェントが使える道具

| 道具 | 内容 |
| --- | --- |
| `getCurrentSelection` / `getLatestSelection` | 選択範囲（なければカーソル位置）とファイル |
| `getOpenEditors` / `getWorkspaceFolders` | 開いているファイル / ルート |
| `getDiagnostics` | LSP の診断。URI を省略すると全ファイル |
| `openFile` | ファイルを開く。`startText` / `endText` で範囲を選択 |
| `openDiff` | 変更を差分で表示する。**Enter で承認、Esc で却下** |
| `closeAllDiffTabs` / `close_tab` | 待っている差分を却下して閉じる |
| `checkDocumentDirty` / `saveDocument` | 未保存かどうかの確認 / 保存 |

- `openDiff` で承認しても、shijimi はファイルを書きません。承認の後に Claude Code が書き込みます。
- 選択範囲やカーソルが動くたびに、shijimi は `selection_changed` を送ります。

## 現在の文脈をエージェントへ送る

次のどれかで実行します。

- `Space a`（Helix モード）
- `Alt+A`
- コマンドパレット

選択範囲（選択がなければカーソルの行）を、次の形に整えます。

1. `パス:開始行-終了行`
2. 選択した本文（コードブロック）
3. その行にかかる診断（エラー・警告など）

IDE 連携で接続しているときは、整えた文を `at_mentioned` として Claude Code に送ります。
接続していないときは、プレビューに出ます。

| キー | 動作 |
| --- | --- |
| `Enter` | ターミナルパネルに貼り付ける（`Enter` は送らない） |
| `Esc` / `q` | やめる |
| `j` / `k` | プレビューをスクロール |

- 保存済みのファイルでだけ使えます。
- 200 行を超える部分は省略します。

## レビューコメントを送る

PR のレビューで書いたコメントを、エージェントに渡せます。
手順は [レビュー](review.md) にあります。

## シェルから shijimi を操作する

shijimi は起動すると Unix ソケット（権限 0600）を作ります。
ソケットの場所は、ターミナルパネルのシェルの環境変数 `SHIJIMI_SOCKET` に入ります。
そのシェルで次のコマンドが使えます。IDE 連携にない操作だけを残しています。

| コマンド | 動作 |
| --- | --- |
| `shj diff [パス]` | git-diff モードにする。パスを付けるとそのファイルを diff 表示で開く |

!!! note "削除したコマンド"
    `shj open` と `shj status` は、IDE 連携の `openFile` と `getOpenEditors` / `getCurrentSelection` に置き換えて削除しました。

## スキルを書き出す

```sh
shj skill install [書き出し先]
```

`shj diff` の使い方を書いたスキル（`SKILL.md`）を書き出します。
書き出し先を省略すると `~/.claude/skills/shijimi/` に書きます。
Claude Code はこのスキルを読んで、差分を shijimi で開けるようになります。
