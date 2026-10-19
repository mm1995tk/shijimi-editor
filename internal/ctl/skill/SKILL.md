---
name: dageki
description: 実行中の dageki（ターミナル IDE）を操作する。差分を git-diff モードで見せる、Markdown プレビューを開く、検索・テーマ変更など `:` コマンドを実行するときに使う。
---

# dageki の操作

dageki のターミナルパネルの中、または環境変数 `DAGEKI_SOCKET` が設定されたシェルで使えます。
`DAGEKI_SOCKET` が無いときは dageki が動いていないので、このスキルは使いません。

ファイルを開く・選択範囲や診断を知る・変更の承認は、Claude Code の IDE 連携（MCP）で行います。`/ide` で dageki に接続されていれば、そちらの道具を使ってください。このスキルはそれらに無い操作だけを扱います。

## コマンド

- `dgk diff [path]`: git-diff モードにする。`path` を付けるとそのファイルを diff 表示で開く。
- `dgk exec ":<コマンド>"`: 人が dageki で打つ `:` コマンドをそのまま実行する。先頭の `:` は省略できる。

`exec` は成功すると終了コード 0 で、ステータス欄に出た文を標準出力に書く。失敗すると終了コード 1 で、エラーの文を標準エラーに書く。画面は見えないので、この返事で成否を確かめる。

## exec で使える `:` コマンド

- 表示: `:preview`（Markdown プレビューの切替）、`:gitdiff [on|off|split|unified|base [head|pr]]`、`:tree`、`:sh [コマンド]`（ターミナルパネルで実行）
- ファイル: `:o <パス>`、`:n`、`:w [パス]`、`:wa`、`:bc`、`:reload`、`:<行番号>`
- 検索: `:grep [-x 除外glob] [-i 含めるglob] <文字列>`、`:replace [...] <文字列> => <置換後>`
- レビュー: `:pr <番号>`
- LSP: `:fmt`、`:signature`、`:lsp-restart`
- 設定: `:theme <名前>`、`:keymap [helix|vscode]`、`:config-reload`
- そのほか: `:browser forward|list|stop`、`:open-url <URL>`

## exec で使えないもの

- 終了するコマンド（`:q` `:q!` `:qa` `:wq` `:wqa` `:x` など）と、保存せずにタブを閉じる `:bc!`。エラーになる。
- 引数なしだと人が選ぶ画面になるコマンド（`:o` `:theme` `:grep` `:replace` `:symbol` `:symbol-all` `:action`、番号以外の引数の `:pr`）。引数を付ける。
- 未保存の変更があるファイルへの `:reload` と `:bc`。先に `:w` で保存する。
- LSP など時間がかかる処理は、始めた時点で成功を返す。結果は返事に入らない。

## 使いどころ

- ファイルを編集したあと、ユーザーに変更を確認してほしいときは `dgk diff <path>` を実行する。
- Markdown を見せたいときは、ファイルを開いて `dgk exec ":preview"` を実行する。

## 注意

- 第 1 引数が `diff` / `exec` / `skill` のときはサブコマンドとして扱う。同名のファイルを指すときは `./diff` のようにパスで書く。
- 編集はこのコマンドではできない。ファイルは通常どおり変更する。dageki はディスク上の変更を追従する。
