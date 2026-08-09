# Build and release scripts

[中文](README.md) · [Release documentation](../docs/development/releasing.md)

This directory provides equivalent Unix Shell and Windows PowerShell release flows. The scripts build the Meerkit host, general conformance checker, and Go official plugins in this repository. They do not build third-party language plugins.

## Scripts

| Shell | PowerShell | Purpose |
| --- | --- | --- |
| `package-plugins.sh` | `package-plugins.ps1` | Build one or all official plugins, generate keys, and optionally sign packages |
| `package.sh` | `package.ps1` | Build the frontend, host, `meerkit-plugincheck`, and official plugin release archives |

Run scripts from the repository root. The default target is the current `GOOS/GOARCH`; separate multiple targets with commas.

```bash
scripts/package-plugins.sh --plugin plugins/http --targets current
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
scripts/package.sh dist/releases darwin/arm64,linux/amd64,windows/amd64
```

Bulk packaging skips `plugins/template` because it is source scaffolding.

## Signing

```bash
scripts/package-plugins.sh --generate-key ./keys/meerkit-release

MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-release.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-release-2026 \
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
```

The private key and key ID must be set together. Keep the private key in the release environment or a CI secret store. The generated public key can publish a fingerprint or preconfigure `plugins.trusted_keys`. Key generation refuses to overwrite existing files.

## Complete release

```bash
MEERKIT_VERSION=v0.1.0 \
MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-release.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-release-2026 \
scripts/package.sh dist/releases linux/amd64,linux/arm64
```

Each target archive contains:

```text
meerkit
meerkit-plugincheck
plugins/
README.md
README.en.md
LICENSE
config.example.yaml
```

Windows produces `.zip` archives and `.exe` files; other targets produce `.tar.gz`. All Go builds use `CGO_ENABLED=0` and `-trimpath`. Official plugins in a complete release share the same signing key.

The root `Makefile` exposes the common flows as `make generate-key`, `make package-plugin`, `make package-plugins`, and `make package-release`.

## License

These scripts are licensed with Meerkit under the [Apache License 2.0](../LICENSE), and complete release archives include the root `LICENSE`.
