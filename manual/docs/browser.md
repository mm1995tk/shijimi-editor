# ブラウザとポート転送

## ブラウザタブ

`:browser <URL>` で、Web ページを画像のタブとして表示します。

- ヘッドレス Chromium でページを描画し、画像にして表示します。
- `chromium` / `chromium-browser` / `google-chrome` を PATH から探します。
- URL に `https://` を付けなければ、自動で補います。`localhost`、`127.0.0.1`、`[::1]` には `http://` を補います。

### タブの中のキー

| キー | 動作 |
| --- | --- |
| `o` | URL を入力する |
| `O` / `Ctrl+L` | 今の URL を入れた状態で URL を入力する（`Ctrl+L` は入力モードでも効く。ライブ表示では状態行のクリックも同じ） |
| `r` | 再読み込み |
| `[` / `]` | 戻る / 進む |
| `j` / `k` | スクロール |
| `Ctrl+D` / `Ctrl+U` | 半ページ下 / 上 |
| `g` / `G` | 先頭 / 末尾 |

`:browser reload`、`:browser back`、`:browser next` でも同じ操作ができます。

### ライブ表示（Kitty graphics protocol 対応の端末）

Kitty / Ghostty / WezTerm では、ブラウザタブが次のように変わります。

- Chromium を常駐させ（Chrome DevTools Protocol）、画面を画像として端末へ送ります。文字が読める解像度で表示され、動画や動きのあるページも動いて見えます（最大 15 fps）。音声は再生しません。
- マウスのクリック・ドラッグ・ホイールを、ページへ転送します。
- ページをクリックするか、`i`（または `Enter`）を押すと、キーをページへ送る入力モードに入ります。`Esc` で戻ります。
- 入力モード以外では、上の表のキーが使えます。
- 別のタブを前に出している間は、画面の送信を止めて負荷を下げます。
- sandbox は無効にしません。sandbox 付きで Chromium を起動できない環境では、エラーを表示します。

端末の判定は環境変数で行います（`TERM=xterm-kitty`、`KITTY_WINDOW_ID`、`TERM_PROGRAM=ghostty` / `WezTerm` など）。Herdr の中では、画面の描き直しで画像が消えるため自動では使いません（`GH_EDITOR_GRAPHICS=kitty` で強制できます）。
判定が合わないときは、`GH_EDITOR_GRAPHICS` で上書きできます。

| 値 | 動作 |
| --- | --- |
| `kitty` | 強制的にライブ表示を使う |
| `halfblock` | 使わない |

SSH 越しでは、端末を示す環境変数が引き継がれないことがあります。そのときは `GH_EDITOR_GRAPHICS=kitty` を設定してください。

対応していない端末では、半ブロック文字（`▀`）の静止画表示になります。静止画は、ページの JavaScript を最大 5 秒（仮想時間）動かしてから撮ります。
画像は Unicode placeholder（U+10EEEE）の方式で配置するので、tmux や Herdr のように端末とのあいだに入るものが Kitty graphics protocol を通す場合にも使えます。

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

### 音声

```toml
[browser]
audio = true
```

既定ではミュートです。`audio = true` にすると、Chromium のミュートを外します。音声は gh-editor を動かしているマシンで鳴ります。

SSH 先（dev-vm など）で動かしている場合、スピーカーがないので Mac まで音声を運ぶ必要があります。PulseAudio を使う例です。

1. Mac で PulseAudio サーバーを起動し、TCP で受け付けるようにします（`module-native-protocol-tcp`、`auth-ip-acl=127.0.0.1`）。
2. `ssh -R 4713:localhost:4713 <host>` で接続します。
3. SSH 先で `PULSE_SERVER=tcp:localhost:4713 gh-editor` を起動します。Chromium は環境変数を引き継ぎます。

Mac 上で直接 gh-editor を動かす場合は、`audio = true` だけで鳴ります。映像は最大 15 fps で遅れるため、音声のほうが先に進むことがあります。
