# 設定ファイル

設定ファイルは TOML で書きます。`languages.toml` は Helix と互換があります。Helix から移る人は、`languages.toml` を `~/.config/shijimi-editor/` にコピーしてください。

## 読み込む順番

次の順に読み込みます。後のものが優先されます。

1. 組み込みの既定値
2. `~/.config/shijimi-editor/languages.toml`、`~/.config/shijimi-editor/config.toml`
3. プロジェクトの `.shijimi-editor/languages.toml`、`.shijimi-editor/config.toml`

`~/.config` は、`$XDG_CONFIG_HOME` があればその場所です。macOS も含めて、OS を問わずこの場所から読みます（`~/Library/Application Support` からは読みません）。

- `languages.toml` には言語サーバーを書きます（[LSP](lsp.md)）。
- `config.toml` にはそれ以外を書きます（このページ）。
- 実行中に `:config-reload` を実行すると、読み直します。
- `[agent]` はユーザー自身の `config.toml`（`~/.config/shijimi-editor/config.toml`、または下の nightly 版のディレクトリ）からだけ読みます（[AI エージェント連携](agent.md)）。

## nightly 版

`shj-nightly`（[インストール](install.md)）は `~/.config/shijimi-editor/` の代わりに `~/.config/shijimi-editor-nightly/` を読むので、2 つの版で設定を分けられます。`~/.config/shijimi-editor-nightly/` が無い間は、代わりに `~/.config/shijimi-editor/` を読みます。nightly 版だけの設定にしたくなったら、ディレクトリを作ってください。プロジェクトの `.shijimi-editor/` は両方で同じです。

`[experimental]` には、リリースの前に試している途中の機能を置きます。キーの既定値は nightly 版では `true`、安定版では `false` です。キーは次のとおりです。

| キー | 意味 |
| --- | --- |
| `multiplexer` | ターミナルマルチプレクサの中で、ターミナルパネルの代わりにそのペインを使う。どれを使うかは `[terminal]` の `multiplexer`（[ターミナル](terminal.md#ターミナルマルチプレクサの中で)）。古い名前 `herdr` も同じキーとして読む |

`[terminal]` の `multiplexer` は、`[experimental]` の `multiplexer` が有効なときに、どのマルチプレクサのペインを使うかを選びます。

| 値 | 意味 |
| --- | --- |
| `"auto"`（既定） | shijimi が動いているマルチプレクサのうち一番内側のもの。shijimi の親プロセスをたどり、最初に見つかったマルチプレクサを使う。無ければターミナルパネル |
| `"herdr"` | herdr（`HERDR_ENV=1` が必要） |
| `"tmux"` | tmux 3.1 以降（`TMUX` が必要） |
| `"zellij"` | zellij 0.44 以降（`ZELLIJ` が必要） |
| `"cmux"` | macOS の cmux（`CMUX_WORKSPACE_ID` と `CMUX_SURFACE_ID` が必要）。ペインは隠せない |
| `"none"` | マルチプレクサの中でも、いつもターミナルパネルを使う |

## config.toml の例

```toml
theme = "github_dark"
keymap = "helix"          # "vscode" で VS Code 風の操作
ui-language = "en"        # 画面の文字の言語。"en"（既定）または "ja"

[editor]
line-number = "relative"  # 行番号の表示
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

[terminal]
multiplexer = "auto"      # "auto" / "herdr" / "tmux" / "zellij" / "cmux" / "none"

# ~/.config/shijimi-editor/config.toml だけで有効
[agent]
command = "claude-agent-acp"
args = []
```

## 項目

| 項目 | 説明 | 詳しいページ |
| --- | --- | --- |
| `theme` | テーマ | このページの「テーマ」 |
| `keymap` | `helix`（既定）または `vscode` | [編集（VS Code モード）](editing-vscode.md) |
| `ui-language` | 画面の文字（ステータス行・コマンドパレット・提案パネル・エラー・`shj --help`）の言語。`en`（英語、既定）または `ja`（日本語）。`LANG` などの環境変数からは選びません。ほかの値は無視して英語になります。`:config-reload` で切り替わります | — |
| `editor.line-number` | 行番号の表示。`absolute` または `relative`。Helix のキー操作では `\` で反対の方式を出せる | — |
| `editor.file-icons` | ファイルツリーのアイコン | [ファイルと検索](files.md) |
| `editor.mermaid-command` | mermaid を PNG にするコマンド | [プレビュー](preview.md) |
| `editor.watch-files` | 外部での変更を反映するか | [ファイルと検索](files.md) |
| `editor.indent-guides.render` | インデントガイドを描くか（既定 true） | — |
| `editor.mouse-hover` | マウスを乗せたときにホバーを表示するか | [LSP](lsp.md) |
| `editor.auto-save.after-delay` | 自動保存 | [ファイルと検索](files.md) |
| `[[language]]` の `formatter` | `:fmt` で動かす整形コマンド（languages.toml） | [LSP](lsp.md) |
| `search.exclude` / `search.include` | 全体検索の除外 / 包含 | [ファイルと検索](files.md) |
| `[keys.normal]` `[keys.insert]` `[keys.select]` | Helix モードのキーの差し替え（`~/.config/shijimi-editor/` だけ） | [編集（Helix モード）](editing-helix.md#キーの差し替え) |
| `browser.listen-host` | ポート転送の待ち受けアドレス | [外部ブラウザとポート転送](browser.md) |
| `agent.command` / `agent.args` / `agent.environment` | 提案パネルが使う ACP エージェント | [AI エージェント連携](agent.md) |

## テーマ

組み込みのテーマは 3 つです。

| 名前 | 説明 |
| --- | --- |
| `dark-plus` | — |
| `github-dark` | 既定 |
| `light-plus` | — |

Helix のテーマ名も使えます。`github_dark*` は `github-dark` に、`github_light*` と `light` は `light-plus` に対応付けます。

切り替える方法は次の 3 つです。

- `config.toml` に `theme = "<名前>"` と書く。
- 実行中に `:theme <名前>` を入力する。名前を省略すると一覧が出ます。
- コマンドパレットの「設定: テーマを切り替え」を選ぶ。

## キーの差し替え

`[keys.normal]` `[keys.insert]` `[keys.select]` で Helix モードのキーを差し替えられます。書き方とコマンド名の一覧は [編集（Helix モード）](editing-helix.md#キーの差し替え) にあります。`~/.config/shijimi-editor/config.toml` からだけ読みます（プロジェクトの `.shijimi-editor/config.toml` では無効）。
