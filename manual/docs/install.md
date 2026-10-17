# インストールと起動

## インストール

次のどれか 1 つを実行します。

```sh
nix run github:mm1995tk/gh-editor           # インストールせずにその場で起動
nix profile install github:mm1995tk/gh-editor
go install github.com/mm1995tk/gh-editor@latest
```

home-manager で入れる方法は [Nix と home-manager](nix.md) にあります。

## 必要なもの

gh-editor 本体だけで編集はできます。次の機能は、別のコマンドが必要です。

| 機能 | 必要なコマンド |
| --- | --- |
| プルリクエストのレビュー | [gh](https://cli.github.com/)。`gh auth login` を済ませておく |
| ファイルツリーの Git の状態、git-diff モード | `git` |
| LSP | 各言語のサーバー（`gopls`、`rust-analyzer` など） |
| mermaid の図 | [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)（`mmdc`） |
| ブラウザタブ | `chromium` / `chromium-browser` / `google-chrome` のどれか |
| 設定を Nix で書く | `nix` |
| ファイルツリーのアイコン | Nerd Font を設定したターミナル |

## 起動

```sh
gh-editor            # カレントディレクトリを開く
gh-editor ./src      # ディレクトリを開く
gh-editor main.go    # ファイルを開く
```

## 終了

`Ctrl+Q` を押します。Helix モードでは `:q` でも終了できます。

!!! note "サブコマンドと同じ名前のファイル"
    第 1 引数が `diff` / `skill` のときは、サブコマンドとして扱います。
    同じ名前のファイルを開くときは `./diff` のように書きます。
    サブコマンドは [AI エージェント連携](agent.md) で説明します。
