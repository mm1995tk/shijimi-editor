#!/usr/bin/env bash
# Records the screenshots and the demo video of the home page with vhs.
# usage: site/screenshots/record.sh [TAPE...]   (default: every tape in tapes/)
# Needs Nix. The results go to site/images/.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

out() { nix build --no-link --print-out-paths "$1" | grep -v -e '-man$' -e '-doc$' -e '-info$' -e '-dev$' | head -1; }

shj=$(nix build --no-link --print-out-paths "$repo#default")
bash_bin=$(out nixpkgs#bashInteractive)/bin/bash

# A home of its own, so that the user's shell prompt and settings do not show up.
mkdir -p "$work/home" "$work/config"
printf 'PS1="\\[\\e[36m\\]todo\\[\\e[0m\\] \\$ "\n' >"$work/home/.bashrc"

# The fonts that the browser inside vhs draws with.
cat >"$work/fonts.conf" <<EOF
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
  <include ignore_missing="yes">$(out nixpkgs#fontconfig.out)/etc/fonts/fonts.conf</include>
  <dir>$(out nixpkgs#nerd-fonts.jetbrains-mono)/share/fonts</dir>
  <dir>$(out nixpkgs#noto-fonts-cjk-sans)/share/fonts</dir>
  <cachedir>$work/fc-cache</cachedir>
</fontconfig>
EOF

export HOME=$work/home SHELL=$bash_bin XDG_CONFIG_HOME=$work/config
export GOPATH=$work/go GOCACHE=$work/go-cache GOFLAGS=-modcacherw
export FONTCONFIG_FILE=$work/fonts.conf
export PATH=$shj/bin:$(out nixpkgs#go)/bin:$(out nixpkgs#gopls)/bin:$PATH
export DEMO=$work/todo

if [ $# -eq 0 ]; then
  set -- "$here"/tapes/*.tape
fi
tapes=()
for tape in "$@"; do
  tapes+=("$(cd "$(dirname "$tape")" && pwd)/$(basename "$tape")")
done

mkdir -p "$work/out"
cd "$work/out"
for tape in "${tapes[@]}"; do
  # Each tape starts from the same project, because some of them save files.
  "$here/demo.sh" "$DEMO"
  echo "recording $tape"
  nix shell nixpkgs#vhs nixpkgs#ttyd nixpkgs#ffmpeg nixpkgs#chromium -c vhs "$tape"
done

# The tapes that only take screenshots also write a GIF; keep just what the page uses.
cp -f "$work"/out/*.png "$work"/out/*.mp4 "$repo/site/images/" 2>/dev/null || true
