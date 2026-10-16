# home-manager module: programs.gh-editor
self: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.gh-editor;
  toml = pkgs.formats.toml {};
in {
  options.programs.gh-editor = {
    enable = lib.mkEnableOption "gh-editor";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "gh-editor.packages.\${system}.default";
      description = "The gh-editor package to install.";
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
      description = "Written to {file}`$XDG_CONFIG_HOME/gh-editor/config.toml` (same structure as Helix's config.toml).";
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
      description = "Written to {file}`$XDG_CONFIG_HOME/gh-editor/languages.toml` (same structure as Helix's languages.toml).";
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [cfg.package];
    xdg.configFile."gh-editor/config.toml" = lib.mkIf (cfg.settings != {}) {
      source = toml.generate "gh-editor-config.toml" cfg.settings;
    };
    xdg.configFile."gh-editor/languages.toml" = lib.mkIf (cfg.languages != {}) {
      source = toml.generate "gh-editor-languages.toml" cfg.languages;
    };
  };
}
