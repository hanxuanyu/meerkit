# Meerkit

Meerkit is a monitoring service with independently managed monitor plugins and Webhook, SMTP, and in-app notifications. HTTP and TCP are distributed as official plugins rather than linked into the host process.

## Development

Local development requires Go, Node.js, npm, and Make. Install dependencies after the initial checkout:

```bash
make deps
```

Start the Go backend and Vite frontend together:

```bash
make dev
```

The frontend development server runs at `http://127.0.0.1:5173` and proxies API and WebSocket requests to `http://127.0.0.1:8080`. Pressing `Ctrl+C` stops both processes. You can also run them separately and pass additional options through variables:

```bash
make dev-frontend FRONTEND_ARGS="--host 0.0.0.0"
make dev-backend BACKEND_ARGS="--config config.yaml --listen 0.0.0.0:8080"
```

The backend embeds built frontend files. `make dev` and `make dev-backend` build them once when `web/dist` is missing. Run `make frontend-build` to build production frontend assets explicitly. `make help` lists the common targets and overridable variables.

### Clean and reset

`make clean` removes only reproducible build artifacts: `dist/`, `web/dist/`, `.gocache/`, and the root-level `meerkit`/`meerkit.exe` binaries. To restart the project from an empty runtime state, run:

```bash
make reset
```

`make reset` runs `clean`, then removes the default `data/`, `logs/`, and `config.yaml`, plus root-level `*.db`, `*.db-shm`, and `*.db-wal` SQLite files. This permanently deletes local runtime data and configuration. It preserves `keys/`, `web/node_modules/`, and other installed dependencies.

## Run

```bash
make frontend-build
make dev-backend BACKEND_ARGS="--config config.yaml"
```

The default UI is available at `http://127.0.0.1:8080`. On first access, set an administrator access key. All management APIs and notification streams then require an authenticated session. Local recovery is available with:

```bash
meerkit --data-dir ./data admin reset-key --key 'a-new-access-key'
```

`config.yaml`, `MEERKIT_*` environment variables, and command-line flags only define startup configuration: listen address and port, data directory, log file directory/name/rotation settings, master key file, plugin source directory, and trusted signing keys. Precedence is built-in defaults, config file, environment variables, then command-line flags. Nested environment variables use double underscores:

```bash
MEERKIT_SERVER__PORT=9090 MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit go run .
```

SQLite is the default database. When `dsn` is empty, Meerkit uses `${storage.data_dir}/meerkit.db`. For MySQL, create the database first; Meerkit can create and upgrade its tables, indexes, and built-in rows automatically:

```yaml
storage:
  data_dir: /var/lib/meerkit
  database:
    type: mysql
    dsn: meerkit:secret@tcp(mysql:3306)/meerkit?tls=true
    auto_migrate: true
    max_open_conns: 25
    max_idle_conns: 10
    conn_max_lifetime: 30m
    conn_max_idle_time: 5m
```

The same settings are available through `MEERKIT_STORAGE__DATABASE__TYPE`, `MEERKIT_STORAGE__DATABASE__DSN`, and the matching command-line flags. A DSN can contain credentials and is never exposed through configuration metadata or logs. When `auto_migrate` is disabled, startup still validates the schema version and refuses to run against a missing schema.

Retention and cleanup periods, scheduler settings, session TTL, host and plugin log levels/formats, and log output switches are runtime configuration. They are stored as JSON rows in the selected database's `system_configs` table and can only be changed from the Settings page or runtime configuration API (`GET /api/v1/system/config`, `PATCH /api/v1/system/config/runtime/:type`). Container restarts do not overwrite database values with environment variables; stored values remain effective after restart. Settings can reset one type or all types to code defaults. The administrator key is also stored in the `auth` row, but is managed only by setup and `admin reset-key --key`; it is never shown or reset as a default.

## Plugins

Plugin archives can be uploaded from the management page, imported with `meerkit plugin import`, or copied to `${data_dir}/plugins/inbox`. Only `.zip` and `.tar.gz` packages are accepted; raw executables are never discovered or executed.

### Generate the official signing key

Official plugins are signed with an Ed25519 key. Run the following command from the repository root; its argument is the output path prefix:

```bash
make generate-key KEY_PREFIX=./keys/meerkit-official
```

This target uses the existing `scripts/package-plugins.sh --generate-key` flow.

The command creates two Base64-encoded files and refuses to overwrite existing keys:

- `keys/meerkit-official.private.key`: the signing private key, which must remain in the release environment or CI secret storage.
- `keys/meerkit-official.public.key`: the public key, suitable for backup, fingerprint publication, or optional preconfigured trust.

