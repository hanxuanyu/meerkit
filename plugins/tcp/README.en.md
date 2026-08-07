# Meerkit TCP monitor plugin

Provides the `tcp` module through the public Meerkit plugin SDK. It checks TCP port connectivity and can optionally send probe data, read one server response, and monitor response changes.

## Capabilities

- Connects to a host and port using IPv4, IPv6, or DNS names.
- Configures connection and response-read timeouts independently.
- Sends plain text or decodes Base64 before sending binary data.
- Reads one server response and exposes text, Base64, byte count, and SHA-256 representations.
- Supports conditions on connectivity, connection latency, response text, response size, and response hash.

The module type is `tcp`; module, configuration, and result schema versions are all `1`. Monitor lists use `host:port` as the content summary.

## Configuration

### Connection

- `host`: required hostname or IP address. IPv6 values do not need manual brackets.
- `port`: required target port in the range `1-65535`.
- `timeout_seconds`: connection timeout; defaults to `10`, range `1-300` seconds.

### Sending data

- `send`: optional. An empty value only opens the connection; otherwise the value is written once after connecting.
- `send_base64`: disabled by default. When enabled, `send` is decoded as standard Base64 and the resulting bytes are written. Invalid Base64 fails the execution.

### Reading a response

- `read_response`: disabled by default. When enabled, the plugin performs one read after sending.
- `read_timeout_seconds`: read timeout; defaults to `3`, range `1-60` seconds.
- `max_read_bytes`: maximum number of bytes read once; defaults to `65536`, range `1-1048576`.

The implementation performs a single `Read`. It is intended for port checks, banners, simple request-response protocols, and fixed probes, not continuous streams, protocol framing, or multi-round conversations.

## Results

The `connection` result set contains:

- `success`: whether the complete probe succeeded.
- `connected`: whether a TCP connection was established.
- `duration_ms`: connection establishment time in milliseconds.
- `remote_addr`: actual remote endpoint used by the connection.
- `response_text`: bytes read represented as text.
- `response_bytes`: standard Base64 representation, suitable for non-text protocols.
- `response_hash`: SHA-256 of the response bytes.
- `bytes_read`: number of bytes read.

When `read_response` is disabled, text and Base64 results are empty, `bytes_read` is `0`, and the hash represents empty content. Meerkit also adds its common execution-summary result set.

## Condition examples

- Port availability: `Current result / TCP connection / Connected is true`.
- Slow connection: `Current result / TCP connection / Duration greater than 500`.
- Banner check: enable response reading and require `Response text contains ready`.
- Binary response change: compare the previous and current `Response hash`.
- Size anomaly: verify that `Bytes read` remains within an expected range.

## Logging and privacy

The plugin logs connection start and completion, bytes written and read, response hashes, and failure causes. It never logs sent or received payload content, protecting protocol credentials and application data.

Configure plugin logging through the host:

Logs are stored under `${storage.data_dir}/plugins/logs` and can be followed from the plugin management page.

## Development and testing

Running `go run .` from the repository root makes a `dev` host build and load `plugins/tcp` directly. Artifacts are written only under the root `data/plugins` directory.

Run plugin tests independently:

```sh
cd plugins/tcp
go test ./...
```

Build an importable package for the current platform:

```sh
scripts/package-plugins.sh --plugin ./plugins/tcp
```

See `plugins/README.en.md` for signing, cross-platform targets, and the trust model.
