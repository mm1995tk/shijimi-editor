# プレビュー

## Markdown

`.md` ファイルで次のどれかを実行すると、バッファの右にプレビューが開きます。

- `Space m`（Helix モード）
- `:preview`
- コマンドパレット

見出し・リスト・コード・表を整形して表示します。
プレビューは編集に追従し、カーソルの位置に合わせてスクロールします。
もう一度実行すると、プレビューだけを全幅で表示します。3 回目で閉じます。
全幅のときは、`j` / `k` やマウスホイールでスクロールします。

## mermaid

mermaid のコードブロックは図として表示します。

- [mermaid-cli](https://github.com/mermaid-js/mermaid-cli)（`mmdc`）で PNG にします。
- `mmdc` が PATH に無いときや、描画に失敗したときは、図の代わりにソースとエラーを表示します。
- 描画した結果は `~/.cache/shijimi/mermaid` に保存します。

`[editor]` の `mermaid-command` で、使うコマンドを差し替えられます。`{in}` が入力ファイル、`{out}` が出力する PNG です。空にすると無効になります。

```toml
[editor]
mermaid-command = "mmdc -q -i {in} -o {out} -b transparent -s 2"
```

## 画像

次の形式の画像ファイルを開くと、画像として表示します。

PNG / JPEG / GIF / BMP / WebP / TIFF

画像と mermaid の図は Kitty graphics protocol で描きます。Kitty / Ghostty / WezTerm など、これに対応した端末が必要です。
