# <img src="manual/docs/assets/icon.png" alt="" width="32" align="top"> shijimi

[English](README.md) | 日本語

ターミナルで動く統合開発環境 (TUI) です。画面の作りは VS Code に合わせています。編集操作は、Helix 風のモード編集と、モードの無い VS Code 風の操作から選べます。コマンド名は `shj` です。

ホームページ: <https://mm1995tk.github.io/shijimi-editor/ja/> / 説明書（英語と日本語）: <https://mm1995tk.github.io/shijimi-editor/manual/ja/>（ソースは [`site/`](site/) と [`manual/`](manual/)）

## 機能

- **モード編集と VS Code 風の操作**：Helix 風のモード編集（ノーマル / 挿入 / 選択、`:` コマンド）と、モードの無い VS Code 風の操作から選べます
- **マルチカーソル**（Helix 風）：`C` / `Alt+C` で下 / 上の行に複製、`s` で選択範囲を正規表現で分割、`,` で 1 つに戻す。挿入・削除・`c` などは全カーソルに適用
- **ファイルツリー**（Git の変更状態を M / U / A で表示）
- **LSP**：診断・補完・ホバー・シグネチャヘルプ・定義へ移動・参照・名前の変更・コードアクション・シンボル一覧・整形
- **LSP の設定**は Helix と互換がある `languages.toml` で書きます（`.shijimi-editor/languages.toml` か `~/.config/shijimi-editor/languages.toml`）。Helix から移る人は、`languages.toml` を `~/.config/shijimi-editor/` にコピーしてください
- **差分モード**（`Space G` / `Ctrl+G` で `HEAD` と、`Space g` / `:pr` でプルリクエストの base と比べる）：エディター全体を差分モードに切り替えます。変更ファイルをツリー（CHANGES）で出し、変更のあるファイルは**ファイル全体**を左右分割（または統合）の diff で開きます。右側は編集できます。右端のバーで変更の位置がわかり、行の中で変わった部分を強調します（下記）
- **画像のプレビュー**（PNG / JPEG / GIF / BMP / WebP / TIFF）
- **Markdown のプレビュー**（`Space m` / `:preview`）：見出し・リスト・コード・表を整形し、編集中のバッファの右に並べて表示します。もう一度押すとプレビューだけを全幅で表示します。mermaid のコードブロックは図として、1 行に画像だけを書いた `![説明](パス)` は画像として表示します（下記）
- **現在の文脈をエージェントへ送る**（`Space a` / `Alt+A` / コマンドパレット）：選択範囲（なければカーソル行）を `パス:行-行`・本文・該当行の診断に整形し、プレビューで確認してからターミナルパネルへ貼り付けます（Enter は押しません）
- **提案パネルと ACP**（`Alt+I` / `Space i`）：git・LSP・ターミナルの状態から「衝突を解消しますか」「PR を作りますか」「失敗の原因を調べますか」などの提案を右のパネルに並べます。選ぶと ACP（Agent Client Protocol）でエージェントに頼み、進み具合と返事をパネルに、変更を差分モードに出します。AI レビューのコメント案は未送信コメントとして置きます
- **Claude Code の IDE 連携**：ターミナルパネルの Claude Code から、選択範囲・開いているファイル・診断を取得し、変更を差分で承認できます（承認後に Claude Code が書き込みます。下記「エージェント連携」）
- 開いているファイルを出すタイトル行、シンタックスハイライト、Quick Open、プロジェクト全体の検索、ターミナルパネル、マウス操作

## インストール

```sh
nix run github:mm1995tk/shijimi-editor/release   # その場で起動
nix profile install github:mm1995tk/shijimi-editor/release
go install github.com/mm1995tk/shijimi-editor/cmd/shj@latest      # shj コマンドとして入れる
go install github.com/mm1995tk/shijimi-editor/cmd/shijimi@latest  # 同じものを shijimi コマンドとして入れる
npm install -g shijimi-editor                                     # shj と shijimi の両方を入れる（Linux と macOS）
```

