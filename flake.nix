{
  description = "shijimi: VS Code-style terminal IDE with Helix keybindings";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix.url = "github:nix-community/gomod2nix";
    gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
    gomod2nix.inputs.flake-utils.follows = "flake-utils";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      gomod2nix,
      treefmt-nix,
    }:
    let
      perSystem = flake-utils.lib.eachDefaultSystem (
        system:
        let
          project-name = "mm1995tk/shijimi-editor";
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ gomod2nix.overlays.default ];
          };

          treefmtEval = treefmt-nix.lib.evalModule pkgs ./flake-treefmt.nix;

          version = "0.6.1";

          # dirtyShortRev ends in "-dirty" when built from uncommitted changes
          commit = self.shortRev or self.dirtyShortRev or "";

          # One build for both channels. The stable shj and the nightly shj-nightly
          # are the same program; the channel is compiled in and only changes the
          # names that let the two be installed side by side (see internal/channel).
          mkApp =
            {
              channel ? "",
            }:
            let
              suffix = pkgs.lib.optionalString (channel != "") "-${channel}";
              bin = "shj${suffix}";
              alias = "shijimi${suffix}";
            in
            pkgs.buildGoApplication {
              go = pkgs.go_1_27;
              pname = alias;
              # A nightly is the stable X.Y.Z plus "-nightly", the date of the
              # commit and its hash, like the npm version (see npm/pack.sh). The
              # binary itself gets X.Y.Z and the channel, and shows "vX.Y.Z-nightly".
              version =
                version
                +
                  pkgs.lib.optionalString (channel != "")
                    "${suffix}.${builtins.substring 0 8 (self.lastModifiedDate or "00000000")}.${commit}";
              pwd = ./.;
              src = ./.;
              modules = ./gomod2nix.toml;
              subPackages = [ "cmd/shj" ];
              CGO_ENABLED = 0;
              ldflags = [
                "-X github.com/mm1995tk/shijimi-editor/internal/ui.Version=${version}"
                "-X github.com/mm1995tk/shijimi-editor/internal/ui.Commit=${commit}"
                "-X github.com/mm1995tk/shijimi-editor/internal/channel.Channel=${channel}"
              ];
              # The same binary also answers to "shijimi" (or "shijimi-nightly").
              postInstall = ''
                ${pkgs.lib.optionalString (bin != "shj") "mv $out/bin/shj $out/bin/${bin}"}
                ln -s ${bin} $out/bin/${alias}
              '';
              meta = {
                description = "VS Code-style terminal IDE with Helix keybindings";
                license = pkgs.lib.licenses.mit;
                mainProgram = bin;
              };
            };

          appDrv = mkApp { };
          # The nightly build of main, installed as shj-nightly so it can sit next to shj.
          nightlyDrv = mkApp { channel = "nightly"; };
        in
        {
          packages = {
            default = appDrv;
            shijimi = appDrv;
            shj-nightly = nightlyDrv;
            treefmt = treefmtEval.config.build.configFile;
          };

          apps.default = flake-utils.lib.mkApp { drv = appDrv; };

          # リポジトリを整える（scripts/tidy.sh）。devShell に入らなくても nix run .#tidy で動かせる。
          # nix fmt は $PATH の nix を使う（runtimeInputs は PATH の前に足されるだけなので、システムの nix が見える）。
          apps.tidy = flake-utils.lib.mkApp {
            drv = pkgs.writeShellApplication {
              name = "tidy";
              runtimeInputs = [
                pkgs.go_1_27
                pkgs.gomod2nix
                pkgs.git
              ];
              text = "exec ${./scripts/tidy.sh}";
            };
          };

          devShells = import ./flake-shell.nix { inherit pkgs project-name; };
          formatter = treefmtEval.config.build.wrapper;
          checks.formatting = treefmtEval.config.build.check self;
        }
      );
    in
    perSystem
    // {
      homeManagerModules.default = import ./nix/home-manager-module.nix self;
      homeManagerModules.shijimi-editor = self.homeManagerModules.default;
    };
}
