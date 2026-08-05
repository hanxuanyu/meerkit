# Meerkit

Meerkit is a single-binary monitoring tool for observing HTTP and TCP response changes and notifying through Webhook or SMTP.

## Run

```bash
npm --prefix web run build
go run . --config config.yaml
```

The default UI is available at `http://127.0.0.1:8080`.

Configuration precedence is built-in defaults, `config.yaml`, `MEERKIT_*` environment variables, then command-line flags. Nested environment variables use double underscores:

```bash
MEERKIT_SERVER__PORT=9090 MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit go run .
```

Monitor configurations use standard five-field cron expressions, with optional seconds and common descriptors such as `@hourly`.

## Layout

```text
main.go
internal/
  api/           HTTP API and embedded SPA handler
  app/           external configuration loading
  core/          monitor, condition, observation, and notification contracts
  monitor/       module registry plus HTTP/TCP implementations
  notification/  Webhook/SMTP implementations
  runtime/       runner and cron scheduler
  store/         SQLite persistence
web/              React/Vite source and shadcn-style UI components
```
