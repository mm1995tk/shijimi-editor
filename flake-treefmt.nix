_: {
  projectRootFile = "flake.nix";
  programs.nixfmt.enable = true;
  programs.gofmt.enable = true;
  programs.taplo.enable = true;
  programs.rustfmt = {
    enable = true;
    edition = "2021";
  };
  # gui/ の Web 画面（Solid + TypeScript）。ほかのディレクトリの Markdown などには触れない
  programs.prettier = {
    enable = true;
    includes = [
      "gui/**/*.ts"
      "gui/**/*.tsx"
      "gui/**/*.html"
      "gui/**/*.json"
    ];
  };
  # gomod2nix writes this file; reformatting it only makes the next gomod2nix run show a diff.
  # package-lock.json は npm が書く。整形すると次の npm install で差分が出る
  settings.global.excludes = [
    "gomod2nix.toml"
    "gui/package-lock.json"
    "gui/src-tauri/icons/*"
    "gui/src-tauri/gen/*"
  ];
}
