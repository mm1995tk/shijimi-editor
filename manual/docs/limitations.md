# 制限

現在わかっている制限です。

- 画像と mermaid の図は Kitty graphics protocol に対応した端末（Kitty / Ghostty / WezTerm など）でしか表示できません。
- Web ページを表示するブラウザは入っていません。外部ブラウザで開いてください（[外部ブラウザとポート転送](browser.md)）。
- git-diff モードの差分は、GitHub に送れません。GitHub にレビューを送れるのは、プルリクエストのレビューだけです。
- プルリクエストのレビューには `gh` と `gh auth login` が必要です。
