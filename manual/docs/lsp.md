# LSP

言語サーバー（LSP）を使って、次の機能を提供します。

| 機能 | Helix モード | VS Code モード |
| --- | --- | --- |
| 診断 | `]d` / `[d` で移動、`Space d` で一覧 | — |
| 補完 | 自動。`Ctrl+X` / `Ctrl+Space` で手動 | 自動。`Ctrl+Space` で手動 |
| ホバー | `K` / `Space k` | — |
| 定義へ移動 | `gd` | `F12` |
| 参照 | `gr` | `Shift+F12` |
| 名前の変更 | `Space r` | `F2` |
| コードアクション（クイックフィックス・リファクタリング） | `Space .` / `:action` | `:action` |
| シンボル一覧（ファイル内） | `Space s` / `:symbol` | `:symbol` |
| シンボル検索（プロジェクト全体） | `Space S` / `:symbol-all` | `:symbol-all` |
| 整形 | `:fmt` | `:fmt` |

`—` の操作は、コマンドパレット（`F1`）から実行できます。

## コードアクション

`Space .`（または `:action`）で、カーソル位置で使えるコードアクションを一覧にします。選ぶと適用します。

- 診断のヒント（例：gopls の「Loop can be simplified using slices.Contains」）の直し方は、この一覧に出ます。
- 選択があるときは、選択した範囲のコードアクションを出します。選択がなく、カーソルが診断の範囲の外にあるときは、カーソルの行にある診断を使います。
- 変更は保存しません。確認してから `:w` で保存してください。

## シンボル一覧

`Space s`（または `:symbol`）で、開いているファイルの関数・型・変数などを一覧にします。選ぶとその行へ移動します。

- 一覧には種類（関数・型・変数など）と行番号が出ます。入れ子のシンボルは、外側の名前も一緒に出ます。
- 入力で絞り込めます。

`Space S`（または `:symbol-all`）は、プロジェクト全体からシンボルを名前で探します。

- 名前を入力すると、言語サーバーに問い合わせて一覧を作ります。入力が止まってから問い合わせます。
- 選ぶとそのファイルを開いて、定義の行へ移動します。

## マウスでホバーを出す

エディタ上の単語にマウスを乗せて 0.5 秒止めると、その位置のホバーが出ます。カーソルは動きません。

- ポップアップはマウスの位置に出ます。ポップアップの上にマウスがある間は閉じず、ホイールでスクロールできます。
- マウスが単語（同じ単語の別の文字へ動かしても閉じません）とポップアップの外へ出る、クリックする、キーを押すと閉じます。
- エディタにフォーカスがあるときだけ開きます。ポップアップはキーを奪わず、キーを押すと閉じます。
- 言語サーバーが未起動のとき、ホバー情報がないとき、エラーのときは何も表示しません。
- `[editor]` の `mouse-hover = false` で無効にできます（[設定ファイル](config.md)）。無効にすると、ボタンを押さないマウス移動のイベントも受け取りません。

## 組み込みの設定

次の言語サーバーの設定を組み込んでいます。サーバーが PATH にあれば、設定なしで動きます。

| 言語 | サーバー |
| --- | --- |
| Go | `gopls` |
| Rust | `rust-analyzer` |
| TypeScript / JavaScript（JSX・TSX を含む） | `typescript-language-server` |
| Python | `pylsp` |
| C / C++ | `clangd` |
| Nix | `nil` |
| Haskell | `haskell-language-server-wrapper` |
| Lua | `lua-language-server` |
| Bash | `bash-language-server` |
| Zig | `zls` |
| TOML | `taplo` |
| YAML | `yaml-language-server` |
| JSON | `vscode-json-language-server` |

## 設定の書き方

設定は Helix と同じ `languages.toml` に書きます。
読み込む場所と順番は [設定ファイル](config.md) にあります。

```toml
# .helix/languages.toml
[language-server.my-gopls]
command = "gopls"
args = ["serve"]
config = { gofumpt = true }          # initializationOptions として送ります
environment = { GOFLAGS = "-tags=integration" }

[[language]]
name = "go"
language-servers = ["my-gopls"]
roots = ["go.mod"]
indent = { tab-width = 4, unit = "\t" }
```

対応しているキーは次のとおりです。

| セクション | キー |
| --- | --- |
| `[language-server.*]` | `command` `args` `config` `environment` |
| `[[language]]` | `name` `language-id` `file-types` `roots` `language-servers` `indent.unit` |

## 再起動

| コマンド | 動作 |
| --- | --- |
| `:lsp-restart` | 言語サーバーを再起動 |
| `:config-reload` | 設定ファイルを読み直す |
