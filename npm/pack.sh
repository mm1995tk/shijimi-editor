#!/usr/bin/env bash
# Builds shj for each OS and CPU and packs the npm packages into OUT_DIR.
#
#   npm/pack.sh VERSION OUT_DIR
#
# VERSION is X.Y.Z without the leading "v". OUT_DIR gets one tarball for each
# platform package (shijimi-editor-<os>-<cpu>-VERSION.tgz) and one for
# shijimi-editor itself, which depends on them as optionalDependencies.
# Needs go, node and npm. Windows is left out: the build fails there
# (internal/kitty uses unix.IoctlGetWinsize).
set -euo pipefail

version="${1:?usage: npm/pack.sh VERSION OUT_DIR}"
out="${2:?usage: npm/pack.sh VERSION OUT_DIR}"
if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: VERSION must be X.Y.Z, got $version" >&2
  exit 1
fi

root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

commit="$(git -C "$root" rev-parse --short HEAD)"
if [ -n "$(git -C "$root" status --porcelain)" ]; then
  commit="$commit-dirty"
fi
# The same ldflags as flake.nix, plus -s -w to drop the symbol table and DWARF.
ldflags="-s -w"
ldflags+=" -X github.com/mm1995tk/shijimi-editor/internal/ui.Version=$version"
ldflags+=" -X github.com/mm1995tk/shijimi-editor/internal/ui.Commit=$commit"

# Go's GOOS/GOARCH next to the os/cpu names npm and Node.js use.
targets=(
  "linux amd64 linux x64"
  "linux arm64 linux arm64"
  "darwin amd64 darwin x64"
  "darwin arm64 darwin arm64"
)

names=()
for t in "${targets[@]}"; do
  read -r goos goarch os cpu <<<"$t"
  name="shijimi-editor-$os-$cpu"
  names+=("$name")
  dir="$work/$name"
  mkdir -p "$dir/bin"
  (cd "$root" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "$ldflags" -o "$dir/bin/shj" ./cmd/shj)
  cp "$root/LICENSE" "$dir/"
  # shellcheck disable=SC2016 # ${OS} and ${CPU} are a JavaScript template literal, not shell
  NAME="$name" VERSION="$version" OS="$os" CPU="$cpu" node -e '
    const { NAME, VERSION, OS, CPU } = process.env;
    const pkg = {
      name: NAME,
      version: VERSION,
      description: `The shj binary of shijimi-editor for ${OS} ${CPU}`,
      license: "MIT",
      homepage: "https://github.com/mm1995tk/shijimi-editor",
      repository: { type: "git", url: "git+https://github.com/mm1995tk/shijimi-editor.git" },
      os: [OS],
      cpu: [CPU],
      files: ["bin"],
    };
    require("fs").writeFileSync(process.argv[1], JSON.stringify(pkg, null, 2) + "\n");
  ' "$dir/package.json"
  (cd "$dir" && npm pack --pack-destination "$out" --loglevel warn >/dev/null)
done

dir="$work/shijimi-editor"
cp -R "$root/npm/shijimi-editor" "$dir"
cp "$root/LICENSE" "$root/README.md" "$dir/"
# optionalDependencies is made from targets, so the list of platforms is only there.
VERSION="$version" NAMES="${names[*]}" node -e '
  const fs = require("fs");
  const file = process.argv[1];
  const pkg = JSON.parse(fs.readFileSync(file, "utf8"));
  pkg.version = process.env.VERSION;
  pkg.optionalDependencies = Object.fromEntries(
    process.env.NAMES.split(" ").map((name) => [name, process.env.VERSION]),
  );
  fs.writeFileSync(file, JSON.stringify(pkg, null, 2) + "\n");
' "$dir/package.json"
(cd "$dir" && npm pack --pack-destination "$out" --loglevel warn >/dev/null)

ls -1 "$out"/*.tgz
