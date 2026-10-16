# gh-editor

ターミナルで動く統合開発環境 (TUI) です。画面の作りは VS Code、編集操作は Helix に合わせています。

## 機能

- **Helix 風のモード編集**（ノーマル / 挿入 / 選択、`:` コマンド）。VS Code 風の操作にも切り替えられます
- **マルチカーソル**（Helix 風）：`C` / `Alt+C` で下 / 上の行に複製、`s` で選択範囲を正規表現で分割、`,` で 1 つに戻す。挿入・削除・`c` などは全カーソルに適用
- **ファイルツリー**（Git の変更状態を M / U / A で表示）
- **LSP**：診断・補完・ホバー・定義へ移動・参照・名前の変更・整形
- **LSP の設定は Helix と同じ `.helix/languages.toml`** をプロジェクトのルートから読み込みます
- **プルリクエストのレビュー**（gh CLI を使用）：差分の表示・行へのコメント・承認・変更を要求
- **ローカルの変更のレビュー**（hunk 風）：`git diff HEAD` と未追跡ファイルをハンク単位で確認し、コメントを Markdown でコピー（`y`）、またはプレビューで確認してターミナルのエージェントへ送る（`a`）
- **画像のプレビュー**（PNG / JPEG / GIF / BMP / WebP / TIFF）
- **Markdown のプレビュー**（`Space m` / `:preview`）：見出し・リスト・コード・表を整形し、編集中のバッファの右に並べて表示します。mermaid のコードブロックは図として表示します（下記）
- **現在の文脈をエージェントへ送る**（`Space a` / `Alt+A` / コマンドパレット）：選択範囲（なければカーソル行）を `パス:行-行`・本文・該当行の診断に整形し、プレビューで確認してからターミナルパネルへ貼り付けます（Enter は押しません）
- タブ、シンタックスハイライト、Quick Open、プロジェクト全体の検索、ターミナルパネル、マウス操作

## インストール

```sh
nix run github:mm1995tk/gh-editor           # その場で起動
nix profile install github:mm1995tk/gh-editor
go install github.com/mm1995tk/gh-editor@latest
```

