# 編集（Helix モード）

既定の操作は Helix 風のモード編集です。
VS Code 風に切り替える方法は [編集（VS Code モード）](editing-vscode.md) にあります。

## モード

| モード | ステータスバーの表示 | 入り方 | 抜け方 |
| --- | --- | --- | --- |
| ノーマル | `NOR` | 起動時。`Esc` | — |
| 挿入 | `INS` | `i` `a` `I` `A` `o` `O`、`c` | `Esc` |
| 選択 | `SEL` | `v` | `v` または `Esc` |

Helix と同じく「先に選択し、次に操作する」順で編集します。
たとえば `w` で単語を選び、`d` で削除します。

## ノーマルモード

### 移動

| キー | 動作 |
| --- | --- |
| `h` `j` `k` `l`（矢印キーも可） | 左 / 下 / 上 / 右へ移動。数字を前に付けると回数指定（例：`5j`） |
| `w` `b` `e`（`W` `B` `E`） | 単語を選択しながら移動 |
| `Home` / `End` | 行頭 / 行末 |
| `PgUp` `Ctrl+B` / `PgDn` `Ctrl+F` | 1 画面上 / 下 |
| `Ctrl+U` / `Ctrl+D` | 上 / 下へ動く。キーを離したことを伝える端末（kitty keyboard protocol に対応した端末）では、押している間だけ 1 行ずつスクロールする。それ以外の端末では半画面動き、カーソル行を画面の中央に置く |
| `Ctrl+Alt+U` / `Ctrl+Alt+D` | カーソルを元の行に残したまま、画面だけを上 / 下へスクロールする。動き方は上と同じ（押している間 1 行ずつ、または 1 回で半画面）。カーソルが画面の端に来たら、端に留まって押し出される |
| `\` | 行番号を、設定（`editor.line-number`）と反対の方式（絶対 ⇄ 相対）で出す。キーを離したことを伝える端末では押している間だけ。それ以外の端末では押すたびに切り替える |
| `zh` / `zl`（`Alt+PgUp` / `Alt+PgDn`） | 画面を半画面ぶん左 / 右へスクロールする。カーソルは動かさない。左端と、一番長い行の末尾が見えるところで止まる。マウスの横ホイールと `Shift` + ホイールでも同じ |
| `G` | 最後の行 |
| `f` `t` `F` `T` + 文字 | 行内で文字を探す（`f` `t` は前方、`F` `T` は後方） |
| `mm` | 対応する括弧へ |
| `mi` / `ma` + 対象 | テキストオブジェクトの内側（`mi`）／外側（`ma`）を選択する。対象は `w`（単語）、`W`（空白で区切った単語）、`p`（段落）、括弧 `(` `)` `[` `]` `{` `}` `<` `>`（開き閉じのどちらでも同じ）、引用符 `'` `"` `` ` ``。`maw` は単語の後ろの空白を、`ma(` は括弧自体を含む。マルチカーソルではカーソルごとに選ぶ。tree-sitter を使う対象（`f` `t` `a` `c` など）は未対応 |

### `g` で始まるジャンプ

| キー | 動作 |
| --- | --- |
| `gg` / `ge` | ファイルの先頭 / 末尾 |
| `gh` / `gl` | 行頭 / 行末 |
| `gs` | 行の最初の空白でない文字 |
| `gt` / `gc` / `gb` | 画面の上端 / 中央 / 下端の行 |
| `数字g` | 指定した行（例：`42g`） |
| `gd` / `gr` | 定義へ移動 / 参照の一覧 |
| `Ctrl+O` / `Ctrl+I` | 飛ぶ前の場所へ戻る / 進む（最大 100 件。`gd` `gg` `ge` `数字g` `:行番号` `/` `n` `N` や各ピッカーが履歴に積む） |
| `ga` | 直前に開いていたファイルと今のファイルを行き来 |

### 選択

| キー | 動作 |
| --- | --- |
| `x`（`X`） | 行を選択。続けて押すと下へ広がる |
| `%` | すべて選択 |
| `;` | 選択を解除 |
| `Alt+;` | 選択の向きを反転（カーソルと起点を入れ替える） |
| `v` | 選択モードの切替 |

### 編集

