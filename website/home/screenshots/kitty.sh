#!/usr/bin/env bash
# Takes the screenshots of one scene in kitty, for what the terminal inside vhs
# cannot draw: images and mermaid diagrams need the Kitty graphics protocol.
# Called by record.sh inside Xvfb.
# usage: kitty.sh SCENE OUTDIR
set -euo pipefail

scene=$1
out=$2

kitty -c NONE \
  -o font_family="JetBrainsMono Nerd Font" -o font_size=11 \
  -o remember_window_size=no -o initial_window_width=1440 -o initial_window_height=860 \
  -o window_padding_width=0 -o confirm_os_window_close=0 \
  --directory "$DEMO" shj &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT

win=$(xdotool search --sync --class kitty | head -1)
xdotool windowfocus --sync "$win"
sleep 4

# The helpers that the scenes use.
press() {
  xdotool key --delay 80 "$@"
  sleep 0.5
}
typing() {
  xdotool type --delay 60 "$1"
  sleep 0.5
}
# The whole screen: grabbing the kitty window alone leaves out the images.
shot() { import -window root "$out/$1"; }

# shellcheck source=/dev/null
source "$scene"
