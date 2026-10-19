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

    extraPackages = lib.mkOption {
      type = with lib.types; listOf package;
      default = [];
      example = lib.literalExpression "with pkgs; [ gh git mermaid-cli gopls ]";
      description = ''
        Commands added to the end of {command}`dgk`'s PATH, such as gh, git,
        mermaid-cli and language servers. Commands already on PATH win.
      '';
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
    home.packages =
      if cfg.extraPackages == []
      then [cfg.package]
      else [
        (pkgs.symlinkJoin {
          name = "${lib.getName cfg.package}-wrapped-${lib.getVersion cfg.package}";
          paths = [cfg.package];
          preferLocalBuild = true;
          nativeBuildInputs = [pkgs.makeWrapper];
          postBuild = ''
            wrapProgram $out/bin/dgk --suffix PATH : ${lib.makeBinPath cfg.extraPackages}
          '';
        })
      ];
    xdg.configFile."dageki/config.toml" = lib.mkIf (cfg.settings != {}) {
      source = toml.generate "dageki-config.toml" cfg.settings;
    };
    xdg.configFile."dageki/languages.toml" = lib.mkIf (cfg.languages != {}) {
      source = toml.generate "dageki-languages.toml" cfg.languages;
    };
  };
}
