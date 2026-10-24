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
      description = "Written to {file}`$XDG_CONFIG_HOME/shijimi/config.toml` (same structure as Helix's config.toml).";
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
      description = "Written to {file}`$XDG_CONFIG_HOME/shijimi/languages.toml` (same structure as Helix's languages.toml).";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ];
    xdg.configFile."shijimi/config.toml" = lib.mkIf (cfg.settings != { }) {
      source = toml.generate "shijimi-config.toml" cfg.settings;
    };
    xdg.configFile."shijimi/languages.toml" = lib.mkIf (cfg.languages != { }) {
      source = toml.generate "shijimi-languages.toml" cfg.languages;
    };
  };
}
