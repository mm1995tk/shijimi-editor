# home-manager module: programs.dageki
self: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.dageki;
  toml = pkgs.formats.toml {};
in {
  options.programs.dageki = {
    enable = lib.mkEnableOption "dageki";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "dageki.packages.\${system}.default";
      description = "The dageki package to install.";
    };

    settings = lib.mkOption {
      type = toml.type;
      default = {};
      example = lib.literalExpression ''
        {
          keymap = "vscode";
          editor.line-number = "relative";
        }
      '';
      description = "Written to {file}`$XDG_CONFIG_HOME/dageki/config.toml` (same structure as Helix's config.toml).";
    };

    languages = lib.mkOption {
      type = toml.type;
      default = {};
      example = lib.literalExpression ''
        {
          language-server.gopls.command = "gopls";
          language = [{ name = "go"; language-servers = ["gopls"]; }];
        }
      '';
      description = "Written to {file}`$XDG_CONFIG_HOME/dageki/languages.toml` (same structure as Helix's languages.toml).";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [cfg.package];
    xdg.configFile."dageki/config.toml" = lib.mkIf (cfg.settings != {}) {
      source = toml.generate "dageki-config.toml" cfg.settings;
    };
    xdg.configFile."dageki/languages.toml" = lib.mkIf (cfg.languages != {}) {
      source = toml.generate "dageki-languages.toml" cfg.languages;
    };
  };
}
