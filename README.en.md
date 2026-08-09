<p align="center">
  <img src="web/public/brand-mark.png" width="72" height="72" alt="Meerkit">
</p>

# Meerkit

[中文](README.md) · [Documentation](docs/index.md) · [Plugin protocol](sdk/PROTOCOL.en.md) · [Apache-2.0](LICENSE)

Meerkit is a self-hosted monitoring service. It schedules checks, stores observations, evaluates conditions, and sends notifications while independent plugin processes provide the actual probing capabilities. This repository currently ships HTTP and TCP plugins together with Webhook, SMTP, and in-app notification channels.

## Current capabilities

- One host binary serves the management UI, HTTP API, cron scheduler, cleanup worker, and plugin runtime.
- Create, test, enable, disable, and manually run monitors; inspect execution history, structured results, and notification deliveries.
- Combine conditions with `ALL` or `ANY` and compare current, previous, or literal values using change, text, regex, numeric, and existence operators.
- Build a status board from condition states or result fields with boolean mappings, numeric thresholds, history windows, and trend rules.
- Store configuration, records, notifications, and plugin state in SQLite by default or in MySQL.
- Inspect startup configuration sources, runtime configuration, host logs, and plugin logs from the web UI.
- Install `.zip` or `.tar.gz` plugins with artifact hash verification, Ed25519 signatures, and publisher trust management.
- Implement third-party plugins in other languages through the gRPC + JSON wire protocol, supported by Proto, JSON Schemas, and a black-box conformance tool.

## Quick start

Source development requires Go 1.26, Node.js 22, npm, and Make.

```bash
make deps
make dev
```

Open `http://127.0.0.1:5173`. The frontend development server proxies API and WebSocket traffic to `http://127.0.0.1:8080`. The first visit asks you to set an administrator access key containing at least 12 characters.

To run only the host with its embedded production UI:

```bash
make frontend-build
go run . serve --listen 127.0.0.1:8080
```

The first start creates `config.yaml` in the working directory. Data and logs default to `./data` and `./logs`, and the default database is `${storage.data_dir}/meerkit.db`.

> `make reset` deletes the default data directory, logs, configuration, and local SQLite files. It is intended for development resets, not production use.

The complete user documentation starts at [Getting started](docs/guide/getting-started.md). The VitePress site is currently written in Chinese; protocol and repository READMEs remain bilingual.

## Plugin model

Plugins are local child processes launched and supervised by the host. HashiCorp go-plugin establishes a local gRPC connection, and application payloads are UTF-8 JSON. Unix uses a Unix Domain Socket and Windows uses loopback TCP. A plugin is neither a remote HTTP service nor a sandboxed process.

Official plugins continue to use the Go SDK. A third-party plugin may use any language that can implement the gRPC service and produce a single-file artifact:

- Go starting points: [plugins/template](plugins/template/README.en.md) and [sdk](sdk/README.en.md)
- Language-neutral contract: [sdk/PROTOCOL.en.md](sdk/PROTOCOL.en.md)
- Machine-readable contract: [Proto](sdk/proto/monitor.proto) and [JSON Schemas](sdk/schema/)
- Black-box check: `go run ./cmd/plugincheck --manifest ... --artifact ... --suite ...`

Meerkit's packager builds only the Go plugins in this repository. Third-party maintainers build their own artifacts and select direct execution or a host interpreter through `artifacts[].runtime` in the manifest.

## Common commands

| Command | Purpose |
| --- | --- |
| `make dev` | Start the Go backend and Vite frontend |
| `make frontend-build` | Build the UI embedded by the host |
| `make docs-dev` | Start the VitePress documentation site |
| `make docs-build` | Build the static documentation site |
| `go test ./...` | Test the root Go module |
| `make package-plugins` | Package all publishable official plugins |
| `make package-release VERSION=v0.1.0` | Build host, checker, and official plugin archives |
| `meerkit admin reset-key --key '...'` | Reset the administrator key and revoke all sessions |

Run `make help` or `go run . --help` for the complete command reference.

## Repository layout

```text
cmd/              plugin packaging and conformance tools
docs/             VitePress documentation site
internal/         API, scheduling, storage, auth, plugin, and runtime code
plugins/          HTTP and TCP plugins plus the Go plugin template
scripts/          Unix and Windows packaging scripts
sdk/              Go SDK, Proto, JSON Schemas, and protocol documents
web/              React/Vite management UI
```

## License

Meerkit source code and project documentation are licensed under the [Apache License 2.0](LICENSE). Third-party dependencies and plugins remain subject to their own licenses.
