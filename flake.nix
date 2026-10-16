{
  description = "gh-editor: VS Code-style terminal IDE with Helix keybindings";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix.url = "github:nix-community/gomod2nix";
    gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
    gomod2nix.inputs.flake-utils.follows = "flake-utils";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    gomod2nix,
    treefmt-nix,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      project-name = "mm1995tk/gh-editor";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [gomod2nix.overlays.default];
      };

      treefmtEval = treefmt-nix.lib.evalModule pkgs ./flake-treefmt.nix;

      appDrv = pkgs.buildGoApplication {
        pname = "gh-editor";
        version = "0.1";
        pwd = ./.;
        src = ./.;
        modules = ./gomod2nix.toml;
        CGO_ENABLED = 0;
      };
    in {
      packages = {
        default = appDrv;
        gh-editor = appDrv;
        treefmt = treefmtEval.config.build.configFile;
      };

      apps.default = flake-utils.lib.mkApp {drv = appDrv;};

      devShells = import ./flake-shell.nix {inherit pkgs project-name;};
      formatter = treefmtEval.config.build.wrapper;
      checks.formatting = treefmtEval.config.build.check self;
    });
}
