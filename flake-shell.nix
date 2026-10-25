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
    ];

    CGO_ENABLED = "0";
    PROJECT_NAME = project-name;
  };
}
