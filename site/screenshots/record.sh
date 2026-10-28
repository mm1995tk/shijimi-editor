#!/usr/bin/env bash
# Records the screenshots and the demo video of the home page.
# The tapes in tapes/ are recorded with vhs, and the scenes in kitty/ are taken
# in kitty, because images and mermaid diagrams need the Kitty graphics protocol.
# usage: site/screenshots/record.sh [TAPE_OR_SCENE...]   (default: all of them)
# Needs Nix. The results go to site/images/.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

out() { nix build --no-link --print-out-paths "$1" | grep -v -e '-man$' -e '-doc$' -e '-info$' -e '-dev$' | head -1; }

shj=$(nix build --no-link --print-out-paths "$repo#default")
bash_bin=$(out nixpkgs#bashInteractive)/bin/bash
mesa=$(out nixpkgs#mesa)

# A home of its own, so that the user's shell prompt and settings do not show up.
mkdir -p "$work/home" "$work/config"
printf 'PS1="\\[\\e[36m\\]todo\\[\\e[0m\\] \\$ "\n' >"$work/home/.bashrc"

# The fonts that vhs and kitty draw with.
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
export PATH=$shj/bin:$(out nixpkgs#go)/bin:$(out nixpkgs#gopls)/bin:$(out nixpkgs#mermaid-cli)/bin:$PATH
export DEMO=$work/todo

if [ $# -eq 0 ]; then
  set -- "$here"/tapes/*.tape "$here"/kitty/*.sh
fi
files=()
for f in "$@"; do
  files+=("$(cd "$(dirname "$f")" && pwd)/$(basename "$f")")
done

mkdir -p "$work/out"
cd "$work/out"
for f in "${files[@]}"; do
  # Each one starts from the same project, because some of them save files.
  "$here/demo.sh" "$DEMO"
  echo "recording $f"
  case $f in
  *.tape)
    nix shell nixpkgs#vhs nixpkgs#ttyd nixpkgs#ffmpeg nixpkgs#chromium -c vhs "$f"
    ;;
  *.sh)
    # kitty needs OpenGL, which Xvfb gets from mesa's software renderer.
    LD_LIBRARY_PATH=$mesa/lib LIBGL_DRIVERS_PATH=$mesa/lib/dri __GLX_VENDOR_LIBRARY_NAME=mesa \
      nix shell nixpkgs#kitty nixpkgs#xvfb-run nixpkgs#xdotool nixpkgs#imagemagick -c \
      xvfb-run -a -s "-screen 0 1440x860x24 +extension GLX" "$here/kitty.sh" "$f" "$work/out"
    ;;
  esac
done

# The tapes that only take screenshots also write a GIF; keep just what the page uses.
cp -f "$work"/out/*.png "$work"/out/*.mp4 "$repo/site/images/" 2>/dev/null || true
