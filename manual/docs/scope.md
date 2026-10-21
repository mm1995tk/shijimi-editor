# 作らないこと

shijimi に入れないと決めた機能と、要望が来たら考える機能です。

## 作らないと決めたこと

| 機能 | 理由と代わりの方法 |
| --- | --- |
| マージエディタ | 衝突の解消は AI エージェントに任せ、結果を[差分モード](review.md)で確認します。 |
| git 操作の UI（コミット・リベース・ブランチ操作など） | AI エージェントか `git` コマンドで行います。 |
| `.vimrc` / `init.lua` の読み込み | Vim script や Lua を実行する仕組みが必要になり、エディタ本体より大きくなるためです。 |
| Vim 風のキー操作 | キー操作は [Helix モード](editing-helix.md)と [VS Code モード](editing-vscode.md)の 2 つに絞ります。 |
| 保存したときの自動整形 | 自動保存のたびに、入力中の文字が動いてしまうためです。整形は `:fmt` で行います。 |
| コマンドで linter を呼ぶ仕組み | linter の出力の形はツールごとに違うためです。LSP として動く linter を使うか、[efm-langserver](https://github.com/mattn/efm-langserver) で LSP として動かします。 |

## 要望が来たら考えること

- [jujutsu](https://jj-vcs.github.io/jj/) への対応
- Codeberg / Forgejo への対応（プルリクエストの base と比べる機能）
- VS Code モードのキーの差し替え
- VS Code の `keybindings.json` の読み込み
- Jsonnet での設定（TOML と Nix に加えて読む形式として）

要望は [GitHub の Discussions](https://github.com/mm1995tk/shijimi-editor/discussions) に書いてください。