| キー | 動作 |
| --- | --- |
| `d` | 選択範囲（なければカーソルの文字）を削除してヤンク |
| `Alt+D` | ヤンクせずに削除 |
| `c` | 削除して挿入モードへ。`Alt+c`（小文字）はヤンクしない |
| `y` | ヤンク（コピー） |
| `p` / `P` | カーソルの後ろ / 前に貼り付け |
| `R` | 選択範囲をヤンクした内容で置き換え |
| `r` + 文字 | 選択範囲の文字を、指定した文字で置き換え |
| `u` / `U` | 元に戻す / やり直し |
| `>` / `<` | インデントを増やす / 減らす |
| `J` | 行を連結 |
| `.` | 直前の挿入を繰り返す。`i` `a` `I` `A` `o` `O` `c` などで挿入モードに入ってから `Esc` までに押したキーを、もう一度実行する（`d` など挿入モードを通らない変更は繰り返さない。補完で確定した文字列は繰り返すが、スニペットは展開しない） |
| `~` | 大文字と小文字を入れ替える |
| `` ` `` / `` Alt+` `` | 小文字にする / 大文字にする |
| `Ctrl+C` | 行コメントの切替 |

### 検索

| キー | 動作 |
| --- | --- |
| `/` / `?` | 前方 / 後方へ検索 |
| `n` / `N` | 次 / 前の一致 |
| `*` | 選択範囲（1 行以内）を検索語にする |

### その他

| キー | 動作 |
| --- | --- |
| `K` | ホバー（LSP） |
| `]d` / `[d` | 次 / 前の診断 |
| `:` | コマンドを入力する（[:コマンド一覧](commands.md)） |

## マルチカーソル

| キー | 動作 |
| --- | --- |
| `C` | カーソルを下の行に複製 |
| `Alt+C`（大文字） | カーソルを上の行に複製 |
| `s` | 選択範囲を正規表現で分割し、一致ごとにカーソルを置く |
| `,` | カーソルを 1 つに戻す |

挿入・削除・`c` などの編集は、すべてのカーソルに適用します。
ステータスバーの右にカーソルの数が出ます。

## Space メニュー

`Space` を押してから、次のキーを押します。

| キー | 動作 |
| --- | --- |
| `Space f` | ファイルを開く（Quick Open） |
| `Space b` | 開いているバッファの一覧 |
| `Space w` | ウィンドウメニュー: `v` エディタを 2 つに分ける、`h` `l` `w` フォーカスを移す、`q` `o` 分割を閉じる |
| `Space e` | ファイルツリーへ移動 |
| `Space E` | CHANGES へ移動（差分モードのとき。差分モードでないときはファイルツリーへ移動） |
| `Space k` | ホバー |
| `Space r` | 名前の変更（LSP） |
| `Space .` | コードアクション（LSP。診断のヒントを適用する） |
| `Space s` / `Space S` | シンボル一覧（LSP。ファイル内 / プロジェクト全体） |
| `Space d` | 診断の一覧 |
| `Space /` | プロジェクト全体を検索・置換（フローティング窓。[ファイルと検索](files.md)） |
| `Space c` | 行コメントの切替 |
| `Space t` | ターミナルの表示切替 |
| `Space g` | 差分モード（比較元はプルリクエストの base）を開く / 閉じる |
| `Space G` | 差分モード（比較元は `HEAD`）を開く / 閉じる |
| `Space v` | レビューのメニュー（比較元がプルリクエストの base のとき。[差分モード](review.md)） |
| `Space m` | Markdown プレビューの切替（並べて表示 → 全幅 → 閉じる） |
| `Space a` | 現在の文脈をエージェントへ送る |
| `Space i` | 提案パネルへ移る / 閉じる |
| `Space o` | カーソル位置の URL を外部ブラウザで開く |
| `Space y` / `Space p` | システムのクリップボードへコピー / から貼り付け |
| `Space ?` | コマンドパレット |

## 挿入モード

文字を入力します。`Esc` でノーマルモードへ戻ります。

| キー | 動作 |
| --- | --- |
| `Ctrl+X` / `Ctrl+Space` | 補完を手動で出す |
| `Tab` / `Enter` | 補完を確定 |
| `Ctrl+W` / `Alt+Backspace` | 前の単語を削除 |
| `Ctrl+U` | 行頭までを削除 |

補完は `.` などの入力や、2 文字の入力で自動的に出ます。

関数の引数などの穴埋め（スニペット）を確定すると、最初の穴が選択されます。そのまま入力すると置き換わります。`Tab` で次の穴、`Shift+Tab` で前の穴へ移ります。最後の穴（または `$0`）に着くか `Esc` で終わります。同じ番号の穴は同時に書き換わります。穴の外へカーソルを動かしても終わります。

その他のキー（矢印キー、`Ctrl+Z` など）は [VS Code モード](editing-vscode.md) と同じ動作です。

## キーの差し替え

`~/.config/shijimi-editor/config.toml` の `[keys.normal]` `[keys.insert]` `[keys.select]` で、キーを差し替えられます。書き方は Helix の `config.toml` と同じです。

```toml
[keys.normal]
"C-s" = ":write"           # : コマンドも書ける
"H" = "goto_line_start"
"A-j" = "move_visual_line_down"
"d" = "delete_selection_noyank"
"q" = "no_op"              # キーを無効にする

[keys.select]
"j" = "extend_line_below"  # 選択モードだけで差し替える

[keys.insert]
"A-j" = "normal_mode"
```

