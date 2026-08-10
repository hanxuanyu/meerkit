# Meerkit monitor plugins

[中文](README.md) · [Plugin development](../docs/development/plugin-go.md) · [Language-neutral protocol](../sdk/PROTOCOL.en.md)

This directory contains the official monitor plugins and a source template. A monitor plugin describes configuration and results, validates configuration, executes probes, and migrates stored configuration. Scheduling, condition evaluation, persistence, and notification delivery remain host responsibilities.

## Contents

| Directory | Module type | Current capability |
| --- | --- | --- |
| [`http`](http/README.en.md) | `http` | HTTP/HTTPS requests, proxies, request bodies, redirects, TLS, and text/JSON response parsing |
| [`tcp`](tcp/README.en.md) | `tcp` | TCP connections, optional writes, one response read, text and Base64 payloads |
| [`template`](template/README.en.md) | `example` | Minimal Go implementation and a conformance suite |

Each plugin is an independent Go module. A development host scans manifests below `plugins.source_dir`, skips `template`, builds the plugin, and runs it from `${storage.data_dir}/plugins/development`. A release host runs installed packages only.

## Lifecycle

1. The host validates `meerkit-plugin.yaml` and selects exactly one artifact for the current `GOOS/GOARCH`.
2. It verifies the artifact size and SHA-256, then verifies `meerkit-plugin.sig` when present.
3. Enablement enforces the publisher state: official, trusted, untrusted, or unsigned.
4. The host starts the child process using the manifest-level `runtime` and any `artifacts[].runtime` override, then completes the go-plugin handshake plus two health checks.
5. The plugin returns module descriptors, which must agree with manifest types and versions.
6. The host migrates saved monitor configuration and atomically replaces the modules owned by that plugin.

Only one version of a plugin ID can be active. Disabling a plugin stops its process and removes its modules. Existing monitors remain stored but cannot execute until a compatible module is active again.

## Package format

Meerkit imports `.zip` and `.tar.gz` archives only. It never discovers a raw executable. A package root contains:

```text
meerkit-plugin.yaml
meerkit-plugin.sig       # optional Ed25519 signature envelope
README.md                # optional, shown in plugin details
README.en.md             # optional
LICENSE                  # recommended
bin/<goos>-<goarch>/...  # artifact declared by the manifest
```

The manifest Schema is [`manifest.schema.json`](manifest.schema.json). A source manifest must declare top-level `runtime` and may use `artifacts: []`; the repository packager preserves the startup configuration and fills platform, path, size, and SHA-256 for Go plugins. Other languages build and assemble their own artifacts against the same manifest and wire contracts.

## Trust model

- Official plugins bootstrapped from a Meerkit release establish `official` trust.
- A signed plugin from an unknown publisher cannot be enabled until the user verifies and confirms its public-key SHA-256 fingerprint.
- Later packages signed by the same public key are recognized as trusted.
- An unsigned package requires an explicit risk confirmation before enablement.
- `plugins.trusted_keys` can preconfigure Base64 Ed25519 public keys.

The signature binds the manifest, artifact hashes recorded by that manifest, and packaged README and LICENSE files. Signature verification identifies the publisher; it does not sandbox the process.

## Development and testing

```bash
(cd plugins/http && go test ./...)
(cd plugins/tcp && go test ./...)
(cd plugins/template && go test ./...)

# Let the development host build and run HTTP/TCP sources
go run . serve
```

Start a Go plugin from [`template`](template/README.en.md). A non-Go implementation should follow [`sdk/PROTOCOL.en.md`](../sdk/PROTOCOL.en.md), [`monitor.proto`](../sdk/proto/monitor.proto), and [`sdk/schema`](../sdk/schema/) without depending on Go interfaces.

See [Packaging and releases](../docs/development/releasing.md) and [`scripts/README.en.md`](../scripts/README.en.md) for build and signing commands.

## License

Official plugins in this repository are licensed with Meerkit under the [Apache License 2.0](../LICENSE). Third-party plugins may use another license and should include it as `LICENSE` in their package.
