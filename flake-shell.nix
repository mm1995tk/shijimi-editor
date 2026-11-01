{
  pkgs,
  project-name,
  ...
}:
let
  goEnv = pkgs.mkGoEnv {
    pwd = ./.;
    go = pkgs.go_1_27;
  };
in
{
  default = pkgs.mkShell {
    buildInputs = [
      goEnv
      pkgs.gomod2nix
      pkgs.gopls
      pkgs.git
      # npm/pack.sh needs node and npm
      pkgs.nodejs_26
      # gui/（Tauri）のビルド。Linux の WebView 周りの C ライブラリは nix build .#gui が持つ
      pkgs.cargo
      pkgs.rustc
    ];

    CGO_ENABLED = "0";
    PROJECT_NAME = project-name;
  };
}