- キーは `C-s`（Ctrl）、`A-x`（Alt）、`S-tab`（Shift）のように書きます。`C-A-x` のように重ねられます。特別なキーは Helix と同じ名前（`ret` `space` `tab` `esc` `backspace` `del` `ins` `left` `right` `up` `down` `home` `end` `pageup` `pagedown` `F1`〜`F12`）です。`-` は `minus`、`<` は `lt`、`>` は `gt`、`+` は `plus` と書きます。
- 値は、下の表のコマンド名か、`:` から始まるコマンド（`":write"` など。[:コマンド一覧](commands.md)）です。
- `[keys.select]` は、`[keys.normal]` を土台にして、その上に重ねます。
- 名前の無いキー（`g` で始まる系列、`/` `n`、`Space` メニューなど）は、今のまま動きます。また `[keys.normal.g]` のような、キーを続けて押す系列の書き方はまだ使えません。
- `Ctrl+S` `Ctrl+Q` `Ctrl+P` など、どのモードでも効く操作のキーは差し替えられません。
- 知らないコマンド名やキー名を書くと、起動時（と `:config-reload`）にステータス行で知らせます。そのキーは既定のまま動きます。
- 安全のため、`[keys.*]` は `~/.config/shijimi-editor/config.toml`（と `config.nix`）からだけ読みます。プロジェクトの `.shijimi-editor/config.toml` に書いても無視します（`:sh` などを割り当てたリポジトリを開くだけで、プログラムが動くのを防ぐため）。
- `[keys.insert]` には、同じコマンド名か `:` コマンドを書けます。そのキーでは文字が入力されなくなります。

### 使えるコマンド名

「既定のキー」は差し替えないときのキーです。

移動：

| コマンド名 | 既定のキー |
| --- | --- |
| `move_char_left` / `move_char_right` | `h` `←` / `l` `→` |
| `move_visual_line_down` / `move_visual_line_up` | `j` `↓` / `k` `↑` |
| `move_line_down` / `move_line_up` | 上の 2 つと同じ動き（Helix と同じ名前で書けるように） |
| `move_next_word_start` / `move_next_long_word_start` | `w` / `W` |
| `move_prev_word_start` / `move_prev_long_word_start` | `b` / `B` |
| `move_next_word_end` / `move_next_long_word_end` | `e` / `E` |
| `goto_line_start` / `goto_line_end` | `Home` / `End` |
| `page_up` / `page_down` | `PgUp` `Ctrl+B` / `PgDn` `Ctrl+F` |
| `jump_backward` / `jump_forward` | `Ctrl+O` / `Tab` `Ctrl+I` |
| `goto_last_line` | `G` |

`move_*` は、選択モードのときだけ選択を広げます。`extend_*` は、いつでも選択を広げます（`extend_char_left` `extend_char_right` `extend_visual_line_down` `extend_visual_line_up` `extend_line_down` `extend_line_up` `extend_next_word_start` `extend_next_long_word_start` `extend_prev_word_start` `extend_prev_long_word_start` `extend_next_word_end` `extend_next_long_word_end` `extend_to_line_start` `extend_to_line_end`）。

選択：

| コマンド名 | 既定のキー |
| --- | --- |
| `extend_line_below` | `x` |
| `extend_to_line_bounds` | `X` |
| `select_all` | `%` |
| `collapse_selection` | `;` |
| `flip_selections` | `Alt+;` |
| `select_mode` | `v`（選択モードの切替） |
| `normal_mode` | `Esc` |
| `no_op` | なし（キーを無効にする） |

編集：

| コマンド名 | 既定のキー |
| --- | --- |
| `delete_selection` / `delete_selection_noyank` | `d` / `Alt+D` |
| `change_selection` / `change_selection_noyank` | `c` / `Alt+c` |
| `yank` | `y` |
| `paste_after` / `paste_before` | `p` / `P` |
| `replace_with_yanked` | `R` |
| `undo` / `redo` | `u` / `U` |
| `insert_mode` / `append_mode` | `i` / `a` |
| `insert_at_line_start` / `insert_at_line_end` | `I` / `A` |
| `open_below` / `open_above` | `o` / `O` |
| `repeat_last_insert` | `.` |
| `indent` / `unindent` | `>` / `<` |
| `join_selections` | `J` |
| `switch_case` | `~` |
| `switch_to_lowercase` / `switch_to_uppercase` | `` ` `` / `` Alt+` `` |
| `toggle_comments` | `Ctrl+C` |

数字を前に付ける回数指定と、複数のカーソルへの適用は、差し替えたキーでも同じように働きます。
