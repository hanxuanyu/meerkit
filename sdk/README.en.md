# Meerkit Go plugin SDK

[中文](README.md) · [Protocol specification](PROTOCOL.en.md) · [Go plugin guide](../docs/development/plugin-go.md)

`github.com/hanxuanyu/meerkit/sdk` is the official Go implementation of the Meerkit monitor plugin protocol. It provides module descriptors, parameter and result contracts, the `Provider` aggregator, gRPC adapters, process handshaking, and structured logging initialization.

The Go SDK is a protocol implementation, not an API that other languages must reproduce. A non-Go plugin should implement [`proto/monitor.proto`](proto/monitor.proto) directly and follow the JSON Schemas under [`schema`](schema/).

## Minimal plugin

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/hanxuanyu/meerkit/sdk"
)

type module struct{}

func (module) Descriptor() sdk.ModuleDescriptor {
    return sdk.ModuleDescriptor{
        Type: "example", Version: "1", ConfigVersion: "1",
        ResultSchemaVersion: "1", Name: "Example",
        ConfigSchema: map[string]any{"type": "object"},
        ResultSchema: map[string]any{"type": "object"},
    }
}

func (module) ValidateConfig(json.RawMessage) error { return nil }
func (module) MigrateConfig(_ context.Context, _, _ string, raw json.RawMessage) (json.RawMessage, error) {
    return raw, nil
}
func (module) Execute(context.Context, json.RawMessage) (sdk.Observation, error) {
    return sdk.Observation{Success: true, SchemaVersion: "1", Result: map[string]any{}}, nil
}

func main() { sdk.Serve(sdk.NewProvider(module{})) }
```

A plugin should depend only on public SDK packages and must not import Meerkit `internal` packages. Manifest module types and versions must agree with runtime descriptors and execution observations.

## Contract files

| Path | Purpose |
| --- | --- |
| [`PROTOCOL.en.md`](PROTOCOL.en.md) | Process, handshake, RPC, versioning, and runtime rules |
| [`proto/monitor.proto`](proto/monitor.proto) | Language-neutral gRPC service definition |
| [`schema/request.schema.json`](schema/request.schema.json) | RPC request JSON |
| [`schema/response.schema.json`](schema/response.schema.json) | RPC response and application-error envelope |
| [`schema/module-descriptor.schema.json`](schema/module-descriptor.schema.json) | Module capability descriptor |
| [`schema/observation.schema.json`](schema/observation.schema.json) | Execution observation |
| [`schema/conformance-suite.schema.json`](schema/conformance-suite.schema.json) | Black-box test suite |

## Testing

```bash
cd sdk
go test ./...
```

A complete artifact should also pass the root module's `cmd/plugincheck`. It intentionally bypasses the Go `Provider` API and checks the real process handshake, gRPC Health, raw JSON, manifest agreement, and optional test cases.

## Compatibility

The current application protocol version is `1`. Optional JSON fields can be added without changing that version. Removing fields, changing existing semantics, or changing the RPC service or method signatures requires a protocol version increment. A package declares its accepted range through `protocol.min` and `protocol.max`.

## License

The SDK is licensed with Meerkit under the [Apache License 2.0](../LICENSE).
