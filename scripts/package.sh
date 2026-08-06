#!/bin/sh
set -eu

output="${1:-dist/releases}"
targets="${2:-$(go env GOOS)/$(go env GOARCH)}"
version="${MEERKIT_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
mkdir -p "$output"
output=$(cd "$output" && pwd)

npm --prefix web run build

old_ifs=$IFS
IFS=','
for target in $targets; do
  IFS=$old_ifs
  goos=${target%/*}
  goarch=${target#*/}
  [ "$goos" != "$target" ] || { echo "Invalid target: $target" >&2; exit 1; }
  stage=$(mktemp -d "${TMPDIR:-/tmp}/meerkit-release.XXXXXX")
  release_dir="$stage/meerkit-$version-$goos-$goarch"
  mkdir -p "$release_dir/plugins"
  binary="meerkit"
  [ "$goos" = "windows" ] && binary="meerkit.exe"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$release_dir/$binary" .
  scripts/package-plugins.sh "$release_dir/plugins" "$goos/$goarch"
  cp README.md README.en.md config.example.yaml "$release_dir/"
  if [ "$goos" = "windows" ]; then
    (cd "$stage" && zip -qr "$output/meerkit-$version-$goos-$goarch.zip" "$(basename "$release_dir")")
  else
    tar -C "$stage" -czf "$output/meerkit-$version-$goos-$goarch.tar.gz" "$(basename "$release_dir")"
  fi
  rm -rf "$stage"
  IFS=','
done
IFS=$old_ifs
