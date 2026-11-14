# Nix と home-manager

## home-manager

flake の `homeManagerModules.default` を読み込みます。

- `settings` は `~/.config/shijimi-editor/config.toml` に書き出します。
- `languages` は `~/.config/shijimi-editor/languages.toml` に書き出します。
- これらのファイルは読み取り専用です。home-manager の設定を変えずに、そのマシンだけで設定を変えるには、同じフォルダの `config.local.toml` か `languages.local.toml` に書きます。home-manager が書き出したファイルより優先されます（[設定ファイル](config.md)）。
- `package = inputs.shijimi.packages.${pkgs.system}.shj-nightly` にすると nightly 版が入ります（[インストール](install.md)）。このとき、上の 2 つのファイルは `~/.config/shijimi-editor-nightly/` に書き出します。

```nix
{
  # inputs.shijimi.url = "github:mm1995tk/shijimi-editor/release";
  imports = [ inputs.shijimi.homeManagerModules.default ];
  programs.shijimi-editor = {
    enable = true;
    settings = { keymap = "vscode"; editor.line-number = "relative"; };
    languages = {
      language-server.gopls.command = "gopls";
      language = [ { name = "go"; language-servers = [ "gopls" ]; } ];
    };
  };
}
```
