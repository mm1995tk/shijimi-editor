# Nix and home-manager

## home-manager

Import the flake's `homeManagerModules.default`.

- `settings` is written to `~/.config/shijimi-editor/config.toml`.
- `languages` is written to `~/.config/shijimi-editor/languages.toml`.
- These files are read-only. To change a setting on one machine without changing the home-manager configuration, write it in `config.local.toml` or `languages.local.toml` in the same folder; they win over the files home-manager writes ([Configuration files](config.md)).
- kisui-terminal's settings (the machines in `[[gui.machines]]`, `[server]`) are not written here. They belong to the home-manager module of kisui-terminal (`programs.kisui-terminal`, from the `homeManagerModules.default` of github:mm1995tk/kisui-terminal), which writes `~/.config/kisui-terminal/config.toml` (see its README).
- With `package = inputs.shijimi.packages.${pkgs.system}.shj-nightly` you get the nightly build ([Installation](install.md)). The two files above are then written to `~/.config/shijimi-editor-nightly/` instead.

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
