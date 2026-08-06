# Meerkit monitor plugins

Each plugin is an independent Go module and communicates with Meerkit through the public SDK and HashiCorp go-plugin gRPC transport. Meerkit imports only `.zip` and `.tar.gz` packages; copying a raw executable into the runtime directory never loads it.

## Plugin development workflow

1. Create an independent Go module from `plugins/template`, implement the SDK `Module` interface, and add contract tests. During repository development, `go run .` builds and runs every source plugin under `plugins` except `template`.
2. Fill in a unique ID, semantic version, vendor, `desp`, source or release `url`, protocol range, and module versions in `meerkit-plugin.yaml`.
3. Build an archive with `scripts/package-plugins.sh --plugin`. The tool cross-compiles artifacts and writes their platform, size, and SHA-256 automatically.
4. Sign production releases with a long-lived Ed25519 private key. Use a separate test key during development.
5. Users verify and trust the public-key fingerprint on first import. Later plugins signed by the same key are verified automatically.

## Trust model

- Official releases: the release pipeline signs bundled HTTP, TCP, and other official plugins with one stable official key. On a fresh data directory, bundled official plugins bootstrap local official trust, so later manually imported packages signed by that key are trusted without a public key service or configuration changes.
- Third-party releases: signed packages carry the public key. Meerkit verifies the manifest signature independently, then displays the key ID and SHA-256 public-key fingerprint. A user's confirmation is persisted in the local database.
- Self-hosting: unsigned packages remain usable when nothing is distributed externally. They require explicit risk confirmation when enabled and cannot prove publisher identity.
- Preconfigured trust: unattended deployments may configure Base64 Ed25519 public keys under `plugins.trusted_keys`. This is optional for normal interactive imports.

A signature proves only that the package came from the private-key holder and was not modified. It does not prove that plugin code is safe. Verify a third-party fingerprint through an independent source or release page before trusting it.

## Packaging

Examples for the current platform, multiple platforms, and a combined archive:

```sh
scripts/package-plugins.sh --plugin ./plugins/http
scripts/package-plugins.sh --plugin ./plugins/http --targets linux/amd64,linux/arm64,windows/amd64,darwin/arm64
scripts/package-plugins.sh --plugin ./plugins/http --targets linux/amd64,windows/amd64 --combined
```

Generate an Ed25519 key pair. The private key is only for package signing and must not be included in a plugin package or committed to the repository:

```sh
scripts/package-plugins.sh --generate-key ./keys/meerkit-release
```

Package the plugin with a stable key ID and private key:

```sh
scripts/package-plugins.sh \
  --plugin ./plugins/http \
  --targets current \
  --sign-key ./keys/meerkit-release.private.key \
  --key-id meerkit-release-2026
```

Copy packages to `${data_dir}/plugins/inbox` or import them from the management page. Signatures cover the manifest, artifact hashes declared by the manifest, and README/license documents. Unknown keys are marked as pending trust; confirmed, preconfigured, or officially bootstrapped keys are verified automatically. Different content with an existing plugin ID and version is rejected until the old version is uninstalled or the manifest version is incremented.

Source development mode is exempt from that last package rule: a `dev` host replaces same-ID, same-version development binaries on every startup. Build the host with a release version when validating the real package import and signature workflow.

Set `MEERKIT_PLUGIN_SIGN_KEY` and `MEERKIT_PLUGIN_KEY_ID` when running `scripts/package.sh` for an official full release so every bundled plugin uses the same release key. Losing or replacing the key requires existing users to confirm a new fingerprint, so back up the private key and plan key rotations deliberately. Plugin processes have the same OS permissions as Meerkit and are not a security sandbox.
