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

          version = "0.8.0";

          # dirtyShortRev ends in "-dirty" when built from uncommitted changes
          commit = self.shortRev or self.dirtyShortRev or "";

          # A nightly is the stable X.Y.Z plus "-nightly", the date of the
          # commit and its hash, like the npm version (see packaging/npm/pack.sh).
          nightlyVersion = "${version}-nightly.${
            builtins.substring 0 8 (self.lastModifiedDate or "00000000")
          }.${commit}";

          # The GUI (kisui-terminal) has its own version, apart from shj (#581). It lives in
          # the Cargo.toml of the Tauri shell; bump it there to release the GUI.
          guiVersion =
            (builtins.fromTOML (builtins.readFile ./ui/gui/app/src-tauri/Cargo.toml)).package.version;
          guiNightlyVersion = "${guiVersion}-nightly.${
            builtins.substring 0 8 (self.lastModifiedDate or "00000000")
          }.${commit}";

          # One build for both channels. The stable shj and the nightly shj-nightly
          # are the same program; the channel is compiled in and only changes the
          # names that let the two be installed side by side (see ui/internal/buildinfo).
          mkApp =
            {
              channel ? "",
            }:
            let
              suffix = pkgs.lib.optionalString (channel != "") "-${channel}";
              bin = "shj${suffix}";
              alias = "shijimi${suffix}";
              # The binary itself gets X.Y.Z and the channel, and shows "vX.Y.Z-nightly".
              pkgVersion = if channel == "" then version else nightlyVersion;
            in
            pkgs.buildGoApplication {
              go = pkgs.go_1_27;
              pname = alias;
              version = pkgVersion;
              pwd = ./.;
              src = ./.;
              modules = ./gomod2nix.toml;
              subPackages = [
                "cmd/shj"
                "cmd/kisui"
              ];
              CGO_ENABLED = 0;
              ldflags = [
                "-X github.com/mm1995tk/shijimi-editor/ui/internal/buildinfo.Version=${version}"
                "-X github.com/mm1995tk/shijimi-editor/ui/internal/buildinfo.Commit=${commit}"
                "-X github.com/mm1995tk/shijimi-editor/ui/internal/buildinfo.Channel=${channel}"
              ];
              # The same binary also answers to "shijimi" (or "shijimi-nightly").
              # kisui, the server of the GUI, is named kisui-nightly on the nightly channel.
              postInstall = ''
                ${pkgs.lib.optionalString (suffix != "") "mv $out/bin/kisui $out/bin/kisui${suffix}"}
                ${pkgs.lib.optionalString (bin != "shj") "mv $out/bin/shj $out/bin/${bin}"}
                ln -s ${bin} $out/bin/${alias}
              '';
              meta = {
                description = "VS Code-style terminal IDE with Helix keybindings";
                license = pkgs.lib.licenses.mit;
                mainProgram = bin;
              };
            };

          # kisui-terminal (the GUI). Built with Nix it has no auto update (update it with nix flake update).
          guiDrv = import ./packaging/nix/gui.nix {
            inherit pkgs;
            version = guiVersion;
            # The kisui of the same build, so the GUI and its server always match.
            kisui = "${appDrv}/bin/kisui";
            # And its shj, which the terminals in the panes use first.
            shj = "${appDrv}/bin/shj";
          };
          # The nightly kisui-terminal, installed as kisui-terminal-nightly.
          guiNightlyDrv = import ./packaging/nix/gui.nix {
            inherit pkgs;
            version = guiNightlyVersion;
            kisui = "${nightlyDrv}/bin/kisui-nightly";
            shj = "${nightlyDrv}/bin/shj-nightly";
            channel = "nightly";
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
            kisui-terminal = guiDrv;
            kisui-terminal-nightly = guiNightlyDrv;
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

          # GUI を開発用に起動する（tauri dev）。リポジトリのどこからでも nix run .#gui-dev で動く。
          # devShell の中で動かすので、Node・Rust・WebView のライブラリは devShell と同じものになる。
          # devShell の中なら、根で npm run tauri dev と同じ
          apps.gui-dev = flake-utils.lib.mkApp {
            drv = pkgs.writeShellApplication {
              name = "gui-dev";
              runtimeInputs = [ pkgs.git ];
              text = ''
                root="$(git rev-parse --show-toplevel)"
                cd "$root"
                # node_modules が無いか package-lock.json より古ければ入れ直す
                exec nix develop "$root" --command bash -c '
                  if [ ! -f node_modules/.package-lock.json ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then
                    npm ci
                  fi
                  # GUI をつなぐ kisui（同梱の代わり）。GUI は SHIJIMI_KISUI を先に見る（locate.rs）
                  go build -o target/dev/kisui ./cmd/kisui
                  export SHIJIMI_KISUI="$PWD/target/dev/kisui"
                  exec npm run tauri dev -- "$@"
                ' gui-dev "$@"
              '';
            };
          };

          devShells = import ./flake-shell.nix { inherit pkgs project-name; };
          formatter = treefmtEval.config.build.wrapper;

          # nix flake check で回す検査。CI（check.yml）も同じものを回す
          checks = {
            formatting = treefmtEval.config.build.check self;

            # GUI の殻（cargo test も回る）と Web 画面のテスト（npm test）
            kisui-terminal = guiDrv;
            gui-web-test = guiDrv.web.overrideAttrs (_: {
              pname = "kisui-terminal-web-test";
              doCheck = true;
              checkPhase = ''
                runHook preCheck
                npm test
                runHook postCheck
              '';
            });
          }
          # Go の全パッケージのテスト。packages の shj は subPackages が cmd/shj だけなので、
          # そのままでは core/・ui/ のテストが回らない。入れるときのビルドは重くしたくないので、ここだけで回す。
          # Linux だけ。macOS の Nix のビルド環境には PATH に ps が無く、ソケットのパスが長すぎ、
          # PTY のシェルも動かないので、テストが前提にする環境にならない。macOS では CI（check.yml）が
          # devShell の中で go test ./... を回す
          // pkgs.lib.optionalAttrs pkgs.stdenv.hostPlatform.isLinux {
            go-test = appDrv.overrideAttrs (_: {
              pname = "shijimi-go-test";
              nativeCheckInputs = [ pkgs.git ];
              checkPhase = ''
                runHook preCheck
                export HOME="$TMPDIR/home"
                mkdir -p "$HOME"
                go test ./...
                runHook postCheck
              '';
            });
          };
        }
      );
    in
    perSystem
    // {
      homeManagerModules.default = import ./packaging/nix/home-manager-module.nix self;
      homeManagerModules.shijimi-editor = self.homeManagerModules.default;
    };
}
