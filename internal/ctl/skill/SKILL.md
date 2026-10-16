---
name: gh-editor
description: 実行中の gh-editor（ターミナル IDE）に git-diff モードで変更を見せる。ファイルを編集したあと、ユーザーに差分を確認してほしいときに使う。
---

# gh-editor の操作

gh-editor のターミナルパネルの中、または環境変数 `GH_EDITOR_SOCKET` が設定されたシェルで使えます。
`GH_EDITOR_SOCKET` が無いときは gh-editor が動いていないので、このスキルは使いません。

ファイルを開く・選択範囲や診断を知る・変更の承認は、Claude Code の IDE 連携（MCP）で行います。`/ide` で gh-editor に接続されていれば、そちらの道具を使ってください。このスキルはそれらに無い操作だけを扱います。

## コマンド

- `gh-editor diff [path]`: git-diff モードにする。`path` を付けるとそのファイルを diff 表示で開く。

## 使いどころ

- ファイルを編集したあと、ユーザーに変更を確認してほしいときは `gh-editor diff <path>` を実行する。

## 注意

- 第 1 引数が `diff` / `skill` のときはサブコマンドとして扱う。同名のファイルを指すときは `./diff` のようにパスで書く。
- 編集はこのコマンドではできない。ファイルは通常どおり変更する。gh-editor はディスク上の変更を追従する。