どれも最新のリリースを入れます。Nix のコマンドは `release` ブランチを、`go install` は一番新しいタグを、npm はリリースのときに publish したバイナリを取ります。同じバイナリを `shj-<os>-<cpu>` として各 [GitHub Release](https://github.com/mm1995tk/shijimi-editor/releases) にも置いています。`main` のリリース前の変更は入りません。

`main` のリリース前の変更を試すには、nightly 版を入れます。`shj` と並べて `shj-nightly`（と `shijimi-nightly`）として入り、設定は `~/.config/shijimi-editor-nightly/` を読み（そのディレクトリが無い間は `~/.config/shijimi-editor/` を読みます）、ソケットのディレクトリも別で、設定の `[experimental]`（試している途中の機能）が既定で有効です。Nix は入れる時点の最新の `main` からビルドします。npm は毎日 publish したビルド（dist-tag `nightly`、版は `X.Y.Z-nightly.<yyyymmdd>.<sha>`）を取り、同じバイナリを `shj-nightly-<os>-<cpu>` として [`nightly` の pre-release](https://github.com/mm1995tk/shijimi-editor/releases/tag/nightly) にも置いています。

```sh
nix run github:mm1995tk/shijimi-editor#shj-nightly
nix profile install github:mm1995tk/shijimi-editor#shj-nightly
npm install -g shijimi-editor-nightly   # shj-nightly と shijimi-nightly を入れる（Linux と macOS）
npx shijimi-editor-nightly              # 入れずにその場で起動
```

プルリクエストの base と比べるには [gh](https://cli.github.com/) と `gh auth login` が必要です。

## 使い方

```sh
shj            # カレントディレクトリを開く
shj ./src      # ディレクトリを開く
shj main.go    # ファイルだけを編集する（単体編集モード。$EDITOR 向け）
```

`shj` の代わりに `shijimi` と打っても同じです（Nix で入れたときは両方入ります）。

ファイルを渡すと単体編集モードになります。ファイルツリー・LSP・ターミナル・差分モード・エージェントを起動せず、`:q` `:wq` で閉じると終了します（詳しくは[インストールと起動](manual/docs/install.ja.md)）。

## 設定

次の順に読み込み、後のものが優先されます（`languages.toml` は Helix と互換があります。Helix から移る人は、`languages.toml` を `~/.config/shijimi-editor/` にコピーしてください）。

1. 組み込みの既定値（gopls / rust-analyzer / typescript-language-server など）
2. `~/.config/shijimi-editor/languages.toml`、`~/.config/shijimi-editor/config.toml`
3. プロジェクトの `.shijimi-editor/languages.toml`、`.shijimi-editor/config.toml`

`~/.config` は、`$XDG_CONFIG_HOME` があればその場所です。macOS も含めて、OS を問わずこの場所から読みます（`~/Library/Application Support` からは読みません）。

### 画面の言語

画面の文字（ステータス行・コマンドパレット・提案パネル・エラーメッセージ・`shj --help`）の言語は、既定が英語です。`config.toml` の `ui-language` で切り替えます。

```toml
ui-language = "ja"   # "en"（既定）または "ja"
```

`LANG` などの環境変数からは選びません。`en` と `ja` 以外の値は無視します。`:config-reload` で再起動なしに反映します。

### テーマ

`config.toml` の `theme` か、実行中の `:theme <名前>`（名前を省略すると一覧）で切り替えます。コマンドパレットの「設定: テーマを切り替え」からも選べます。

- 組み込み: `github-dark`（既定）、`dark-plus`、`light-plus`
- Helix の `github_dark*` は `github-dark`、`github_light*` と `light` は `light-plus` に対応付けます

```toml
theme = "github_light"
```

### ポート転送の待ち受けアドレス

`:browser forward` の待ち受けアドレスを設定します。

```toml
[browser]
listen-host = "127.0.0.1"
```

### エージェント（提案パネル）

提案を実行する ACP エージェントのコマンドを、`~/.config/shijimi-editor/config.toml` に書きます（プロジェクトの設定からは読みません）。

```toml
[agent]
command = "claude-agent-acp"
args = []
```

### LSP の設定例（`.shijimi-editor/languages.toml`）

```toml
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

対応しているキー：`[language-server.*]` の `command` `args` `config` `environment`、`[[language]]` の `name` `language-id` `file-types` `roots` `language-servers` `indent.unit`。

### エディタの設定（`config.toml`）

```toml
keymap = "helix"          # "vscode" で VS Code 風の操作

[editor]
line-number = "relative"  # 行番号の表示
file-icons = true         # ファイルツリーの Nerd Font アイコン。false で ▸/▾ 表示
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"  # mermaid を PNG にする外部コマンド。空で無効
watch-files = true        # 他のプロセス（AI エージェントなど）によるファイル変更を検知して反映。未保存の編集があるファイルは :reload を待つ
mouse-hover = true        # エディタ上の単語にマウスを 0.5 秒乗せると LSP のホバーを表示。false で無効（動きだけのマウスイベントも受け取らなくなる）

# インデントガイド（インデントごとの薄い縦線）。false で無効
[editor.indent-guides]
render = true

# 自動保存（既定で有効。編集が 1 秒止まると保存。名前のないファイルは対象外）
[editor.auto-save.after-delay]
enable = true
timeout = 1000            # ミリ秒
```

### Nix で書く

各ディレクトリの `config.toml` / `languages.toml` と同じ場所に `config.nix` / `languages.nix` を置くと、`nix eval --json --file` で評価して TOML と同じ構造として読みます（同じディレクトリでは TOML のあとに適用）。`nix` が無い、または評価に失敗したファイルは無視します。

```nix
# ~/.config/shijimi-editor/languages.nix
{
  language-server.nil.command = "nil";
  language = [
    { name = "nix"; language-servers = [ "nil" ]; }
  ];
}
```

### home-manager

flake の `homeManagerModules.default` を読み込みます。`settings` が `config.toml`、`languages` が `languages.toml` として `~/.config/shijimi-editor/` に書き出されます。

```nix
{
  # inputs.shijimi.url = "github:mm1995tk/shijimi-editor/release";
  imports = [ inputs.shijimi.homeManagerModules.default ];
  programs.shijimi-editor = {
    enable = true;
    settings = { keymap = "vscode"; editor.line-number = "relative"; };
    languages = {
      language-server.gopls.command = "gopls";
      language = [ { name = "go"; language-servers = [ "gopls" ]; } ];
    };
  };
}
```

herdr を使う場合は、`programs.shijimi-editor.herdr.enable = true` にすると、`programs.shijimi-editor.herdr.keysCommand` が herdr の `[[keys.command]]` 用のリストになります。`herdr.hideKey`（既定 `prefix+t`）のキーで `shj exec ':sh hide'`（ペインを隠す）を実行します。shijimi-editor は herdr の `config.toml` を書きません。このリストを自分の herdr の設定に差し込んでください。`herdr.enable` が `false` の間は空のリストです。

```nix
programs.shijimi-editor.herdr.enable = true;
xdg.configFile."herdr/config.toml".source = toml.generate "herdr-config.toml" {
  # ...自分の herdr の設定...
  keys.command = config.programs.shijimi-editor.herdr.keysCommand;
};
```

### Markdown プレビューと mermaid

`.md` ファイルで `Space m`（または `:preview`、コマンドパレット）を押すと、バッファの右にプレビューが開きます。編集に追従し、カーソル位置に合わせてスクロールします。もう一度押すとプレビューだけを全幅で表示し、3 回目で閉じます。全幅のときは `j` / `k` やマウスホイールでスクロールします。

mermaid の図は [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)（`mmdc`）で PNG にして表示します。`mmdc` が PATH に無い場合、または描画に失敗した場合は、図の代わりにソースとエラーを表示します。`mermaid-command` で別のコマンドに差し替えられます（`{in}` が入力、`{out}` が出力 PNG）。描画結果は `~/.cache/shijimi/mermaid` に保存します。

1 行に画像だけを書いた `![説明](パス)` も、同じように画像として表示します。パスは Markdown ファイルからの相対パスです。文の途中の画像と、`https://` などのファイルでない画像は文字で表示します。

## キー操作（Helix モード）

### ノーマルモード

| キー | 動作 |
| --- | --- |
| `h j k l` | 移動（数字を前に付けると回数指定） |
| `w` `b` `e` | 単語を選択しながら移動 |
| `x` | 行を選択（続けて押すと広がる） |
| `%` / `;` | すべて選択 / 選択解除 |
| `v` | 選択モード |
| `d` `c` `y` | 削除 / 変更 / ヤンク |
| `p` `P` | 後ろ / 前に貼り付け |
| `i` `a` `I` `A` `o` `O` | 挿入モードへ |
| `u` `U` | 元に戻す / やり直し |
| `f` `t` `F` `T` + 文字 | 行内で文字を探す |
| `r` + 文字 | 置き換え |
| `>` `<` | インデント |
| `J` | 行を連結 |
| `mm` | 対応する括弧へ |
| `gg` `ge` `gh` `gl` `gs` / `数字g` | ジャンプ |
| `gd` `gr` | 定義へ移動 / 参照 |
| `Ctrl+O` `Ctrl+I` | 飛ぶ前の場所へ戻る / 進む（VS Code モードは `Alt+←` `Alt+→`） |
| `Ctrl+Alt+D` `Ctrl+Alt+U` | カーソルを元の行に残したまま画面だけを下 / 上へスクロール（動き方は `Ctrl+D` / `Ctrl+U` と同じ。キーを離したことを伝える端末では押している間 1 行ずつ、それ以外は 1 回で半画面）。画面の端に来たらカーソルは端に留まり、押し出される |
| `]d` `[d` | 次 / 前の診断 |
| `ga` | 直前に開いていたファイルと今のファイルを行き来（VS Code モードでは `Ctrl+Tab`） |
| `/` `?` `n` `N` `*` | 検索 |
| `Ctrl+C` | 行コメントの切替 |
| `:` | コマンド（下の表） |

### Space メニュー

`Space` を押すと、続けて押せるキーの一覧がエディタ領域の右下にポップアップで出ます。`Esc` か割り当てのないキーで閉じます。

| キー | 動作 |
| --- | --- |
| `Space f` | ファイルを開く |
| `Space b` | 開いているバッファ |
| `Space w` | ウィンドウ（分割）メニュー: `v` エディタを左右 2 つに分ける、`h` `l` `w` フォーカスを移す、`q` `o` 分割を閉じる |
| `Space e` | ファイルツリーへ移動 |
| `Space k` | ホバー。表示中は `Ctrl+D` / `Ctrl+U`（`PgDn` / `PgUp`・ホイール）でスクロール、`y` または枠の右上の `[コピー]` クリックで markdown の原文をコピー |
| `Space o` | カーソル位置の URL を外部ブラウザで開く（Linux: `xdg-open` / macOS: `open`） |
| `Space r` | 名前の変更 |
| `Space .` | コードアクション（診断のヒントを適用する） |
| `Space s` / `Space S` | シンボル一覧（ファイル内 / プロジェクト全体） |
| `Space d` | 診断の一覧 |
| `Space /` | プロジェクト全体を検索・置換（フローティング窓。「全文検索」の節を参照） |
| `Space g` | 差分モード（比較元はプルリクエストの base）を開く / 閉じる |
| `Space m` | Markdown プレビューの切替（並べて表示 → 全幅 → 閉じる） |
| `Space G` | 差分モード（比較元は `HEAD`）を開く / 閉じる |
| `Space v` | レビューのメニュー（比較元がプルリクエストの base のとき） |
| `Space y` `Space p` | システムのクリップボードへコピー / 貼り付け |
| `Space ?` | コマンドパレット |

### 挿入モード

`Esc` でノーマルモードへ戻ります。補完は `.` などの入力や 2 文字入力で自動的に出ます（`Ctrl+X` で手動）。`Tab` / `Enter` で確定します。スニペット補完は穴が選択された状態で入り、`Tab` / `Shift+Tab` で穴を移れます。

### `:` コマンド

`:w` `:w <パス>` `:wa` `:q` `:q!` `:wq` `:wqa` `:o <パス>` `:n` `:bc` `:bc!` `:fmt` `:reload` `:<行番号>` `:sh <コマンド>` `:grep [-x 除外glob] [-i 含めるglob] <文字列>` `:replace [-x 除外glob] [-i 含めるglob] <文字列> => <置換後>` `:pr [番号]` `:gitdiff [on|off|split|unified]` `:preview` `:lsp-restart` `:config-reload` `:browser forward|list|stop` `:keymap [helix|vscode]` `:open-url [URL]`（省略時はカーソル位置の URL）

### 全体

| キー | 動作 |
| --- | --- |
| `Ctrl+P` | ファイルを開く |
| `F1` / `Ctrl+K` | コマンドパレット |
| `Ctrl+B` | ファイルツリーの表示切替 |
| `Ctrl+E` | ファイルツリーとエディタのフォーカス切替 |
| `Ctrl+G` | 差分モード（比較元は `HEAD`）を開く / 閉じる |
| ``Ctrl+` `` / `Ctrl+J` | ターミナルの表示切替（ターミナルの中では ``Ctrl+` `` だけ） |
| `Alt+R` | ペインのリサイズモード（`←→` `↑↓`（`hjkl`）でフォーカスのあるペインの境界を動かす、`<` `>` エディタとプレビューの境界、`=` 初期値、`Esc` で終了）。境界線のマウスドラッグでも変えられます。大きさはエディタを終了するまで保たれます |
| `Ctrl+S` | 保存 |
| `Ctrl+W` | 今のファイルを閉じる（開いたファイルの一覧から外す） |
| `Ctrl+Tab` | 直前に開いていたファイルと今のファイルを行き来。kitty keyboard protocol に対応した端末が必要で、そうでない端末では Helix モードの `ga` か `Space b` を使います |
| `Ctrl+\` | エディタを左右 2 つの欄に分割する / 分割を閉じる（`Ctrl+1` / `Ctrl+2` でフォーカスを移す）。Helix モードでは `Space w v`、`Space w h` `l` `w`、`Space w q` `o`。`:vsplit` と `:only` でも。欄ごとにタイトル行が出て、`Space b`、`ga` / `Ctrl+Tab`、`Ctrl+O` / `Ctrl+I` はフォーカス中の欄に働く。最大 2 つの欄を左右に並べるだけで、同じファイルを両方には出さない。差分モードでは使えない（1 つの欄になる） |
| `Ctrl+Q` | 終了 |

### ファイルツリー

`j/k` 移動、`Enter` 開く、`h/l` 閉じる / 展開、`a` 新しいファイル、`A` 新しいフォルダー、`r` 名前の変更、`d` 削除、`y` 相対パスをコピー、`Y` 絶対パスをコピー、`R` 更新、`Esc` エディタへ。

### 差分モード

開き方で、何と比べるか（比較元）が決まります。開いているときに同じキーを押すと閉じ、違うキーを押すと比較元が切り替わります。

| キー | 比較元 |
| --- | --- |
| `Space G` / `Ctrl+G` / `:gitdiff` | `HEAD`（未追跡ファイルを含む） |
| `Space g` / `:pr [番号]` | プルリクエストの base（`git merge-base <PR の base> HEAD`）。今のブランチの PR を開く。違うブランチの PR は `gh pr checkout` してから開く（`gh` が必要） |

- 画面は上から、ヘッダー（比較元と比較先）、サイドバーと差分、ヘルプ行（今使えるキー）、ステータス行の順です。
- サイドバーが上下 7:3 に分かれ、下の 3 に **CHANGES**（変更ファイルのツリー）が出ます。子が 1 つだけのフォルダは `internal/config` のようにまとめます。`Tab` でファイルツリーと行き来します。
- 変更のないファイルは通常のエディターで開きます。変更のあるファイルは、ファイル全体を **Split**（左が比較元、右が今のファイル）で開きます。新規ファイル（未追跡を含む）は、エディター 1 枚で開きます。行に色は付けず、右端のバーは全体が緑になり、上のファイル名の行の `+N` / `-N` は残ります。
- 右側（今のファイル）は通常どおり編集・保存できます。編集すると diff がすぐに更新されます。
- **Unified** にも切り替えられます（`s`、`:gitdiff unified` / `split`、ファイル名の行のクリック）。
- 差分の右端のバーに、変更の位置を色で出します（緑：追加、赤：削除、青：書き換え）。今表示している範囲は明るくなり、クリックするとその位置へスクロールします。
- 行の一部だけが変わった行は、変わった部分を濃い色で強調します。
- プルリクエストの base と比べているときは、`Space v` でレビューできます（行・範囲・ファイルへのコメント、返信、解決、suggestion の適用、viewed、承認などの送信、`Space v y` / `Space v o` での PR の URL のコピーと外部ブラウザで開く操作）。コメントは、作業ツリーの中身が PR の最新コミットと同じファイルにだけ書けます。詳しくはマニュアルの「差分モード」を参照してください。

| キー（CHANGES にフォーカス中） | 動作 |
| --- | --- |
| `j/k` | 移動 |
| `Enter` / `l` | ファイルを開く |
| `s` | 分割 / 統合の切替 |
| `r` | 一覧の更新 |
| `Tab` | ファイルツリーへ |
| `Esc` | エディタへ |

## 制限

- ターミナルパネルは PTY 上のログインシェル（`$SHELL -l`）です。`.zshrc` などが読まれ、`vim` / `top` / `lazygit` も動きます。フォーカス中はほぼ全てのキーがシェルへ渡ります（グローバルは `Ctrl+Q` / ``Ctrl+` `` / `F1` / `Alt+R` のみ）。`Ctrl+J` はシェルへ渡るので、Shift+Enter を LF で送る端末でも Claude Code で改行できます。ターミナルは 1 つだけです（複数ほしいときは herdr や tmux などのマルチプレクサを使います）。シェルが終了するとパネルが閉じ、次に開いたときに新しいシェルが起動します。マウスのホイールで流れた出力をさかのぼれます。
- 画像と mermaid の図は Kitty graphics protocol で描きます。対応した端末（Kitty / Ghostty / WezTerm など）が必要です。

## 開発

Nix の devShell（Go・gopls・gomod2nix・git）を使います。direnv があれば `.envrc` で自動で入ります。

```sh
nix develop            # または direnv allow
go test ./...
go build ./cmd/shj
nix build              # パッケージをビルド（テストも実行）
scripts/tidy.sh        # go mod tidy / gomod2nix / go fix / nix fmt
```

push の前に `scripts/tidy.sh` を実行してください（devShell の外では `nix run .#tidy` で同じことができます）。CI も同じスクリプトを動かし、差分が出たら失敗します。`go.mod` を変えたときの `gomod2nix.toml` の更新も、このスクリプトがします。

画面に出る文字列は、言語ごとの表 [`internal/i18n/en.go`](internal/i18n/en.go) と [`internal/i18n/ja.go`](internal/i18n/ja.go) にまとめています。コードは `i18n.T("キー")` で取り出し、画面の文字を直接は書きません。文字列を足すときは両方の表に同じキーを足してください（そろっているかはテストで確かめます）。

### 全文検索・置換

`Space /`、コマンドパレットの「検索: プロジェクト全体を検索 / 置換」、`:grep`、`:replace` はどれも同じフローティング窓（画面の上下左右 1 桁を残してほぼ全画面）を開きます。上部に条件（検索・置換・含める・除外の 4 欄を枠線付きの箱で 2 列に並べ、選択中の欄は枠線の色が変わります。その下にトグル）、左下に一致した行（Matched）、右下に選択行の前後（View）を出します。条件を変えると短い待ち時間のあと自動で検索し直します。置換欄に文字があると、View に `-` / `+` の差分も出ます。

| キー | 動作 |
| --- | --- |
| `Tab` / `Shift+Tab` | 次 / 前の欄へ |
| `↑` `↓` / `Ctrl+P` `Ctrl+N` / `PgUp` `PgDn` | 一致した行を選ぶ |
| `Enter` | 選んだ行をそのファイルで開く |
| `Ctrl+X`（`Alt+X`） | 選んだ行を置換の対象から外す / 戻す（外した行は `x` で表示） |
| `Alt+C` / `Alt+W` / `Alt+R` / `Alt+P` | 大文字小文字の区別 / 単語単位 / 正規表現 / 置換で大文字小文字を保持 |
| `Alt+1` | 選んだ行だけを置換（パネルは閉じず、次の一致へ進む） |
| `Alt+Enter` | 対象の行を全置換 |
| `Esc` | 閉じる |

- 全置換は対象のファイルをバッファとして開いて置換します。保存するまでディスクは変わらず、`u` / `Ctrl+Z` で元に戻せます。
- 正規表現が有効なときは、置換欄で `$1` などのキャプチャを使えます。無効なときの `$` はそのままの文字です。
- 検索結果が 5000 件を超えたときは、条件を絞るまで全置換できません。
- 検索対象は Git の管理下（未追跡で ignore されていないものを含む）のテキストファイルです。Git の外ではビルド用ディレクトリを除いて探します。開いているバッファの未保存の内容も検索します。
- `:grep -x '*.lock,dist/**' -i '*.go' foo` と `:replace foo => bar` の引数は、各欄に入れた状態で窓を開きます。`:grep` / `:replace` だけを打つと前回の条件のまま開きます。

### 全文検索の除外・包含

「含める」「除外」欄にはカンマ区切りの glob を書きます（例: `*.lock,dist/**`）。`:grep` / `:replace` では `-x` / `--exclude=`（除外）と `-i` / `--include=`（包含）でも指定できます。初期値は `[search]` 設定の値です。常に効かせたい指定は `~/.config/shijimi-editor/config.toml` か `.shijimi-editor/config.toml` に書きます。

```toml
[search]
exclude = ["*.lock", "dist/**"]
include = []
```

### ポート転送（`:browser forward`）

`:browser forward <対象ポート> [待ち受けポート]` は、shijimi が `127.0.0.1` で待ち受け、`localhost:<対象ポート>` へ TCP で転送します（HTTP も WebSocket も通ります）。待ち受けポートを省略すると空いているポートが選ばれます。`:browser list` で一覧、`:browser stop <待ち受けポート>` で停止します。shijimi を終了すると全て止まります。

リモートの VM で shijimi を動かしているときは、手元で次を実行すると、手元のブラウザから `http://localhost:8080` で開発サーバーを見られます。

```sh
ssh -L 8080:127.0.0.1:<待ち受けポート> vm
```

直接 `ssh -L` するのに比べて、待ち受けポートを固定できること、`0.0.0.0` や IPv6 のみで待ち受けるサーバーも `127.0.0.1` で受けられることが利点です。これらが不要なら `ssh -L` を直接使ってください。

待ち受けアドレスは `[browser]` の `listen-host` で変えられます（設定は「ポート転送の待ち受けアドレス」節）。既定では `0.0.0.0` にバインドしません。信頼できるネットワークでのみ変えてください。

## エージェント連携

### Claude Code の IDE 連携

shijimi は Claude Code の IDE 連携プロトコル（VS Code 拡張や claudecode.nvim と同じもの）に対応しています。起動すると 127.0.0.1 の空きポートで WebSocket の MCP サーバーを立て、ロックファイル `~/.claude/ide/<port>.lock`（0600、`CLAUDE_CONFIG_DIR` があればその下の `ide/`）に接続先と認証トークンを書きます。終了時に消します。

shijimi のターミナルパネルで `claude` を起動すると、環境変数 `CLAUDE_CODE_SSE_PORT` `ENABLE_IDE_INTEGRATION=true` `FORCE_CODE_TERMINAL=true` により自動で接続します。外のターミナルで動く Claude Code は、作業ディレクトリが shijimi のルート配下なら `/ide` で選べます。

エージェントが使える道具:

| 道具 | 内容 |
|------|------|
| `getCurrentSelection` / `getLatestSelection` | 選択範囲（なければカーソル位置）とファイル |
| `getOpenEditors` / `getWorkspaceFolders` | 開いているファイル / ルート |
| `getDiagnostics` | LSP の診断（URI 省略で全ファイル） |
| `openFile` | ファイルを開く（`startText` / `endText` で範囲選択） |
| `openDiff` | 変更を差分で表示。**Enter で承認、Esc で却下**。ファイルは承認後に Claude Code が書き込みます |
| `closeAllDiffTabs` / `close_tab` | 待っている差分を却下して閉じる |
| `checkDocumentDirty` / `saveDocument` | 未保存の確認 / 保存 |

- 選択範囲やカーソルが動くたびに `selection_changed` を送ります。
- 接続中は `Space a` / `Alt+A`（現在の文脈をエージェントへ送る）が貼り付けではなく `at_mentioned` を送ります。接続していないときは、これまでどおりプレビューを出してターミナルへ貼り付けます。
- 認証トークンの無い接続やブラウザ（`Origin` ヘッダー付き）からの接続は拒否します。

### 制御用ソケット

実行中の shijimi は Unix ソケット（0600）を作り、環境変数 `SHIJIMI_SOCKET` で場所を伝えます。ターミナルパネル内のシェルから次を実行できます。IDE 連携に無い操作だけを残しています。

`SHIJIMI_SOCKET` が無いとき（ターミナルマルチプレクサ herdr のキーに割り当てたシェルコマンドなど。herdr は自分の変数しか渡しません）は、`shj` が起動中の shijimi を自分で探します。ソケットは `$XDG_RUNTIME_DIR/shijimi-<uid>/<pid>.sock`（nightly 版は `shijimi-nightly-<uid>`。`XDG_RUNTIME_DIR` が無ければ一時ディレクトリの下）にあり、herdr の中（`HERDR_ENV=1`）で起動した shijimi は、同じディレクトリに `herdr-<HERDR_WORKSPACE_ID>/<HERDR_PANE_ID>.sock` というリンクも作ります。herdr のワークスペースの中（`HERDR_WORKSPACE_ID` あり）の `shj` は、自分のペイン `HERDR_PANE_ID` のリンクか、そのワークスペースで 1 つだけ起動している shijimi を使います。herdr の外では、そのユーザーで 1 つだけ起動している shijimi を使います。接続できなくなったソケットは飛ばし、shijimi は終了時に自分のファイルを消します。見つからないときと複数あるときは、`shj` はエラーを書いて終了コード 1 で終わります（`SHIJIMI_SOCKET` で 1 つを指定してください）。

- `shj diff [path]`: 差分モード（比較元 `HEAD`）にする。`path` を付けるとそのファイルを diff 表示で開く
- `shj exec ":<コマンド>"`: `:` コマンドをそのまま実行する（例: `shj exec ":preview"`）。成功するとステータス欄の文を標準出力へ、失敗するとエラーの文を標準エラーへ書き、終了コード 1 にする。`:q` `:q!` `:wq` `:bc!` などの終了・破棄するコマンド、引数なしで選択画面を開くコマンド（`:o` `:theme` `:grep` `:replace` `:symbol` `:action`、番号以外の引数の `:pr`）、未保存の変更があるファイルの `:reload` と `:bc` は実行せずエラーを返す

第 1 引数が `diff` / `exec` / `command-end` / `skill` のときはサブコマンドとして扱います。同名のファイルは `./diff` のように書きます。

`shj skill install [書き出し先]` は、このコマンドの使い方を書いたスキル（`SKILL.md`）を書き出します。書き出し先を省略すると `~/.claude/skills/shijimi/` に書きます。

> `shj open` と `shj status` は IDE 連携（`openFile` / `getOpenEditors` / `getCurrentSelection`）に置き換えて削除しました。

### ターミナルマルチプレクサの中で

ターミナルマルチプレクサの中で shijimi を動かしていて、`config.toml` の `[experimental]` の `multiplexer` が `true`（既定値は nightly 版が `true`、安定版が `false`。古いキー `herdr` も同じキーとして読みます）で、そのマルチプレクサに対応しているとき、``Ctrl+` ``・`Space t`・`:sh`・`:sh <コマンド>`・`:run`・`:!` はターミナルパネルの代わりにマルチプレクサのペインを使います。ペインの中で動かしたエージェントの状態検出・通知・復元はマルチプレクサに任せられます。

どのマルチプレクサを使うかは `[terminal]` の `multiplexer` で決めます。値は `"auto"`（既定）・`"herdr"`・`"tmux"`・`"zellij"`・`"cmux"`・`"none"` です。`"auto"` は shijimi の親プロセスをたどり、最初に見つかったマルチプレクサ（入れ子のときは一番内側。たとえば herdr の中の tmux なら tmux）を使います。見つからなければターミナルパネルです。`"none"` はいつもターミナルパネルを使います。今は herdr だけに対応していて、`"tmux"`・`"zellij"`・`"cmux"` のときはメッセージを出してターミナルパネルを使います。対応するマルチプレクサが無いとき、または `multiplexer = false` のときは、今までどおりターミナルパネルを使います。

この節の以降は [herdr](https://herdr.dev)（`HERDR_ENV=1`）の説明です。

- 初回は、shijimi のペインの真下（`herdr pane split`）にワークスペースの root でペインを作り、`SHIJIMI_SOCKET` と IDE 連携の環境変数を渡して、焦点を移します。2 回目以降は同じペインを再表示し、終わっていたら新しく作ります。もう一度キーを押すと隠れ（shijimi のペインをズーム）、ペインの中で `shj exec ":sh hide"` を実行しても隠れます。`Ctrl+J` はペインを表示して焦点を移します。戻るには herdr のキー（既定は `Ctrl+B` `k`）を使います。
- `:sh <コマンド>` はペインでコマンドを実行します。`Space a` / `Alt+A` はペインに文脈を Enter 無しで貼ります（IDE 連携がつながっているときの `at_mentioned` は今のままです）。
- ペインのシェル（zsh、bash）は、終了コードを `shj command-end <終了コード>` で知らせるので、失敗したコマンドを調べる提案は今までどおり出ます。ペインは `exit` か herdr の `Ctrl+B x` で終わり、shijimi は終了時に自分が作ったペインを閉じます。
- ペインの画面・スクロール・マウスでのコピー・OSC 52 は herdr のもので、shijimi 自身のキー（`Ctrl+Q`・``Ctrl+` ``・`Alt+R`・`F1`）はペインの中では効きません。

## 説明書とホームページの編集

説明書は [MkDocs Material](https://squidfunk.github.io/mkdocs-material/) で `manual/` から作ります。ホームページは `site/` の素の HTML です。`main` に push すると GitHub Actions が両方を GitHub Pages に公開します。ホームページは一番上に、説明書は `/manual/` の下に置きます。説明書を手元で確認するには次を実行します。

```sh
pip install -r manual/requirements.txt
mkdocs serve --config-file manual/mkdocs.yml
```

説明書は、ページごとに英語（`名前.en.md`、既定）と日本語（`名前.ja.md`）を置きます。片方を変えたら、もう片方も変えてください。`manual/check-translations.sh` が、全ページに両方の言語があることと構造が同じことを確かめ、CI がプルリクエストで実行します（片方の言語だけが変わったときも失敗します。誤字の修正など、翻訳が要らないときは、プルリクエストのタイトルに `[skip-i18n-check]` を入れるとその部分を飛ばせます）。

ホームページのスクリーンショットとデモ動画（`site/images/`）は [vhs](https://github.com/charmbracelet/vhs) で撮ります。画像と mermaid の図は Kitty graphics protocol が要るので、kitty で撮ります。画面を変えたら `site/screenshots/record.sh`（Nix が要ります）を実行して撮り直してください。場面は `site/screenshots/tapes/`（vhs）と `site/screenshots/kitty/`（kitty）に書いてあります。

## ライセンス

[MIT](LICENSE)
