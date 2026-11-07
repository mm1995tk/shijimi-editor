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
- 間違いのあるファイルがあると、shijimi-editor は起動しません。知らない項目は警告だけです（[ファイルに間違いがあるとき](#ファイルに間違いがあるとき)）。
- `[agent]` はユーザー自身の `config.toml`（`~/.config/shijimi-editor/config.toml`、または下の nightly 版のディレクトリ）からだけ読みます（[AI エージェント連携](agent.md)）。

## ファイルに間違いがあるとき

どれか 1 つのファイルに間違いがあると、`shj` も `kisui serve` も起動しません。間違い全部について、ファイルの場所・行・理由を出して、0 以外で終わります。

```
/home/me/.config/shijimi-editor/config.toml:3: editor.file-icons: true か false で書いてください（今は 文字列 です）
/home/me/.config/shijimi-editor/config.toml:8: ui-language: 値が正しくないか、この版の shijimi-editor では使えない値です。次のどれかで書いてください: en, ja
```

次のものが間違いです。

- TOML の書き方の間違い。評価できない Nix のファイル（`config.nix` / `languages.nix`）。Nix には行が無いので、行の代わりに項目の名前を出します。
- 型の違い（数を書くところに文字を書いた、など）と、使えない値（`ui-language`、`[terminal]` の `multiplexer`、負の `idle_stop_days`、0 以下の `timeout`、フォルダでない `start_dir`）。値が決まっている項目では、新しい版で足された値も、この版では使えないので止まります（メッセージに「値が正しくないか、この版の shijimi-editor では使えない値です」と出ます）。
- プロジェクトの `.shijimi-editor/config.toml` に書いた `[agent]`・`[server]`・`[gui]`・`[keys]`。これらはユーザー自身のフォルダからだけ読みます。

以前のバージョンは、これらの間違いを黙って無視していました。更新したあと、今まで起動できたファイルで起動しなくなることがあります。出たメッセージの場所を直してください。

起動中の `:config-reload` で間違いが見つかっても、エディタは止まりません。今の設定のまま、最初の間違いをステータス行に出します。ユーザー自身のファイルの間違いは `kisui serve` 自体を止め、`kisui attach` がその理由を kisui-terminal の接続エラーとして渡します。

## 知らない項目は無視します

この版が読まない項目は、間違いではありません。shijimi-editor は無視して起動します。新しい版用に書いた項目があっても古い版が起動するので、1 つのファイルを版の違う shijimi-editor で使えます。Helix にあっても shijimi-editor が読まない項目（`languages.toml` の `comment-token` など）も同じです。このマニュアルに載っている項目だけを読みます。

- `shj` は、起動したときと `:config-reload` のあとに、無視した最初の項目（ファイルの場所と行つき）をステータス行に出します。
- `kisui serve` は、無視した項目を全部、標準エラーに出してログにも書き、そのまま動きます。
- `[gui]` の中の知らない項目も同じです。

```
/home/me/.config/shijimi-editor/config.toml:4: editor.bogus: 知らない項目なので無視しました（新しい版の shijimi-editor 用かもしれません。この版は読みません）
```

## nightly 版

`shj-nightly`（[インストール](install.md)）は `~/.config/shijimi-editor/` の代わりに `~/.config/shijimi-editor-nightly/` を読むので、2 つの版で設定を分けられます。`~/.config/shijimi-editor-nightly/` が無い間は、代わりに `~/.config/shijimi-editor/` を読みます。nightly 版だけの設定にしたくなったら、ディレクトリを作ってください。プロジェクトの `.shijimi-editor/` は両方で同じです。

`[experimental]` には、リリースの前に試している途中の機能を置きます。キーの既定値は nightly 版では `true`、安定版では `false` です。キーは次のとおりです。

| キー | 意味 |
| --- | --- |
| `multiplexer` | ターミナルマルチプレクサの中で、ターミナルパネルの代わりにそのペインを使う。どれを使うかは `[terminal]` の `multiplexer`（[ターミナル](terminal.md#ターミナルマルチプレクサの中で)）。古い名前 `herdr` も同じキーとして読む |

`[terminal]` の `multiplexer` は、`[experimental]` の `multiplexer` が有効なときに、どのマルチプレクサのペインを使うかを選びます。

| 値 | 意味 |
| --- | --- |
| `"auto"`（既定） | shijimi が動いているマルチプレクサのうち一番内側のもの。kisui-terminal のプライマリターミナルでは、そのペイン（そこでは `"none"` 以外のどの値でも同じ）。それ以外は shijimi の親プロセスをたどり、最初に見つかったマルチプレクサを使う。無ければターミナルパネル |
| `"herdr"` | herdr（`HERDR_ENV=1` が必要） |
| `"tmux"` | tmux 3.1 以降（`TMUX` が必要） |
| `"zellij"` | zellij 0.44 以降（`ZELLIJ` が必要） |
| `"cmux"` | macOS の cmux（`CMUX_WORKSPACE_ID` と `CMUX_SURFACE_ID` が必要）。ペインは隠せない |
| `"gui"` | kisui-terminal のペイン（kisui-terminal のワークスペースのプライマリターミナルで動く shijimi-editor だけ。ほかではターミナルパネル） |
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
multiplexer = "auto"      # "auto" / "herdr" / "tmux" / "zellij" / "cmux" / "gui" / "none"


# ~/.config/shijimi-editor/config.toml だけで有効
[agent]
command = "claude-agent-acp"
args = []

[server]
idle_stop_days = 7        # 0 = never stop
start_dir = "~/work"      # 新しいワークスペースのシェルを始めるフォルダ（既定 ~）
```

## kisui-terminal のマシン

kisui-terminal（`shj` ではなく）は、kisui-terminal を動かしているマシンの config.toml の `[[gui.machines]]` を読み、サイドバーの「ローカル」のあとに出します。使うマシンが決まっている人が、kisui-terminal を入れ直したり別の Mac に入れたりするたびにマシンを足し直さなくて済むようにするものです。

```toml
[[gui.machines]]
ssh = "dev-vm"   # ssh の行き先（必須。~/.ssh/config のホスト名など）
name = "dev"     # サイドバーに出す名前（省くと ssh の行き先）
```

- kisui-terminal は起動したときにファイルを読みます。nightly 版の kisui-terminal は、`shj-nightly` と同じく `shijimi-editor-nightly/` を読みます（このページの「nightly 版」）。読むのは `config.toml` だけで、`config.nix` は読みません。
- ここに書いたマシンは、kisui-terminal から外せず、名前も変えられません（右クリックのメニューは押せず、理由が出ます）。変えるには、ファイルを直して kisui-terminal を起動し直します。kisui-terminal で足したマシンは、今までどおり kisui-terminal の中に保存します。
- kisui-terminal で足したマシンの ssh の行き先がファイルのものと同じなら、ファイルのほうを出します。kisui-terminal で足したマシンの名前がファイルのマシンと同じあいだは、そのマシンは出しません。
- ファイルに間違い（TOML の書き方、型の違い、使えない ssh の行き先、同じ ssh の行き先や名前が 2 つ、空の `name` や「ローカル」）があると、kisui-terminal は起動しません。ファイルの場所・行・理由だけの画面を出して、ほかは何もしません。直してから kisui-terminal を起動し直してください。`shj` は `[gui]` のあるファイルでも起動します。
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
| `editor.auto-save.on-switch` | 今は使いません。ワークスペースが別のフォルダへ切り替わるときに `kisui serve` が開いているファイルをどうするかを決めるものでしたが、`kisui serve` はファイルを編集しなくなりました。書いてあるファイルが読めなくならないよう、`save` か `discard` は受け付けます。自動保存を切っても必須ではありません | — |
| `[[language]]` の `formatter` | `:fmt` で動かす整形コマンド（languages.toml） | [LSP](lsp.md) |
| `search.exclude` / `search.include` | 全体検索の除外 / 包含 | [ファイルと検索](files.md) |
| `[keys.normal]` `[keys.insert]` `[keys.select]` | Helix モードのキーの差し替え（`~/.config/shijimi-editor/` だけ） | [編集（Helix モード）](editing-helix.md#キーの差し替え) |
| `browser.listen-host` | ポート転送の待ち受けアドレス | [外部ブラウザとポート転送](browser.md) |
| `agent.command` / `agent.args` / `agent.environment` | 提案パネルが使う ACP エージェント | [AI エージェント連携](agent.md) |
| `server.start_dir` | 新しいワークスペースのプライマリターミナルが始まるフォルダ（絶対パス、または `~` で始まるパス）。既定は `~`。無いフォルダは設定の間違い。プロジェクトの中なら、ワークスペースのフォルダがそのまま決まる | — |
| `server.idle_stop_days` | kisui-terminal がつながらないまま `kisui serve` が動き続ける日数。過ぎると自分で止まる（既定 7、0 で止まらない）。動いているシェルも終わる。ワークスペースの一覧（それぞれのフォルダ。フォルダ未定のものも）は `serve.log` の隣の `workspaces.json` に保存され、次の `kisui serve` が作り直す。プライマリターミナルはそのフォルダ（フォルダが無い・未定なら `start_dir`）で新しいシェルを始める。分割したターミナルは戻らない | — |

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