PR レビューには [gh](https://cli.github.com/) と `gh auth login` が必要です。

## 使い方

```sh
gh-editor            # カレントディレクトリを開く
gh-editor ./src      # ディレクトリを開く
gh-editor main.go    # ファイルを開く
```

## 設定

次の順に読み込み、後のものが優先されます（形式は Helix と同じです）。

1. 組み込みの既定値（gopls / rust-analyzer / typescript-language-server など）
2. `~/.config/helix/languages.toml`、`~/.config/helix/config.toml`
3. `~/.config/gh-editor/languages.toml`、`~/.config/gh-editor/config.toml`
4. **プロジェクトの `.helix/languages.toml`、`.helix/config.toml`**
5. プロジェクトの `.gh-editor/languages.toml`、`.gh-editor/config.toml`

### テーマ

`config.toml` の `theme`（Helix と同じ書き方）か、実行中の `:theme <名前>`（名前を省略すると一覧）で切り替えます。コマンドパレットの「設定: テーマを切り替え」からも選べます。

- 組み込み: `dark-plus`（既定）、`github-dark`、`light-plus`
- Helix の `github_dark*` は `github-dark`、`github_light*` と `light` は `light-plus` に対応付けます

```toml
theme = "github_dark"
```

### ブラウザ（Web ページの画像化）

Web ページはヘッドレス Chromium で描画して PNG にします。`chromium` / `chromium-browser` / `google-chrome` を PATH から探します。実行ファイルのパスは次のように上書きできます。

```toml
[browser]
chromium = "/usr/bin/chromium"
```

`[browser]` は `~/.config/gh-editor/config.toml` か `.gh-editor/config.toml` にのみ書いてください。`.helix/config.toml` に書くと、Helix が未知のセクションを拒否します（`[search]` と同じ）。

### LSP の設定例（`.helix/languages.toml`）

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
line-number = "relative"  # Helix と同じ
file-icons = true         # ファイルツリーの Nerd Font アイコン。false で ▸/▾ 表示
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"  # mermaid を PNG にする外部コマンド。空で無効
watch-files = true        # 他のプロセス（AI エージェントなど）によるファイル変更を検知して反映。未保存の編集があるファイルは :reload を待つ

# 自動保存（既定で有効。編集が 1 秒止まると保存。名前のないファイルは対象外）
[editor.auto-save.after-delay]
enable = true
timeout = 1000            # ミリ秒
```

### Nix で書く

各ディレクトリの `config.toml` / `languages.toml` と同じ場所に `config.nix` / `languages.nix` を置くと、`nix eval --json --file` で評価して TOML と同じ構造として読みます（同じディレクトリでは TOML のあとに適用）。`nix` が無い、または評価に失敗したファイルは無視します。

```nix
# ~/.config/gh-editor/languages.nix
{
  language-server.nil.command = "nil";
  language = [
    { name = "nix"; language-servers = [ "nil" ]; }
  ];
}
```

### home-manager

flake の `homeManagerModules.default` を読み込みます。`settings` が `config.toml`、`languages` が `languages.toml` として `~/.config/gh-editor/` に書き出されます。

```nix
{
  imports = [ inputs.gh-editor.homeManagerModules.default ];
  programs.gh-editor = {
    enable = true;
    settings = { keymap = "vscode"; editor.line-number = "relative"; };
    languages = {
      language-server.gopls.command = "gopls";
      language = [ { name = "go"; language-servers = [ "gopls" ]; } ];
    };
  };
}
```

### Markdown プレビューと mermaid

`.md` ファイルで `Space m`（または `:preview`、コマンドパレット）を押すと、バッファの右にプレビューが開きます。編集に追従し、カーソル位置に合わせてスクロールします。もう一度押すと閉じます。

mermaid の図は [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)（`mmdc`）で PNG にして表示します。`mmdc` が PATH に無い場合、または描画に失敗した場合は、図の代わりにソースとエラーを表示します。`mermaid-command` で別のコマンドに差し替えられます（`{in}` が入力、`{out}` が出力 PNG）。描画結果は `~/.cache/gh-editor/mermaid` に保存します。

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
| `]d` `[d` | 次 / 前の診断 |
| `gn` `gp` | 次 / 前のタブ |
| `/` `?` `n` `N` `*` | 検索 |
| `Ctrl+C` | 行コメントの切替 |
| `:` | コマンド（下の表） |

### Space メニュー

| キー | 動作 |
| --- | --- |
| `Space f` | ファイルを開く |
| `Space b` | 開いているバッファ |
| `Space e` | ファイルツリーへ移動 |
| `Space k` | ホバー |
| `Space o` | カーソル位置の URL を外部ブラウザで開く（Linux: `xdg-open` / macOS: `open`） |
| `Space r` | 名前の変更 |
| `Space d` | 診断の一覧 |
| `Space /` | プロジェクト全体を検索 |
| `Space g` | プルリクエスト一覧 → レビュー |
| `Space m` | Markdown プレビューの切替 |
| `Space G` | ローカルの変更をレビュー |
| `Space y` `Space p` | システムのクリップボードへコピー / 貼り付け |
| `Space ?` | コマンドパレット |

### 挿入モード

`Esc` でノーマルモードへ戻ります。補完は `.` などの入力や 2 文字入力で自動的に出ます（`Ctrl+X` で手動）。`Tab` / `Enter` で確定します。

### `:` コマンド

`:w` `:w <パス>` `:wa` `:q` `:q!` `:wq` `:wqa` `:o <パス>` `:n` `:bc` `:bc!` `:fmt` `:reload` `:<行番号>` `:sh <コマンド>` `:grep [-x 除外glob] [-i 含めるglob] <文字列>` `:replace [-x 除外glob] [-i 含めるglob] <文字列> => <置換後>` `:pr [番号]` `:review` `:preview` `:lsp-restart` `:config-reload` `:keymap [helix|vscode]` `:open-url [URL]`（省略時はカーソル位置の URL）

### 全体

| キー | 動作 |
| --- | --- |
| `Ctrl+P` | ファイルを開く |
| `F1` / `Ctrl+K` | コマンドパレット |
| `Ctrl+B` | ファイルツリーの表示切替 |
| `Ctrl+E` | ファイルツリーとエディタのフォーカス切替 |
| `Ctrl+G` | ローカルの変更をレビュー |
| `Ctrl+J` | ターミナルの表示切替 |
| `Ctrl+S` | 保存 |
| `Ctrl+W` | タブを閉じる |
| `Ctrl+PgUp` / `Ctrl+PgDn` / `Alt+1`〜`9` | タブの切替 |
| `Ctrl+Q` | 終了 |

### ファイルツリー

`j/k` 移動、`Enter` 開く、`h/l` 閉じる / 展開、`a` 新しいファイル、`A` 新しいフォルダー、`r` 名前の変更、`d` 削除、`R` 更新、`Esc` エディタへ。

### レビュー画面

| キー | 動作 |
| --- | --- |
| `j/k` | 移動 |
| `]` `[` | 次 / 前のハンク |
| `J` `K` | 次 / 前のファイル |
| `Tab` / `h` `l` | ファイル一覧と差分の切替 |
| `s` | 左右分割 / 1 列の切替 |
| `v` | 確認済みにする |
| `c` | この行にコメント（`\n` で改行） |
| `d` | 未送信のコメントを削除 |
| `y` | コメントを Markdown でコピー（エージェントに渡す用） |
| `Enter` | ファイルをエディタで開く |
| `A` / `R` / `S` | 承認 / 変更を要求 / コメントのみで送信（PR のみ） |
| `C` | `gh pr checkout` |
| `r` | 再読み込み |
| `q` | 閉じる |

## 制限

- ターミナルパネルは PTY 上のログインシェル（`$SHELL -l`）です。`.zshrc` などが読まれ、`vim` / `top` / `lazygit` も動きます。フォーカス中はほぼ全てのキーがシェルへ渡ります（グローバルは `Ctrl+Q` / `Ctrl+J` / `F1` のみ）。`F2` で新しいタブ、`Ctrl+PageUp/PageDown` でタブ切替、シェルが終了するとタブが閉じます。スクロールバックと全角文字の正確な桁計算は未対応です。
- 画像は文字（▀）で表示します。細かい部分は粗くなります。
- 複数カーソル（Helix の `C` `s`）は未対応です。

## 開発

Nix の devShell（Go・gopls・gomod2nix・gh・git）を使います。direnv があれば `.envrc` で自動で入ります。

```sh
nix develop            # または direnv allow
go test ./...
go build .
nix build              # パッケージをビルド（テストも実行）
nix fmt                # alejandra / gofmt / taplo で整形
nix flake check        # 整形のチェック
```

`go.mod` を変えたら `gomod2nix` を実行して `gomod2nix.toml` を更新してください。

### 全文検索の除外・包含

`:grep -x '*.lock,dist/**' -i '*.go' foo` のように、カンマ区切りの glob で除外 (`-x` / `--exclude=`) と包含 (`-i` / `--include=`) を指定できます。常に効かせたい指定は `~/.config/gh-editor/config.toml` か `.gh-editor/config.toml` に書きます（`.helix/config.toml` などの Helix 設定には書かないでください。Helix が未知の `[search]` を拒否します）。

`:replace foo => bar` はプロジェクト全体の `foo` を `bar` に置換します（`-x` / `-i` は `:grep` と同じ。検索は文字列そのままで、小文字のみなら大文字小文字を区別しません）。実行前に差分プレビューのタブが開き、`j`/`k` で移動、`x` で行（ファイル行ならファイルごと）を除外/復帰、`Enter` で実行、`q` で閉じます。実行すると対象ファイルをバッファとして開いて置換します（保存するまでディスクは変わらず、`u` / Ctrl+Z で元に戻せます）。

```toml
[search]
exclude = ["*.lock", "dist/**"]
include = []
```

## エージェント連携（制御用ソケット）

実行中の gh-editor は Unix ソケット（0600）を作り、環境変数 `GH_EDITOR_SOCKET` で場所を伝えます。ターミナルパネル内のシェルから次を実行できます。

- `gh-editor open <path>[:行]`: ファイルを開く
- `gh-editor diff [path]`: ローカルの変更のレビューを開く
- `gh-editor status`: 開いているタブとカーソル位置を JSON で表示する

第 1 引数が `open` / `diff` / `status` / `skill` のときはサブコマンドとして扱います。同名のファイルは `./status` のように書きます。
