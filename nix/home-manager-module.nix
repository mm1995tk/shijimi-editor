# home-manager module: programs.shijimi-editor
self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.programs.shijimi-editor;
  toml = pkgs.formats.toml { };
  # The nightly package is shj-nightly and reads its config from shijimi-editor-nightly/,
  # so the directory follows the package's main program (see internal/channel).
  bin = cfg.package.meta.mainProgram or "shj";
  configDir = "shijimi-editor${lib.removePrefix "shj" bin}";
in
{
  options.programs.shijimi-editor = {
    enable = lib.mkEnableOption "shijimi";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "shijimi.packages.\${system}.default";
      description = "The shijimi package to install.";
    };

    settings = lib.mkOption {
      type = toml.type;
      default = { };
      example = lib.literalExpression ''
        {
          keymap = "vscode";
          editor.line-number = "relative";
        }
      '';
      description = "Written to {file}`$XDG_CONFIG_HOME/shijimi-editor/config.toml` (same structure as Helix's config.toml), or {file}`shijimi-editor-nightly/config.toml` for the nightly package.";
    };

    languages = lib.mkOption {
      type = toml.type;
      default = { };
      example = lib.literalExpression ''
        {
          language-server.gopls.command = "gopls";
          language = [{ name = "go"; language-servers = ["gopls"]; }];
        }
      '';
      description = "Written to {file}`$XDG_CONFIG_HOME/shijimi-editor/languages.toml` (same structure as Helix's languages.toml), or {file}`shijimi-editor-nightly/languages.toml` for the nightly package.";
    };

    gui = {
      enable = lib.mkEnableOption "the GUI (shijimi-editor-gui). Built with Nix it has no auto update; update it with `nix flake update` and `home-manager switch`";

      package = lib.mkOption {
        type = lib.types.package;
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.gui;
        defaultText = lib.literalExpression "shijimi.packages.\${system}.gui";
        description = "The GUI package to install. Use `gui-nightly` for the nightly build (shijimi-editor-gui-nightly).";
      };
    };

    herdr = {
      enable = lib.mkEnableOption "the herdr key entry (`keysCommand`) that hides the pane; shijimi-editor never writes herdr's config.toml";

      hideKey = lib.mkOption {
        type = lib.types.str;
        default = "prefix+t";
        example = "prefix+h";
        description = "Key in herdr's notation that runs `shj exec ':sh hide'`.";
      };

      keysCommand = lib.mkOption {
        type = lib.types.listOf lib.types.attrs;
        readOnly = true;
        default =
          if cfg.herdr.enable then
            [
              {
                key = cfg.herdr.hideKey;
                type = "shell";
                command = "${cfg.package}/bin/${bin} exec ':sh hide'";
              }
            ]
          else
            [ ];
        defaultText = lib.literalExpression "[ ] when herdr.enable is false, otherwise one `shell` entry";
        description = "Read-only. A list for herdr's `[[keys.command]]`. Put it into your own herdr config.toml (e.g. `keys.command = config.programs.shijimi-editor.herdr.keysCommand;`). Empty unless `herdr.enable` is true.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ] ++ lib.optional cfg.gui.enable cfg.gui.package;
    xdg.configFile."${configDir}/config.toml" = lib.mkIf (cfg.settings != { }) {
      source = toml.generate "shijimi-config.toml" cfg.settings;
    };
    xdg.configFile."${configDir}/languages.toml" = lib.mkIf (cfg.languages != { }) {
      source = toml.generate "shijimi-languages.toml" cfg.languages;
    };
  };
}
