# ブラウザとポート転送

## ブラウザタブ

`:browser <URL>` で、Web ページを画像のタブとして表示します。

- ヘッドレス Chromium でページを描画し、PNG にして表示します。
- `chromium` / `chromium-browser` / `google-chrome` を PATH から探します。
- URL に `https://` を付けなければ、自動で補います。`localhost`、`127.0.0.1`、`[::1]` には `http://` を補います。

### タブの中のキー

| キー | 動作 |
| --- | --- |
| `o` | URL を入力する |
| `r` | 再読み込み |
| `[` / `]` | 戻る / 進む |
| `j` / `k` | スクロール |
| `Ctrl+D` / `Ctrl+U` | 半ページ下 / 上 |
| `g` / `G` | 先頭 / 末尾 |

`:browser reload`、`:browser back`、`:browser next` でも同じ操作ができます。

### Chromium の場所を指定する

```toml
[browser]
chromium = "/usr/bin/chromium"
```

## 外部ブラウザで開く

- `Space o`（Helix モード）：カーソル位置の URL を開きます。
- `:open-url [URL]`：URL を開きます。省略するとカーソル位置の URL を開きます。

Linux では `xdg-open`、macOS では `open` を使います。

## ポート転送

`:browser forward <対象ポート> [待ち受けポート]` を実行すると、gh-editor が `127.0.0.1` で待ち受け、`localhost:<対象ポート>` へ TCP で転送します。
HTTP も WebSocket も通ります。

| コマンド | 動作 |
| --- | --- |
| `:browser forward <対象ポート> [待ち受けポート]` | 転送を始める。待ち受けポートを省略すると、空いているポートを選ぶ |
| `:browser list` | 転送の一覧 |
| `:browser stop <待ち受けポート>` | 転送を止める |

gh-editor を終了すると、すべての転送が止まります。

### リモートの VM で使う例

VM で gh-editor を動かし、開発サーバーを手元のブラウザで見る手順です。

1. VM の gh-editor で `:browser forward 3000` を実行します。表示された待ち受けポートを控えます。
2. 手元で次を実行します。

    ```sh
    ssh -L 8080:127.0.0.1:<待ち受けポート> vm
    ```

3. 手元のブラウザで `http://localhost:8080` を開きます。

直接 `ssh -L` するのに比べて、次の利点があります。

- 待ち受けポートを固定できます。
- `0.0.0.0` や IPv6 だけで待ち受けるサーバーも、`127.0.0.1` で受けられます。

この 2 つが不要なら、`ssh -L` を直接使ってください。

### 待ち受けアドレス

```toml
[browser]
listen-host = "127.0.0.1"
```

既定は `127.0.0.1` です。`0.0.0.0` に変えるのは、信頼できるネットワークの中だけにしてください。

!!! warning "書く場所"
    `[browser]` は `~/.config/gh-editor/config.toml` か `.gh-editor/config.toml` に書きます。
    `.helix/config.toml` に書くと、Helix が未知のセクションとして拒否します。
