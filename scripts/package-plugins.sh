#!/bin/sh
set -eu

for argument in "$@"; do
  if [ "$argument" = "--plugin" ] || [ "$argument" = "--generate-key" ]; then
    exec go run ./cmd/pluginpack "$@"
  fi
done

output="${1:-dist/plugins}"
targets="${2:-current}"
sign_key="${MEERKIT_PLUGIN_SIGN_KEY:-}"
key_id="${MEERKIT_PLUGIN_KEY_ID:-}"
mkdir -p "$output"

if { [ -n "$sign_key" ] && [ -z "$key_id" ]; } || { [ -z "$sign_key" ] && [ -n "$key_id" ]; }; then
  echo "MEERKIT_PLUGIN_SIGN_KEY and MEERKIT_PLUGIN_KEY_ID must be set together" >&2
  exit 1
fi

found=0
for manifest in plugins/*/meerkit-plugin.yaml; do
  [ -f "$manifest" ] || continue
  plugin_dir=$(dirname "$manifest")
  [ "$(basename "$plugin_dir")" = "template" ] && continue
  found=1
  if [ -n "$sign_key" ]; then
    go run ./cmd/pluginpack --plugin "$plugin_dir" --output "$output" --targets "$targets" --sign-key "$sign_key" --key-id "$key_id"
  else
    go run ./cmd/pluginpack --plugin "$plugin_dir" --output "$output" --targets "$targets"
  fi
done

[ "$found" -eq 1 ] || { echo "No publishable plugins found under plugins/" >&2; exit 1; }
