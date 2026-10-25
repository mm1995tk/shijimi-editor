_: {
  projectRootFile = "flake.nix";
  programs.nixfmt.enable = true;
  programs.gofmt.enable = true;
  programs.taplo.enable = true;
  # gomod2nix writes this file; reformatting it only makes the next gomod2nix run show a diff.
  settings.global.excludes = [ "gomod2nix.toml" ];
}
