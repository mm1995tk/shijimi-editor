# LSP

言語サーバー（LSP）を使って、次の機能を提供します。

| 機能 | Helix モード | VS Code モード |
| --- | --- | --- |
| 診断 | `]d` / `[d` で移動、`Space d` で一覧 | — |
| 補完 | 自動。`Ctrl+X` / `Ctrl+Space` で手動 | 自動 |
| ホバー | `K` / `Space k` | — |
| 定義へ移動 | `gd` | `F12` |
| 参照 | `gr` | `Shift+F12` |
| 名前の変更 | `Space r` | `F2` |
| 整形 | `:fmt` | `:fmt` |

`—` の操作は、コマンドパレット（`F1`）から実行できます。

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
