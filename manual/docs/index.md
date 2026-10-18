# gh-editor

gh-editor はターミナルで動く統合開発環境（TUI）です。

- 画面の作りは **VS Code** に合わせています。
- 編集操作は **Helix** に合わせています。VS Code 風の操作にも切り替えられます。

## できること

| 機能 | 説明 | ページ |
| --- | --- | --- |
| モード編集 | Helix 風のノーマル / 挿入 / 選択モード。マルチカーソル | [編集（Helix モード）](editing-helix.md) |
| ファイルツリー | Git の変更状態を M / U / A で表示 | [ファイルと検索](files.md) |
| 検索と置換 | プロジェクト全体の検索と置換（フローティング窓） | [ファイルと検索](files.md) |
| LSP | 診断・補完・ホバー・定義へ移動・参照・名前の変更・整形 | [LSP](lsp.md) |
| レビュー | プルリクエストのレビューと、ファイル全体を diff で見て編集できる git-diff モード | [レビュー](review.md) |
| プレビュー | Markdown（mermaid を含む）と画像 | [プレビュー](preview.md) |
| ターミナル | 画面下のターミナルパネル。タブで複数開ける | [ターミナル](terminal.md) |
| ブラウザ | URL を外部ブラウザで開く。ポート転送 | [外部ブラウザとポート転送](browser.md) |
| エージェント連携 | Claude Code の IDE 連携。選択範囲やレビューコメントを AI エージェントへ渡す | [AI エージェント連携](agent.md) |

## 最初に読むページ

1. [インストールと起動](install.md)
2. [画面の見方](screen.md)
3. [編集（Helix モード）](editing-helix.md)。VS Code 風に操作したい場合は [編集（VS Code モード）](editing-vscode.md)
