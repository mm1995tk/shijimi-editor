# 編集（VS Code モード）

モードのない、VS Code 風の操作です。

## 切り替え方

次のどれかで切り替えます。

- 実行中に `:keymap vscode` を入力する。Helix モードに戻すときは `:keymap helix`。
- `config.toml` に `keymap = "vscode"` と書く（[設定ファイル](config.md)）。

## キー操作

| キー | 動作 |
| --- | --- |
| 矢印キー | 移動 |
| `Shift` + 矢印キー | 選択しながら移動 |
| `Ctrl+←` / `Ctrl+→`（`Alt+B` / `Alt+F`） | 単語単位で移動。`Shift` を足すと選択 |
| `Home` / `End` | 行頭 / 行末。`Shift` を足すと選択 |
| `Ctrl+Home` / `Ctrl+End` | ファイルの先頭 / 末尾。`Shift` を足すと選択 |
| `PgUp` / `PgDn` | 1 画面上 / 下。`Shift` を足すと選択 |
| `Alt+↑` / `Alt+↓` | 行を上 / 下へ移動 |
| `Tab` / `Shift+Tab` | インデントを増やす / 減らす |
| `Ctrl+Z` | 元に戻す |
| `Ctrl+Y` / `Ctrl+Shift+Z` | やり直し |
| `Ctrl+A` | すべて選択 |
| `Ctrl+C` / `Ctrl+X` | コピー / 切り取り。選択がなければ行全体 |
| `Ctrl+V` | 貼り付け |
| `Ctrl+F` | ファイル内を検索 |
| `Ctrl+/` | 行コメントの切替 |
| `Ctrl+Space` | 補完を手動で出す（ターミナルの切替には使いません） |
| `F12` | 定義へ移動 |
| `Shift+F12` | 参照の一覧 |
| `Alt+←` / `Alt+→` | 飛ぶ前の場所へ戻る / 進む |
| `F2` | 名前の変更 |
| `Esc` | 選択と追加のカーソルを解除 |

全体のキー（`Ctrl+P`、`Ctrl+S` など）は [画面の見方](screen.md) と [ファイルと検索](files.md) にあります。
