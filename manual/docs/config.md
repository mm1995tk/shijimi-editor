# 設定ファイル

設定ファイルの形式は Helix と同じです。

## 読み込む順番

次の順に読み込みます。後のものが優先されます。

1. 組み込みの既定値
2. `~/.config/helix/languages.toml`、`~/.config/helix/config.toml`
3. `~/.config/dageki/languages.toml`、`~/.config/dageki/config.toml`
4. プロジェクトの `.helix/languages.toml`、`.helix/config.toml`
5. プロジェクトの `.dageki/languages.toml`、`.dageki/config.toml`

- `languages.toml` には言語サーバーを書きます（[LSP](lsp.md)）。
- `config.toml` にはそれ以外を書きます（このページ）。
- 実行中に `:config-reload` を実行すると、読み直します。

!!! warning "dageki だけの設定"
    `[browser]` と `[search]` は Helix にない設定です。
    `~/.config/dageki/config.toml` か `.dageki/config.toml` に書いてください。
    `.helix/config.toml` に書くと、Helix が未知のセクションとして拒否します。

## config.toml の例

```toml
theme = "github_dark"
keymap = "helix"          # "vscode" で VS Code 風の操作

[editor]
line-number = "relative"  # Helix と同じ
file-icons = true         # ファイルツリーの Nerd Font アイコン。false で ▸/▾ 表示
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"  # 空で無効
watch-files = true        # 他のプロセスによるファイル変更を反映する
mouse-hover = true        # 単語にマウスを乗せるとホバーを表示する

# インデントガイド（インデントごとの薄い縦線）。false で無効
[editor.indent-guides]
render = true

# 自動保存（既定で有効。編集が 1 秒止まると保存。名前のないファイルは対象外）
[editor.auto-save.after-delay]
enable = true
timeout = 1000            # ミリ秒

[search]
exclude = ["*.lock", "dist/**"]
include = []

[browser]
listen-host = "127.0.0.1"
```

## 項目

| 項目 | 説明 | 詳しいページ |
| --- | --- | --- |
| `theme` | テーマ | このページの「テーマ」 |
| `keymap` | `helix`（既定）または `vscode` | [編集（VS Code モード）](editing-vscode.md) |
| `editor.line-number` | 行番号の表示。Helix と同じ値 | — |
| `editor.file-icons` | ファイルツリーのアイコン | [ファイルと検索](files.md) |
| `editor.mermaid-command` | mermaid を PNG にするコマンド | [プレビュー](preview.md) |
| `editor.watch-files` | 外部での変更を反映するか | [ファイルと検索](files.md) |
| `editor.indent-guides.render` | インデントガイドを描くか（既定 true） | — |
| `editor.mouse-hover` | マウスを乗せたときにホバーを表示するか | [LSP](lsp.md) |
| `editor.auto-save.after-delay` | 自動保存 | [ファイルと検索](files.md) |
| `search.exclude` / `search.include` | 全体検索の除外 / 包含 | [ファイルと検索](files.md) |
| `browser.listen-host` | ポート転送の待ち受けアドレス | [外部ブラウザとポート転送](browser.md) |

## テーマ

組み込みのテーマは 3 つです。

| 名前 | 説明 |
| --- | --- |
| `dark-plus` | 既定 |
| `github-dark` | — |
| `light-plus` | — |

Helix のテーマ名も使えます。`github_dark*` は `github-dark` に、`github_light*` と `light` は `light-plus` に対応付けます。

切り替える方法は次の 3 つです。

- `config.toml` に `theme = "<名前>"` と書く。
- 実行中に `:theme <名前>` を入力する。名前を省略すると一覧が出ます。
- コマンドパレットの「設定: テーマを切り替え」を選ぶ。
