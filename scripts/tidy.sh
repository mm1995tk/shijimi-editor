#!/usr/bin/env bash
# リポジトリを整える。手元でも CI でも、これ 1 つを動かす（nix develop -c scripts/tidy.sh）。
#
# 順番に意味がある:
#   go mod tidy → gomod2nix は go.mod と go.sum から gomod2nix.toml を作るので、tidy のあと
#   go fix      → コードを書き換える（gopls が「シンプルに書ける」と出す modernize の修正）ので、nix fmt はそのあと
set -euo pipefail
cd "$(dirname "$0")/.."

go mod tidy
gomod2nix
go fix ./...
nix fmt
