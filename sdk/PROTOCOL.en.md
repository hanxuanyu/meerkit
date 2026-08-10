# Meerkit monitor plugin protocol v1

This is the wire contract for third-party monitor plugins. The Go SDK is one implementation of this protocol, not the protocol itself.

## Process and handshake

The host launches the plugin as a child process. The plugin must verify `MEERKIT_MONITOR_PLUGIN=meerkit-monitor-v1`, start a local gRPC server, and write one HashiCorp go-plugin handshake line to stdout:

```text
1|1|unix|/path/to/socket|grpc
```

The fields are the go-plugin core version, Meerkit application protocol version, network type, listener address, and RPC protocol. Use a user-private Unix Domain Socket on Unix and `127.0.0.1` TCP on Windows. The first stdout line is reserved for the handshake; write application logs to stderr.

Register the standard gRPC Health Checking service and report service `plugin` as `SERVING`. The plugin endpoint is local IPC, not a remote API, and the plugin process is not sandboxed.

## Service and JSON payloads

[`proto/monitor.proto`](proto/monitor.proto) is the canonical service definition. `meerkit.sdk.Monitor` has five unary methods:

| Method | Request JSON | Successful response JSON |
| --- | --- | --- |
| `ListModules` | empty `BytesValue.value` | `modules` |
| `ValidateConfig` | `module_type`, `config` | `{}` |
| `Execute` | `module_type`, `config` | `observation` |
| `MigrateConfig` | `module_type`, `from_version`, `to_version`, `config` | a present `config` field containing any JSON value |
| `Health` | `{}` | `{}` |

Every non-empty `BytesValue.value` is UTF-8 JSON. The request, response, module descriptor, observation, and conformance suite contracts are published under [`schema/`](schema/).

Return application errors as a successful gRPC response containing a non-empty JSON `error` field. Reserve non-OK gRPC status for malformed wire requests and failures that prevent response encoding. Honor gRPC cancellation and deadlines.

`ListModules` must match the manifest module count, types, and module versions exactly. An execution's `observation.schema_version` should match the module's manifest `result_schema_version`.

## Artifact runtime

The manifest must declare a top-level `runtime` as the startup default for source builds and all artifacts. An `artifacts` entry may override it for a specific platform. Direct mode executes the artifact itself, does not allow `command`, and requires `args`; use an empty array when no arguments are needed:

```yaml
runtime:
  mode: direct
  args: []
```

A single-file third-party artifact may instead declare a host interpreter:

```yaml
runtime:
  mode: interpreter
  command: python3
  args: ["-I", "{artifact}"]
```

In interpreter mode, the host resolves `command` from `PATH` and `args` must contain exactly one standalone `{artifact}` argument. Meerkit executes an argv array directly and never invokes a shell or expands variables and globs. The interpreter and its compatible version are deployment prerequisites.

## Conformance tool

From the repository root:

```bash
go run ./cmd/plugincheck \
  --manifest ./meerkit-plugin.yaml \
  --artifact ./build/plugin \
  --suite ./conformance.json
```

The tool checks the go-plugin handshake, standard gRPC health, Meerkit application health, raw response JSON Schemas, and manifest agreement. A suite optionally exercises validation, execution, and migration. Its format is [`schema/conformance-suite.schema.json`](schema/conformance-suite.schema.json).
