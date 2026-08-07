# Meerkit HTTP monitor plugin

Provides the `http` module through the public Meerkit plugin SDK. It requests HTTP/HTTPS services, captures structured responses, and supports conditions based on status, latency, body content, or JSON fields.

## Capabilities

- Supports `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, `TRACE`, and `CONNECT`.
- Supports query parameters, custom headers, HTTP/HTTPS proxies, and request timeouts.
- Supports URL-encoded forms, multipart forms, raw JSON, and raw text bodies.
- Supports redirect limits, TLS verification, and response-size limits.
- Detects JSON responses automatically or handles a response explicitly as text or JSON.
- Produces a SHA-256 body hash for efficient change detection.
- Supports nested JSON paths such as `data.status` in conditions.

The module type is `http`, module version is `2`, and both the configuration and result schema versions are `1`.

## Configuration

### Request target

- `url`: required full `http://` or `https://` URL. The monitor list uses this value as its content summary.
- `method`: request method; defaults to `GET`.
- `timeout_seconds`: timeout for the complete request; defaults to `30`, range `1-300` seconds.
- `proxy_url`: optional HTTP/HTTPS proxy, for example `http://127.0.0.1:7890`.

### Query and headers

- `query`: query parameters represented as key-value entries. A key may have multiple values.
- `headers`: headers sent unchanged, including fields such as `Accept`, `Authorization`, and `Cookie`.

Parameters already present in the URL are retained and merged with `query`.

### Request body

Body controls are available for `POST`, `PUT`, `PATCH`, `DELETE`, and `OPTIONS`.

- `body_mode`: `none`, `form_urlencoded`, `multipart_form`, `raw_json`, or `raw_text`; defaults to `none`.
- `form_fields`: fields sent by either form mode.
- `json_body`: JSON value sent in raw JSON mode; syntax is validated before saving.
- `raw_body`: unmodified content sent in raw text mode.

The plugin supplies the appropriate `Content-Type` for form and raw modes unless an explicit header is configured.

### Response processing

- `response_mode`: defaults to `auto`. `auto` uses the response `Content-Type`; `text` retains text only; `json` attempts JSON parsing.
- `normalize`: controls text and hash normalization. `raw` preserves input, `trim` removes surrounding whitespace, and `json` canonicalizes valid JSON.
- `max_body_bytes`: maximum captured response size; defaults to `262144`, range `1024-1048576`. Larger responses set `truncated=true`.
- `follow_redirects`: follows 3xx responses by default.
- `max_redirects`: defaults to `10`, range `1-50`.
- `verify_tls`: verifies HTTPS server certificates by default. Disable it only when the reduced connection security is understood.

## Results

The `response` result set contains:

- `success`: whether the network request and response read completed. It does not mean the status was 2xx.
- `status_code`: HTTP response status.
- `duration_ms`: request duration in milliseconds.
- `response_headers`: flattened response headers.
- `body_text`: response text after normalization.
- `body_json`: parsed JSON when parsing succeeds; otherwise absent.
- `body_hash`: SHA-256 of the normalized body.
- `body_size`: number of response bytes retained.
- `truncated`: whether the body exceeded `max_body_bytes`.

Meerkit adds its common execution-summary result set to every plugin result. The plugin detail dialog displays declared inputs and result fields, while an execution detail displays captured values.

## Condition examples

- Availability: `Current result / HTTP response / Status code equals 200`.
- Latency: `Current result / HTTP response / Duration greater than 1000`.
- Business status: select `Response JSON`, enter `data.status`, and compare it with a fixed value.
- Content change: compare the previous and current `Body hash`.
- Body matching: apply contains, not-contains, or regular-expression operators to `Response text`.

## Logging and privacy

The plugin logs request start, request failures, response-read failures, and processed-response summaries. Entries include the method, target without query parameters, status, duration, body size, truncation state, and body hash. Headers, bodies, query values, and response content are never logged.

Configure plugin logging through the host:

Logs are stored under `${storage.data_dir}/plugins/logs` and can be followed from the plugin management page.

## Development and testing

Run the host from the repository root:

```sh
go run .
```

A `dev` host builds and loads `plugins/http` directly. Artifacts are written only under the root `data/plugins` directory, so source changes can be tested after restarting without packaging and re-importing.

Run plugin tests independently:

```sh
cd plugins/http
go test ./...
```

Build an importable package for the current platform:

```sh
scripts/package-plugins.sh --plugin ./plugins/http
```

See `plugins/README.en.md` for signing, cross-platform targets, and the trust model.
