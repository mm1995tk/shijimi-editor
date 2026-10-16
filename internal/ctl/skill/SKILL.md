---
name: gh-editor
description: 実行中の gh-editor（ターミナル IDE）を `gh-editor` コマンドで操作する。ユーザーにファイルを見せたい、差分を見せたい、いま開いているファイルやカーソル位置を知りたいときに使う。
---

# gh-editor の操作

gh-editor のターミナルパネルの中、または環境変数 `GH_EDITOR_SOCKET` が設定されたシェルで使えます。
`GH_EDITOR_SOCKET` が無いときは gh-editor が動いていないので、このスキルは使いません。

## コマンド

- `gh-editor open <path>[:行]`: ファイルを開く。`:行` を付けるとその行へ移動する。
- `gh-editor diff [path]`: git-diff モードにする。`path` を付けるとそのファイルを diff 表示で開く。
- `gh-editor status`: ルート、ブランチ、アクティブなタブ、カーソル行、開いているタブを JSON で表示する。

## 使いどころ

- ファイルを編集したあと、ユーザーに変更を確認してほしいときは `gh-editor diff <path>` を実行する。
- 特定の箇所を見せたいときは `gh-editor open src/main.go:42` のように行番号つきで開く。
- ユーザーが「いま開いているファイル」「この行」と言ったときは `gh-editor status` で場所を調べる。

## 注意

- 第 1 引数が `open` / `diff` / `status` / `skill` のときはサブコマンドとして扱う。同名のファイルを指すときは `./status` のようにパスで書く。
- 編集はこのコマンドではできない。ファイルは通常どおり変更する。gh-editor はディスク上の変更を追従する。
