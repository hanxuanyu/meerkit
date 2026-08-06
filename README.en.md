# Meerkit

Meerkit is a monitoring service with independently managed monitor plugins and Webhook, SMTP, and in-app notifications. HTTP and TCP are distributed as official plugins rather than linked into the host process.

## Run

```bash
npm --prefix web run build
go run . serve --config config.yaml
```

The default UI is available at `http://127.0.0.1:8080`. On first access, set an administrator access key. All management APIs and notification streams then require an authenticated session. Local recovery is available with:

```bash
meerkit --data-dir ./data admin reset-key --key 'a-new-access-key'
```

Configuration precedence is built-in defaults, `config.yaml`, `MEERKIT_*` environment variables, then command-line flags. Nested environment variables use double underscores:

```bash
MEERKIT_SERVER__PORT=9090 MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit go run .
```

## Plugins

Plugin archives can be uploaded from the management page, imported with `meerkit plugin import`, or copied to `${data_dir}/plugins/inbox`. Only `.zip` and `.tar.gz` packages are accepted; raw executables are never discovered or executed.

Use `scripts/package-plugins.sh` to package every publishable plugin, or `scripts/package.sh` to create a platform release containing the host and all official plugins. Windows equivalents are in the same directory. See [`plugins/README.en.md`](plugins/README.en.md) for details.

## Layout

```text
main.go
internal/         host application packages
plugins/          official plugins, template, and packaging tools
sdk/              public monitor plugin SDK and gRPC protocol
scripts/          project and plugin release scripts
web/              React/Vite frontend and shared UI components
```
