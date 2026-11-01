# GUI（gui/）のパッケージ。Tauri の殻（Rust）と、Solid の Web 画面（npm）を 1 つにする。
#   web … buildNpmPackage で vite build した dist
#   殻  … buildRustPackage。dist を gui/dist に置いてから cargo build する
#          （tauri が generate_context! で dist を実行ファイルに埋め込む）
{ pkgs, version }:
let
  inherit (pkgs) lib stdenv;

  web = pkgs.buildNpmPackage {
    pname = "shijimi-editor-gui-web";
    inherit version;
    src = ../gui;
    # package-lock.json を変えたら、ここを lib.fakeHash にして nix build .#gui を回し、出た値に直す
    npmDepsHash = "sha256-ZLHV1zITAwPQMNfFR5r4E72gIYtObL5QoXCQdliUNdQ=";
    # tauri の CLI は殻のビルドには使わない（cargo を直接回す）。バイナリの取得を避ける
    npmFlags = [ "--ignore-scripts" ];
    installPhase = ''
      runHook preInstall
      cp -r dist $out
      runHook postInstall
    '';
  };
in
pkgs.rustPlatform.buildRustPackage {
  pname = "shijimi-editor-gui";
  inherit version;
  src = ../gui;
  sourceRoot = "gui/src-tauri";
  cargoLock.lockFile = ../gui/src-tauri/Cargo.lock;

  # release の画面は埋め込んだ Web を使う（cargo tauri build が自動で付ける feature）
  buildFeatures = [ "custom-protocol" ];

  postPatch = ''
    chmod u+w ..
    mkdir -p ../dist
    cp -r ${web}/. ../dist/
  '';

  nativeBuildInputs = [
    pkgs.pkg-config
  ]
  ++ lib.optionals stdenv.hostPlatform.isLinux [ pkgs.wrapGAppsHook3 ];
  buildInputs = lib.optionals stdenv.hostPlatform.isLinux [
    pkgs.webkitgtk_4_1
    pkgs.gtk3
    pkgs.libsoup_3
    pkgs.glib
    pkgs.openssl
    pkgs.glib-networking
  ];

  doCheck = false;

  meta = {
    description = "GUI for shijimi-editor (Tauri)";
    license = lib.licenses.mit;
    mainProgram = "shijimi-editor-gui";
    platforms = lib.platforms.linux ++ lib.platforms.darwin;
  };
}
