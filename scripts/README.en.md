# Packaging scripts

- `package-plugins.sh --plugin <directory> [pluginpack arguments]` packages one plugin; use `--generate-key` to generate signing keys.
- `package-plugins.sh [output] [targets]` packages every publishable plugin under `plugins/` when `--plugin` is omitted. Targets use `GOOS/GOARCH` and may be comma-separated.
- `package.sh [output] [targets]` builds the frontend, host executable, and all publishable plugins into complete release archives.
- PowerShell equivalents provide the same workflow on Windows.

```sh
scripts/package-plugins.sh --plugin plugins/http --targets current
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
scripts/package.sh dist/releases darwin/arm64,linux/amd64,windows/amd64
```

To sign every generated plugin package, first generate a key pair with `pluginpack --generate-key`, then set the signing private key and key ID for the script. `package.sh` passes these environment variables to its plugin packaging step:

```sh
scripts/package-plugins.sh --generate-key ./keys/meerkit-release
MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-release.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-release-2026 \
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
```

`package.sh` uses the same environment variables to sign every bundled plugin in a complete release. Bundled plugins bootstrap official trust on first start with an empty data directory; users confirm third-party fingerprints once. `plugins.trusted_keys` remains available only for optional preconfigured trust. Never commit the private key or include it in a release archive. See [`plugins/README.en.md`](../plugins/README.en.md) for the complete procedure.

`plugins/template` is source scaffolding rather than a distributable monitor and is the only directory excluded from automatic release packaging.
