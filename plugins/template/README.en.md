# Meerkit Go plugin template

[中文](README.md) · [Go plugin guide](../../docs/development/plugin-go.md) · [Protocol specification](../../sdk/PROTOCOL.en.md)

This directory is a minimal compilable monitor plugin template. It provides an `example` module, a source manifest, and a black-box conformance suite. The development host and bulk release scripts both skip this directory.

## Create a plugin

1. Copy the directory, for example `cp -R plugins/template plugins/dns`.
2. Change the module path in `go.mod`.
3. Update the plugin ID, name, publisher, versions, and module declarations in `meerkit-plugin.yaml`.
4. Declare parameters, result sets, field types, and supported condition operators in `Descriptor`.
5. Implement `ValidateConfig`, `Execute`, and any required `MigrateConfig` behavior.
6. Add unit tests for validation, successful and failed executions, timeouts, and cancellation.
7. Update `conformance.json`, build an artifact, and run the black-box checker.

Use a separate implementation package for a nontrivial plugin:

```text
my-plugin/
├── go.mod
├── go.sum
├── main.go
├── conformance.json
├── meerkit-plugin.yaml
├── README.md
└── monitor/
    ├── module.go
    └── module_test.go
```

Keep `main.go` limited to serving the Provider:

```go
func main() {
    sdk.Serve(sdk.NewProvider(monitor.New()))
}
```

A Provider may include multiple modules, but every runtime descriptor type and version must agree with the manifest.

## Descriptor guidelines

- `Parameters` drives the management UI form and should declare types, defaults, constraints, options, and conditional visibility.
- `ResultSets` drives result presentation, the condition editor, and the status board. Declare only operators a field truly supports.
- Structured JSON can enable `Path`; large text or binary results should also expose hashes, lengths, or summaries.
- `Parameters` are the only field source for dynamic forms and must match `ConfigSchema.properties`, including required declarations; `ResultSets` likewise define result UI capabilities.
- `Execute` must honor `context.Context` deadlines and cancellation. Never log secrets, tokens, complete headers, or sensitive bodies.

## Test and run

```bash
cd plugins/template
go test ./...
go build -o /tmp/meerkit-example-plugin .

cd ../..
go run ./cmd/plugincheck \
  --manifest ./plugins/template/meerkit-plugin.yaml \
  --artifact /tmp/meerkit-example-plugin \
  --suite ./plugins/template/conformance.json
```

Place the copied plugin below `plugins.source_dir` and run `go run . serve` from the repository root. The development host builds and loads it automatically.

## License

The template is licensed with Meerkit under the [Apache License 2.0](../../LICENSE). When creating an independent third-party plugin, select and package its own license explicitly.