Never commit the private key, copy it into `dist/`, or include it in a release archive. The package signature file, `meerkit-plugin.sig`, automatically embeds the public key derived from the private key, so the public key file is not passed to the packaging command. The key ID is a stable, human-readable release identifier such as `meerkit-official-2026`; the SHA-256 public-key fingerprint is the actual trust identity.

### Package official plugins

Set the private-key path and key ID, then run the batch target. It packages every publishable manifest under `plugins/`, currently HTTP and TCP, while automatically excluding the `plugins/template` source scaffold:

```bash
make package-plugins \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026 \
  TARGETS=linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

The script creates one plugin package per platform: `.zip` for Windows and `.tar.gz` for other platforms. The signature covers the manifest and its artifact hashes, plus packaged README and LICENSE files. To package only one official plugin, pass the signing arguments directly:

```bash
make package-plugin \
  PLUGIN=./plugins/http \
  TARGETS=linux/amd64 \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026
```

### Build a complete official release

`scripts/package.sh` builds the frontend, host executable, and every official plugin for each target. It forwards the signing environment variables to the internal plugin packaging step, so all official plugins in a release use the same key:

```bash
make package-release \
  VERSION=v0.1.0 \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026 \
  TARGETS=linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

The default output directories are `dist/plugins` and `dist/releases`; override them with `PLUGIN_OUTPUT` and `RELEASE_OUTPUT`. `TARGETS` defaults to the current platform. Omitting both `SIGN_KEY` and `KEY_ID` creates unsigned packages; when signing, both variables are required. The Make targets still delegate the actual work to `scripts/package-plugins.sh` and `scripts/package.sh`, so the scripts remain available directly for finer-grained options.

Inside each generated release archive, official plugins are stored in the `plugins/` directory next to the host executable. On the first start with an empty data directory, the host scans that directory, verifies and enables the packages, and records their signing fingerprint as an official publisher. Later versions or plugins signed by the same key are verified automatically when imported manually. Official status is established by this first-run release bootstrap; the public key does not also need to be added to `plugins.trusted_keys`. A plugin manifest cannot declare itself official, so importing a separately generated signed package into an installation that has not bootstrapped official trust still requires the user to verify and confirm its public-key fingerprint like any third-party signed package.

### Run plugin sources in development

The host version defaults to `dev` when no version is injected through `-ldflags "-X main.version=..."`. Running `go run .` from the repository root therefore discovers `plugins/*/meerkit-plugin.yaml` (excluding `plugins/template`) and rebuilds each source plugin on every startup. Changes to the built-in HTTP and TCP plugins only require another `go run .`; no package, import, upgrade, or manifest version bump is needed.

```yaml
plugins:
  source_dir: ./plugins
  trusted_keys: {}
```

Plugin loading is now derived from the host version and no longer requires `plugins.mode`: a `dev` host prefers source plugins and falls back to packages when none are present, while a versioned release host loads packages only. `MEERKIT_PLUGINS__SOURCE_DIR` can override the source directory. Development staging files and binaries are written only under `${storage.data_dir}/plugins` (the repository-root `data/plugins` directory by default), never under a plugin source directory. Source plugins are marked as development sources in the management page and have no exportable signed archive. A release host removes development installation records before restoring packaged plugins next to the executable.

Host and plugin log levels, formats, and output switches are edited in runtime settings. Formats are `text`, `simple`, and `json`, with `simple` as the default. Plugin logging produces compact lines such as `[09:08:07] [INFO] plugin activated plugin_id=meerkit.http`; changes apply to subsequently started plugins and may restart an active plugin process.

Use the equivalent PowerShell scripts on Windows:

```powershell
.\scripts\package-plugins.ps1 -GenerateKey .\keys\meerkit-official
$env:MEERKIT_PLUGIN_SIGN_KEY = ".\keys\meerkit-official.private.key"
$env:MEERKIT_PLUGIN_KEY_ID = "meerkit-official-2026"
.\scripts\package.ps1 -Output dist/releases -Targets "windows/amd64" -Version "v0.1.0"
```

Back up and retain the private key for the lifetime of the release line. Replacing it creates a new public-key fingerprint that existing installations do not recognize as the same publisher, so key rotation requires a planned release and a new trust decision. See [`plugins/README.en.md`](plugins/README.en.md) for plugin development, third-party signing, and the trust model, and [`scripts/README.en.md`](scripts/README.en.md) for script details.

## Layout

```text
Makefile          development, packaging, and key-generation shortcuts
main.go
internal/         host application packages
internal/runtimeconfig/ runtime configuration defaults, validation, and hot application
plugins/          plugin protocol definitions, official plugins, and examples
cmd/pluginpack/   plugin packaging and signing tool
sdk/              public monitor plugin SDK and gRPC protocol
scripts/          project and plugin release scripts
web/              React/Vite frontend and shared UI components
```
