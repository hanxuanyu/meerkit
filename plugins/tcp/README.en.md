# Meerkit TCP plugin

[中文](README.md) · [Plugin overview](../README.en.md)

The official `meerkit.tcp` plugin provides the `tcp` monitor module. It establishes a TCP connection and can optionally write one payload and read one server response.

## Current capability

- Connect to a hostname or IP and port.
- Configure a connection timeout from 1 to 300 seconds.
- Write UTF-8 text or Base64-decode the configured value before writing binary data.
- Probe the connection only or read one response.
- Configure a separate 1 to 60 second read timeout and a read limit up to 1 MiB.
- Return the remote address, connect time, response text, Base64 binary, content hash, and byte count.

The plugin does not implement TLS handshaking, persistent sessions, protocol-level retries, or multi-step exchanges. Use the HTTP plugin for HTTP/TLS response semantics or implement a dedicated monitor plugin for an application protocol.

## Configuration

| Parameter | Default | Description |
| --- | --- | --- |
| `host` | required | Hostname or IP address |
| `port` | required | 1 to 65535 |
| `timeout_seconds` | `10` | Connection timeout |
| `send` | empty | Value written after connecting; empty means connect only |
| `send_base64` | `false` | Decode `send` from Base64 before writing |
| `read_response` | `false` | Read one response after writing |
| `read_timeout_seconds` | `3` | Read timeout when response reading is enabled |
| `max_read_bytes` | `65536` | Read limit from 1 to 1048576 bytes |

## Results

The result-set key is `connection`:

| Field | Type | Description |
| --- | --- | --- |
| `success` | boolean | The complete execution succeeded |
| `connected` | boolean | A TCP connection was established |
| `duration_ms` | number | Connection time in milliseconds |
| `remote_addr` | string | Connected remote address |
| `response_text` | text | Text representation of the response |
| `response_bytes` | binary | Base64-encoded raw response |
| `response_hash` | string | Raw response content hash |
| `bytes_read` | number | Number of bytes read |

These fields can drive connection-failure, latency, content-change, and byte-count conditions.

## Development

```bash
cd plugins/tcp
go test ./...

cd ../..
go run . serve
```

The source manifest is [`meerkit-plugin.yaml`](meerkit-plugin.yaml), and the implementation is [`monitor/tcp.go`](monitor/tcp.go). See [`scripts/README.en.md`](../../scripts/README.en.md) for packaging.

## License

This plugin is licensed with Meerkit under the [Apache License 2.0](../../LICENSE).
