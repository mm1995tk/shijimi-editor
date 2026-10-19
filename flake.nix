{
  description = "dageki: VS Code-style terminal IDE with Helix keybindings";

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
  }: let
    perSystem = flake-utils.lib.eachDefaultSystem (system: let
      project-name = "mm1995tk/dageki";
      pkgs = import nixpkgs {
        inherit system;
        overlays = [gomod2nix.overlays.default];
      };

      treefmtEval = treefmt-nix.lib.evalModule pkgs ./flake-treefmt.nix;

      version = "0.3.0";

      appDrv = pkgs.buildGoApplication {
        pname = "dageki";
        inherit version;
        pwd = ./.;
        src = ./.;
        modules = ./gomod2nix.toml;
        subPackages = ["cmd/dgk"];
        CGO_ENABLED = 0;
        ldflags = [
          "-X github.com/mm1995tk/dageki/internal/ui.Version=${version}"
          # dirtyShortRev ends in "-dirty" when built from uncommitted changes
          "-X github.com/mm1995tk/dageki/internal/ui.Commit=${self.shortRev or self.dirtyShortRev or ""}"
        ];
        meta.mainProgram = "dgk";
      };
    in {
      packages = {
        default = appDrv;
        dageki = appDrv;
        treefmt = treefmtEval.config.build.configFile;
      };

      apps.default = flake-utils.lib.mkApp {drv = appDrv;};

      devShells = import ./flake-shell.nix {inherit pkgs project-name;};
      formatter = treefmtEval.config.build.wrapper;
      checks.formatting = treefmtEval.config.build.check self;
    });
  in
    perSystem
    // {
      homeManagerModules.default = import ./nix/home-manager-module.nix self;
      homeManagerModules.dageki = self.homeManagerModules.default;
    };
}
