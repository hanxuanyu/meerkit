# Meerkit HTTP plugin

[中文](README.md) · [Plugin overview](../README.en.md)

The official `meerkit.http` plugin provides the `http` monitor module. It sends HTTP or HTTPS requests and returns status, timing, response headers, body content, and content hashes as structured results.

## Current capability

- Methods: `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, `TRACE`, and `CONNECT`.
- Query parameters and request headers.
- URL-encoded forms, multipart forms, raw JSON, and raw text bodies.
- Optional HTTP or HTTPS proxy.
- Configurable request timeout, maximum response size, redirect behavior, and redirect limit.
- TLS certificate verification by default, with an explicit per-monitor opt-out.
- `auto`, `text`, or `json` response parsing with raw, trimmed, or canonical JSON normalization.
- Stable hashes for text and JSON content and a truncation marker when the body limit is reached.

## Configuration

| Parameter | Default | Description |
| --- | --- | --- |
| `url` | required | `http://` or `https://` request URL |
| `method` | `GET` | Request method |
| `timeout_seconds` | `30` | 1 to 300 seconds |
| `proxy_url` | empty | Optional HTTP/HTTPS proxy |
| `response_mode` | `auto` | Automatic, text, or JSON parsing |
| `normalize` | `trim` | Raw, trim whitespace, or canonicalize JSON |
| `max_body_bytes` | `1048576` | 1 KiB to 1 MiB |
| `query` | empty | Query parameter map |
| `headers` | empty | Request header map |
| `body_mode` | `none` | None, URL-encoded form, multipart, raw JSON, or raw text |
| `follow_redirects` | `true` | Follow 3xx responses |
| `verify_tls` | `true` | Verify the server certificate |
| `max_redirects` | `10` | 1 to 50 when redirects are enabled |

Body fields appear conditionally based on the method and `body_mode`. Raw JSON must be valid before execution.

## Results

The result-set key is `response`:

| Field | Type | Description |
| --- | --- | --- |
| `success` | boolean | Request and response read completed successfully |
| `status_code` | string | HTTP status code, such as `200` |
| `duration_ms` | number | Request duration |
| `response_headers` | map | Response headers |
| `body_text` | text | Text response |
| `body_json` | json | Parsed JSON with path selection support |
| `body_hash` | string | Hash of normalized content |
| `body_size` | number | Number of response bytes read |
| `truncated` | boolean | The configured response limit was reached |

The module observes a response and does not automatically treat a non-2xx status as an execution error. Configure Meerkit conditions on `status_code`, body content, timing, or another field to trigger an alert.

## Development

```bash
cd plugins/http
go test ./...

cd ../..
go run . serve
```

The source manifest is [`meerkit-plugin.yaml`](meerkit-plugin.yaml), and the implementation is [`monitor/http.go`](monitor/http.go). See [`scripts/README.en.md`](../../scripts/README.en.md) for packaging.

## License

This plugin is licensed with Meerkit under the [Apache License 2.0](../../LICENSE).
