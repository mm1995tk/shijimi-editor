# インストールと起動

## インストール

次のどれか 1 つを実行します。

```sh
nix run github:mm1995tk/shijimi-editor/release   # インストールせずにその場で起動
nix profile install github:mm1995tk/shijimi-editor/release
go install github.com/mm1995tk/shijimi-editor/cmd/shj@latest      # shj コマンドとして入れる
go install github.com/mm1995tk/shijimi-editor/cmd/shijimi@latest  # 同じものを shijimi コマンドとして入れる
```

どちらも最新のリリースを入れます。Nix のコマンドは `release` ブランチを、`go install` は一番新しいタグを取ります。`main` のリリース前の変更は入りません。

home-manager で入れる方法は [Nix と home-manager](nix.md) にあります。

## 必要なもの

shijimi 本体だけで編集はできます。次の機能は、別のコマンドが必要です。

| 機能 | 必要なコマンド |
| --- | --- |
| 差分モードでプルリクエストの base と比べる | [gh](https://cli.github.com/)。`gh auth login` を済ませておく |
| ファイルツリーの Git の状態、差分モード | `git` |
| LSP | 各言語のサーバー（`gopls`、`rust-analyzer` など） |
| mermaid の図 | [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)（`mmdc`） |
| 設定を Nix で書く | `nix` |
| システムのクリップボードへのコピーと貼り付け（`Space y` / `Space p`） | Wayland は `wl-copy` と `wl-paste`、X11 は `xclip` か `xsel`、macOS は `pbcopy` と `pbpaste`（macOS には最初から入っている）。無いときは shijimi の中のクリップボードだけを使う |
| カーソル位置の URL を外部ブラウザで開く（`Space o`） | Linux は `xdg-open`、macOS は `open`（macOS には最初から入っている） |
| ファイルツリーのアイコン | Nerd Font を設定したターミナル |

## 起動

```sh
shj            # カレントディレクトリを開く
shj ./src      # ディレクトリを開く
shj main.go    # ファイルだけを編集する（単体編集モード）
```

`shj` の代わりに `shijimi` と打っても同じです（Nix で入れたときは両方入ります）。

ファイルを渡すと、そのファイルだけを編集する単体編集モードで開きます。`$EDITOR` から呼ぶ使い方を想定しています。ファイルツリー・LSP・ターミナル・差分モード・提案パネル・Claude Code の IDE 連携は起動せず、タブも使いません。`:q` `:wq`（VS Code モードでは `Ctrl+W` か `Ctrl+Q`）でファイルを閉じると shijimi を終了するので、呼び出し元は終了を待って次へ進めます。存在しないパスは空のバッファで開き、保存したときにファイルを作ります。ディレクトリを渡したときの動作は変わりません。

## 終了

`Ctrl+Q` を押します。Helix モードでは `:q` でも終了できます。

!!! note "サブコマンドと同じ名前のファイル"
    第 1 引数が `diff` / `skill` のときは、サブコマンドとして扱います。
    同じ名前のファイルを開くときは `./diff` のように書きます。
    サブコマンドは [AI エージェント連携](agent.md) で説明します。
