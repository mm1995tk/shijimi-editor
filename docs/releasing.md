# リリースの流れ

リリースは、リポジトリオーナーの Claude のスケジュールした作業「shijimi-editor 毎日のリリース判定」が毎日 1 回起動します。人が手で `release.yml` を起動することは、普段はありません。

## 毎日のリリース判定

- claude.ai の Claude Code のスケジュールした作業です。毎日 19:23 UTC（日本時間 4:23）に動きます。リポジトリの中には置いていません。
- 前のタグから main に入ったコミットを読み、上げる桁を決めて、`gh workflow run release.yml -f bump=<桁>` を起動します。
- リポジトリのファイル・ラベル・タグは書き換えません。バージョン上げは `release.yml` が行います。

### 桁の決め方

1. 前のタグから、コミットが無い（またはバージョン上げのコミットだけの）日は、何もしません。
2. マージされた PR に `breaking` ラベルが 1 つでもあれば、major にします。
3. ラベルが無ければ、変更を読んで決めます。基準は「利用者の手元で、何もしなくても今までどおり動くか」です。
   - minor：利用者から見える機能が増えた（コマンド・オプション・設定・テーマ・モードなど）。
   - patch：`shj` の動きか npm のパッケージの中身が変わり、minor にも破壊的にも当たらない変更（バグ修正・性能・`go.mod` の依存の更新・`README.md` の変更）。
   - リリースしない：`shj` の動きも npm のパッケージの中身も変わらない変更（Go のコメントだけ・CI・テスト・`docs/`・`manual/`・`flake.lock` の更新だけ）。次にリリースするとき、一緒に出ます。`manual/` は main に入った時点で GitHub Pages に公開されます。
   - ただし、前のタグから 7 日以上たっていて、その間に `flake.lock` が変わっていれば、patch としてリリースします。依存の更新を 1 週間ごとに届けるためです。
   - 破壊的に見える変更があるのにラベルが無いとき：リリースせず、「メジャーバージョン上げの判断（<前のタグ> 以降）」という Issue を立てます。

### 破壊的な変更の扱い

- 破壊的な変更（設定のキーの改名・既定のキー割り当ての変更・コマンドの削除など、利用者が何かを変えないと今までどおり動かない変更）の PR には、マージの前に `breaking` ラベルを付けます。
- 「メジャーバージョン上げの判断」の Issue が開いている間は、リリースしません。
  - major にするなら、その PR に `breaking` ラベルを付けて Issue を閉じます。
  - 破壊的でないなら、ラベルを付けずに Issue を閉じます。次の実行では、その PR を破壊的とみなしません。

## release.yml がすること

1. `flake.nix` の `version` を上げてコミットし、タグ `vX.Y.Z` を打ち、main と `release` ブランチと一緒に push します。
2. GitHub Release を作ります（リリースノートは自動生成）。
3. OS と CPU ごとの `shj` をビルドし、npm に publish し、GitHub Release にも `shj-<os>-<cpu>` として置きます。

## nightly 版

通常のリリースとは別に、`main` の最新を nightly 版として配ります（#252）。`release` ブランチ・タグ・npm の `latest` の流れは変えません。

- nightly 版は安定版と同じプログラムで、ビルド時の `-X github.com/mm1995tk/shijimi-editor/internal/channel.Channel=nightly` だけが違います。バイナリ名は `shj-nightly`（`shijimi-nightly` も）、設定は `~/.config/shijimi-editor-nightly/`、ソケットのディレクトリも別なので、安定版と並べて入れられます。設定の `[experimental]` の既定値は nightly 版だけ `true` です。
- Nix：`nix build .#shj-nightly`。`version` は `flake.nix` の `X.Y.Z` に `-nightly.<yyyymmdd>.<sha>` を付けたものです（`X.Y.Z` はまだ出ていない次のリリースの番号ではなく、`flake.nix` にある今の番号です）。`shj-nightly --version` は `vX.Y.Z-nightly (<sha>)` と出ます。
- npm：`npm/pack.sh X.Y.Z-nightly.<yyyymmdd>.<sha> <dir>` で、本体のパッケージ `shijimi-editor-nightly`（コマンドは `shj-nightly` と `shijimi-nightly`）と、OS / CPU ごとのパッケージ（名前は安定版と同じ `shijimi-editor-<os>-<cpu>`、版だけ nightly の版）の tarball ができます。
- 毎日ビルドして配るのは `nightly.yml`（次の節）です。

### nightly.yml がすること

`.github/workflows/nightly.yml` は、毎日 15:00 UTC（日本時間 0:00）の `schedule` と、`gh workflow run nightly.yml` などの `workflow_dispatch` で main で動きます。`update-flake.yml`（21:00 UTC）と「毎日のリリース判定」（19:23 UTC）とは重ねていません。

1. 版 `X.Y.Z-nightly.<yyyymmdd>.<sha>` を決めます。`X.Y.Z` は `flake.nix` の `version`、日付はコミットの日付（UTC）、`<sha>` は短い形です。`flake.nix` は書き換えず、`vX.Y.Z` のタグも打ちません。
2. pre-release `nightly` の本文の `Commit: <sha>` を読み、main のコミットが前の nightly から進んでいなければ、何もせずに成功します（`workflow_dispatch` の `force` を `true` にすると、進んでいなくても作り直します）。
3. `npm/pack.sh` で OS と CPU ごとの `shj-nightly` をビルドし、npm に publish します。`shijimi-editor-<os>-<cpu>` は安定版と共有なので `--tag nightly` で publish し、`latest` は動かしません。`shijimi-editor-nightly` は nightly 専用なので `--tag latest` で publish し、`npm install -g shijimi-editor-nightly` が毎回の nightly の版を入れるようにします（dist-tag `nightly` も `npm dist-tag add` で合わせますが、OIDC で通らなければ警告だけで済ませます。Trusted Publishing で認証できるのは publish だけのためです）。認証は `release.yml` と同じ Trusted Publishing（OIDC）で、npm 側で `nightly.yml` を `shijimi-editor-nightly` と `shijimi-editor-<os>-<cpu>` の Trusted Publisher に登録しておく必要があります（`release.yml` の登録とは別です）。
4. pre-release `nightly` を作り直します（タグ `nightly` をそのコミットに付け直し、`shj-nightly-<os>-<cpu>` を assets に置き、本文に版・コミット・日付を書きます）。

## 止める・変える

- 自動のリリースを止めるには、リポジトリオーナーが claude.ai の Claude Code のスケジュールした作業から「shijimi-editor 毎日のリリース判定」を無効にします。
- 桁の決め方を変えるときは、そのスケジュールした作業のプロンプトを書き換え、この文書も直します。
- 手でリリースするときは、Actions の「リリース」を main で起動し、桁を選びます。
- nightly を止めるには、Actions の「nightly 版を配る」（`nightly.yml`）を無効にします（`gh workflow disable nightly.yml`）。`schedule` だけ止めて手で動かせるようにしておくなら、`nightly.yml` の `schedule` を消します。npm の `nightly` の dist-tag と pre-release は残るので、消すなら `npm dist-tag rm` と `gh release delete nightly --cleanup-tag` で消します。
