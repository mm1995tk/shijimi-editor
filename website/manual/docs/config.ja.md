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
- 間違いのあるファイルがあると、shijimi-editor は起動しません（[ファイルに間違いがあるとき](#ファイルに間違いがあるとき)）。
- `[agent]` はユーザー自身の `config.toml`（`~/.config/shijimi-editor/config.toml`、または下の nightly 版のディレクトリ）からだけ読みます（[AI エージェント連携](agent.md)）。

## ファイルに間違いがあるとき

どれか 1 つのファイルに間違いがあると、`shj` も `shj serve` も起動しません。間違い全部について、ファイルの場所・行・理由を出して、0 以外で終わります。

```
/home/me/.config/shijimi-editor/config.toml:4: editor.bogus: 知らない項目です（shijimi-editor は読みません）
/home/me/.config/shijimi-editor/config.toml:3: editor.file-icons: true か false で書いてください（今は 文字列 です）
```

次のものが間違いです。

- TOML の書き方の間違い。評価できない Nix のファイル（`config.nix` / `languages.nix`）。Nix には行が無いので、行の代わりに項目の名前を出します。
- 知らない項目。Helix にあっても shijimi-editor が読まない項目（`languages.toml` の `comment-token` など）も含みます。このマニュアルに載っている項目だけを読みます。
- 型の違い（数を書くところに文字を書いた、など）と、使えない値（`ui-language`、`[terminal]` の `multiplexer`、負の `idle_stop_days`、0 以下の `timeout`、フォルダでない `start_dir`、`save` / `discard` 以外の `on-switch`）。
- 自動保存を切った（`[editor.auto-save.after-delay] enable = false`）のに `[editor.auto-save] on-switch` が無い。切った行を指して知らせます。
- プロジェクトの `.shijimi-editor/config.toml` に書いた `[agent]`・`[server]`・`[gui]`・`[keys]`。これらはユーザー自身のフォルダからだけ読みます。

以前のバージョンは、これらの間違いを黙って無視していました。更新したあと、今まで起動できたファイルで起動しなくなることがあります。出たメッセージの場所を直してください。

起動中の `:config-reload` で間違いが見つかっても、エディタは止まりません。今の設定のまま、最初の間違いをステータス行に出します。`shj serve` では、プロジェクトのファイルの間違いは、そのワークスペースを開けない形で出て、GUI にはワークスペースを切り替えたときのエラーとして理由が出ます。ユーザー自身のファイルの間違いは `shj serve` 自体を止め、`shj attach` がその理由を GUI の接続エラーとして渡します。

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

# 自動保存を切る（enable = false）ときだけ必要: ワークスペースが別のフォルダへ
# 切り替わるとき、`shj serve` が開いているファイルをどうするか。"save" か "discard"
# [editor.auto-save]
# on-switch = "save"

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

[server]
idle_stop_days = 7        # 0 = never stop
start_dir = "~/work"      # 新しいワークスペースのシェルを始めるフォルダ（既定 ~）
```

## GUI のマシン

GUI（`shj` ではなく）は、GUI を動かしているマシンの config.toml の `[[gui.machines]]` を読み、サイドバーの「ローカル」のあとに出します。使うマシンが決まっている人が、GUI を入れ直したり別の Mac に入れたりするたびにマシンを足し直さなくて済むようにするものです。

```toml
[[gui.machines]]
ssh = "dev-vm"   # ssh の行き先（必須。~/.ssh/config のホスト名など）
name = "dev"     # サイドバーに出す名前（省くと ssh の行き先）
```

- GUI は起動したときにファイルを読みます。nightly 版の GUI は、`shj-nightly` と同じく `shijimi-editor-nightly/` を読みます（このページの「nightly 版」）。読むのは `config.toml` だけで、`config.nix` は読みません。
- ここに書いたマシンは、GUI から外せず、名前も変えられません（右クリックのメニューは押せず、理由が出ます）。変えるには、ファイルを直して GUI を起動し直します。GUI で足したマシンは、今までどおり GUI の中に保存します。
- GUI で足したマシンの ssh の行き先がファイルのものと同じなら、ファイルのほうを出します。GUI で足したマシンの名前がファイルのマシンと同じあいだは、そのマシンは出しません。
- ファイルに間違い（TOML の書き方、知らない項目、型の違い、使えない ssh の行き先、同じ ssh の行き先や名前が 2 つ、空の `name` や「ローカル」）があると、GUI は起動しません。ファイルの場所・行・理由だけの画面を出して、ほかは何もしません。直してから GUI を起動し直してください。`shj` は `[gui]` のあるファイルでも起動します。
- home-manager では `settings` に書きます（[Nix と home-manager](nix.md)）。

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
| `editor.auto-save.on-switch` | ワークスペースが別のフォルダへ切り替わる（プライマリターミナルで `cd` する）とき、`shj serve` が開いているファイルをどうするか。`save` は保存、`discard` は編集を捨てる。自動保存が有効なあいだは必ず保存するので要らない。**自動保存を切ったときは既定値が無く**、書いていなければ設定の間違いとして起動しない | — |
| `[[language]]` の `formatter` | `:fmt` で動かす整形コマンド（languages.toml） | [LSP](lsp.md) |
| `search.exclude` / `search.include` | 全体検索の除外 / 包含 | [ファイルと検索](files.md) |
| `[keys.normal]` `[keys.insert]` `[keys.select]` | Helix モードのキーの差し替え（`~/.config/shijimi-editor/` だけ） | [編集（Helix モード）](editing-helix.md#キーの差し替え) |
| `browser.listen-host` | ポート転送の待ち受けアドレス | [外部ブラウザとポート転送](browser.md) |
| `agent.command` / `agent.args` / `agent.environment` | 提案パネルが使う ACP エージェント | [AI エージェント連携](agent.md) |
| `server.start_dir` | 新しいワークスペースのプライマリターミナルが始まるフォルダ（絶対パス、または `~` で始まるパス）。既定は `~`。無いフォルダは設定の間違い。プロジェクトの中なら、ワークスペースのフォルダがそのまま決まる | — |
| `server.idle_stop_days` | GUI がつながらないまま `shj serve` が動き続ける日数。過ぎると自分で止まる（既定 7、0 で止まらない）。動いているシェルも終わる。未保存の編集は元のファイルの隣に `<ファイル名>.bk`（あれば `.bk.1` …）として書き、元のファイルは書き換えない。ワークスペースの一覧（それぞれのフォルダ。フォルダ未定のものも）は `serve.log` の隣の `workspaces.json` に保存され、次の `shj serve` が作り直す。プライマリターミナルはそのフォルダ（フォルダが無い・未定なら `start_dir`）で新しいシェルを始める。分割したターミナルと開いていたファイルは戻らない | — |

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
