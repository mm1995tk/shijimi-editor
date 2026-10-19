# Nix と home-manager

## 設定を Nix で書く

`config.toml` / `languages.toml` と同じディレクトリに `config.nix` / `languages.nix` を置くと、Nix で設定を書けます。

- `nix eval --json --file` で評価し、TOML と同じ構造として読みます。
- 同じディレクトリでは、TOML を読んだあとに Nix を適用します。
- `nix` が無いとき、または評価に失敗したときは、そのファイルを無視します。

```nix
# ~/.config/dageki/languages.nix
{
  language-server.nil.command = "nil";
  language = [
    { name = "nix"; language-servers = [ "nil" ]; }
  ];
}
```

## home-manager

flake の `homeManagerModules.default` を読み込みます。

- `settings` は `~/.config/dageki/config.toml` に書き出します。
- `languages` は `~/.config/dageki/languages.toml` に書き出します。

```nix
{
  imports = [ inputs.dageki.homeManagerModules.default ];
  programs.dageki = {
    enable = true;
    settings = { keymap = "vscode"; editor.line-number = "relative"; };
    languages = {
      language-server.gopls.command = "gopls";
      language = [ { name = "go"; language-servers = [ "gopls" ]; } ];
    };
  };
}
```
