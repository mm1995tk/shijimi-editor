# Nix and home-manager

## Writing the configuration in Nix

If you put `config.nix` / `languages.nix` in the same directory as `config.toml` / `languages.toml`, you can write the configuration in Nix.

- It is evaluated with `nix eval --json --file` and read as the same structure as TOML.
- In the same directory, the Nix file is applied after the TOML file is read.
- If `nix` is not available, or evaluation fails, that file is ignored.

```nix
# ~/.config/shijimi-editor/languages.nix
{
  language-server.nil.command = "nil";
  language = [
    { name = "nix"; language-servers = [ "nil" ]; }
  ];
}
```

## home-manager

Import the flake's `homeManagerModules.default`.

- `settings` is written to `~/.config/shijimi-editor/config.toml`.
- `languages` is written to `~/.config/shijimi-editor/languages.toml`.
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
