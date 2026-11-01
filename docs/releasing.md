# リリースの流れ

リリースは、リポジトリオーナーの Claude のスケジュールした作業「shijimi-editor 毎日のリリース判定」が毎日 1 回起動します。人が手で `release.yml` を起動することは、普段はありません。

安定版は、main の最新ではなく、**nightly 版として 3 日以上配ったコミット**から作ります。main に入った変更は、まず nightly で試されます。

## 毎日のリリース判定

- claude.ai の Claude Code のスケジュールした作業です。毎日 19:23 UTC（日本時間 4:23）に動きます。リポジトリの中には置いていません。
- リリースするコミット（候補）と上げる桁を決めて、`gh workflow run release.yml --ref main -f bump=<桁> -f ref=<候補のコミット>` を起動します。
- リポジトリのファイル・ラベル・タグは書き換えません。バージョン上げは `release.yml` が行います。

### 前のリリース

前のリリースは、main の `flake.nix` の `version` で決めます（`v<version>`）。タグの並びでは決めません。`v1.0.1` は、誤って出した `v1.0.0` を Go で取り消す（`go.mod` の `retract`）ためだけのタグだからです。

### 候補の決め方

1. npm の `shijimi-editor-nightly` の版（`X.Y.Z-nightly.<yyyymmdd>.<sha>`）のうち、publish してから 72 時間以上たったものの中で、一番新しいものを選びます。その `<sha>` のコミットが候補です。
2. 候補が前のリリースより後のコミットでなければ（前のリリースの子孫でない、または同じ）、何もしません。
3. 次のどれかに当たるときは、その日はリリースしません。次の日に、もう一度候補を選び直します。
   - 候補より後に main に入ったコミットの中に、前のリリースから候補までの変更の不具合を直すものがある。候補には、その不具合が残っているためです。
   - `bug` ラベルの付いた開いている Issue のうち、前のリリースから候補までの変更が原因のものがある。
   - 前のリリースから候補までに、`shj` の動きも npm のパッケージの中身も変わる変更が無い（Go のコメントだけ・CI・テスト・`docs/`・`manual/`・`flake.lock` の更新だけ）。ただし、前のリリースから 7 日以上たっていて、その間に `flake.lock` が変わっていれば、patch としてリリースします。依存の更新を 1 週間ごとに届けるためです。

### 桁の決め方

0.x の間と 1.0.0 以降では、決まりを分けます。今は 0.x の間の決まりだけを決めています。

#### 0.x の間

- major は上げません。0.x から 1.0.0 に上げるのは、人が `release.yml` を手で起動して決めます。
- minor：破壊的な変更がある、または利用者から見える機能が増えた（コマンド・オプション・設定・テーマ・モードなど）。
- patch：`shj` の動きか npm のパッケージの中身が変わり、minor に当たらない変更（バグ修正・性能・`go.mod` の依存の更新・`README.md` の変更）。

破壊的な変更（設定のキーの改名・既定のキー割り当ての変更・コマンドの削除など、利用者が何かを変えないと今までどおり動かない変更）の PR には、マージの前に `breaking` ラベルを付けます。0.x の間は、このラベルの付いた PR があれば minor にします。

#### 1.0.0 にする条件

1.0.0 と 1.0.1 は使えないので（1.0.0 は誤って出して取り消した版、1.0.1 は Go の retract のためだけのタグ）、0.x の次の major は 1.1.0 になります。`release.yml` が自動でそうします。この文書では、それも「1.0.0」と呼びます。

次の 3 つを全部満たしたら、毎日のリリース判定は「1.0.0 にする判断」という Issue を立てて知らせます（同じ題の Issue が開いていれば立てません）。1.0.0 を出すのは、人が `release.yml` を手で起動して決めます。

1. 作り終わり：GUI 版の親 Issue #271 が閉じている。
2. 壊さなくなった：`breaking` ラベルの付いた PR が、30 日間 main にマージされていない。
3. 試し終わり：1 と 2 を両方満たした日より後に publish した nightly が、publish から 14 日以上たっている。その nightly のコミットまでの変更が原因の `bug` Issue が開いていない。

#### 1.0.0 以降

まだ決めていません。1.0.0 を出すと決めたときに、この節と毎日のリリース判定のプロンプトを書きます。nightly で試す期間は、0.x の間の 3 日より長い 14 日にするつもりです。それまでに 1.0.0 以降の版が出ていたら、毎日のリリース判定は桁を決めずに止まります。

## release.yml がすること

1. 入力 `ref` のコミット（空なら main の最新）が、main の上にあり、前のリリースより後のコミットであることを確かめます。
2. そのコミットの `flake.nix` の `version` を上げてコミットし、タグ `vX.Y.Z` を打ちます。
3. そのコミットを main にマージします（`ref` が main の最新なら早送り）。main・タグ・`release` ブランチを一緒に push します。main にマージするのは、main の `version` を上げ、次のリリースで `release` ブランチを早送りで進められるようにするためです。
4. GitHub Release を作ります（リリースノートは前のリリースのタグからの自動生成）。
5. OS と CPU ごとの `shj` をビルドし、npm に publish し、GitHub Release にも `shj-<os>-<cpu>` として置きます。

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
3. `npm/pack.sh` で OS と CPU ごとの `shj-nightly` をビルドし、npm に publish します。`shijimi-editor-<os>-<cpu>` は安定版と共有なので `--tag nightly` で publish し、`latest` は動かしません。`shijimi-editor-nightly` は nightly 専用なので `--tag latest` で publish し、`npm install -g shijimi-editor-nightly` が毎回の nightly の版を入れるようにします。dist-tag `nightly` は `shijimi-editor-nightly` では使いません（`npm dist-tag add` は Trusted Publishing では認証できないため。過去に付いた古い `nightly` タグは `npm dist-tag rm shijimi-editor-nightly nightly` で手で消します）。認証は `release.yml` と同じ Trusted Publishing（OIDC）で、npm 側で `nightly.yml` を `shijimi-editor-nightly` と `shijimi-editor-<os>-<cpu>` の Trusted Publisher に登録しておく必要があります（`release.yml` の登録とは別です）。
4. pre-release `nightly` を作り直します（タグ `nightly` をそのコミットに付け直し、`shj-nightly-<os>-<cpu>` を assets に置き、本文に版・コミット・日付を書きます）。

## 止める・変える

- 自動のリリースを止めるには、リポジトリオーナーが claude.ai の Claude Code のスケジュールした作業から「shijimi-editor 毎日のリリース判定」を無効にします。
- 桁の決め方を変えるときは、そのスケジュールした作業のプロンプトを書き換え、この文書も直します。
- 手でリリースするときは、Actions の「リリース」を main で起動し、桁を選びます。`ref` を空にすると main の最新を出します。
- nightly を止めるには、Actions の「nightly 版を配る」（`nightly.yml`）を無効にします（`gh workflow disable nightly.yml`）。`schedule` だけ止めて手で動かせるようにしておくなら、`nightly.yml` の `schedule` を消します。npm の `nightly` の dist-tag と pre-release は残るので、消すなら `npm dist-tag rm` と `gh release delete nightly --cleanup-tag` で消します。
