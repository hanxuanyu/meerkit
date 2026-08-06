# Meerkit monitor plugin template

This is a minimal runnable skeleton for building an independent Go module and exposing monitor capabilities through the Meerkit SDK. The development host does not load `template`, and the bulk packaging script does not publish it.

## Creating a plugin

1. Copy this directory, for example to `plugins/dns`.
2. Change the module path in `go.mod` and replace the implementation in `main.go`.
3. Update plugin identity, protocol, and module versions in `meerkit-plugin.yaml`.
4. Declare configuration parameters, result sets, and supported operators in `Descriptor`.
5. Implement validation, execution, and any required configuration migration.
6. Add contract tests, then verify the UI and execution through the development host.

## Suggested layout

```text
my-plugin/
├── go.mod
├── go.sum
├── main.go
├── meerkit-plugin.yaml
├── README.md
├── README.en.md
└── monitor/
    ├── module.go
    └── module_test.go
```

For nontrivial plugins, keep the module implementation in a separate `monitor` package and make `main.go` responsible only for serving it:

```go
func main() {
    sdk.Serve(sdk.NewProvider(monitor.New()))
}
```

A provider may receive multiple modules, but every runtime `Descriptor` must exactly match a module declared in the manifest.

## Manifest

The source manifest must conform to `plugins/manifest.schema.json`. Important fields are:

- `id`: stable globally unique plugin ID, preferably namespaced by an organization or domain.
- `version`: semantic package version. Different release contents cannot reuse an ID and version.
- `vendor`: publisher name.
- `desp`: capability summary shown by plugin management.
- `url`: source repository or trusted release page.
- `protocol.min/max`: supported Meerkit plugin protocol range.
- `modules`: module types and versions provided by the plugin.
- `artifacts`: empty in source; the packager writes platform, path, size, and SHA-256 values.

Package `version`, module `version`, `config_version`, and `result_schema_version` have different roles:

- Package `version` identifies the complete release.
- Module `version` identifies the compatible module implementation.
- Changing `config_version` asks the host to migrate stored monitor configuration.
- `result_schema_version` identifies the persisted execution-result structure.

## Module descriptor

`Descriptor` is the primary capability contract and drives both the monitor editor and plugin detail dialog.

- `Type` and `Version` must match the manifest.
- `Name` and `Description` are presented in the UI.
- `ListSummary` selects configuration values shown below a monitor title, such as a URL or `host:port`.
- `Parameters` declares input types, order, defaults, ranges, options, and conditional visibility.
- `ResultSets` groups output fields and declares type, format, unit, path support, and operators.
- `ConfigSchema` and `ResultSchema` provide machine-readable structural constraints.

Do not provide JSON Schema alone. Meerkit's dynamic forms and compact capability display primarily use `Parameters` and `ResultSets`.

## Input parameters

Supported parameter types include string, multiline text, list, map, boolean, integer, number, URL, JSON, and duration. Common constraints include:

- `Required`: makes the value mandatory.
- `Default`: supplies a default value.
- `Minimum`, `Maximum`, and `Step`: constrain numeric input.
- `Options`: declares a fixed option set.
- `OptionsWhen`: changes options based on other values.
- `VisibleWhen` and `EnabledWhen`: control field visibility and availability.
- `Secret`: marks sensitive input; plugin logs must still avoid emitting its value.
- `FullWidth`, `Rows`, `Placeholder`, `Format`, and `Unit`: refine editing behavior.

## Results

`Execute` returns an `sdk.Observation`:

- `Success` states whether module execution succeeded.
- `SchemaVersion` should match the manifest `result_schema_version`.
- `ResultSets` stores structured results grouped by descriptor.
- `Result` may retain a compatible flat result.
- `Summary` is a short human-readable account and should not duplicate large bodies or sensitive data.
- `ErrorCode` and `ErrorMessage` may expose stable machine errors and user-facing detail.

Declare only operators the field truly supports. Enable `Path` only for structured values such as JSON. Large text, binary content, and objects should also expose hashes, lengths, or summaries so conditions and notifications remain compact.

## Validation, migration, and execution

- `ValidateConfig` must reject invalid input before network or file operations and return understandable errors.
- `Execute` must honor its `context.Context`, use bounded external operations, and return a useful Observation on failure.
- Raise `config_version` and implement `MigrateConfig` when the configuration structure changes. Return an error when migration cannot be performed safely.
- Do not rely on long-lived mutable process state. A plugin may restart after enablement, upgrade, failed health checks, or host shutdown.

## Logging

The SDK logs process startup, module discovery, health checks, validation, execution, and migration lifecycle events. Modules may add business diagnostics through standard `log/slog`:

```go
slog.InfoContext(ctx, "dns query completed",
    "server", server,
    "record_type", recordType,
    "duration_ms", duration,
)
```

Log only metadata required for diagnosis. Never emit passwords, tokens, headers, complete request or response bodies, or sensitive configuration. The host controls plugin logs through `plugins.log_format` and `plugins.log_level`.

## Local development

From the repository root, run:

```sh
go run .
```

A `dev` host scans `plugins.source_dir`, skips directories named `template` and the `example.monitor` template ID, and builds other source plugins. Staging files, binaries, and logs stay under `${storage.data_dir}/plugins`.

Plugins are independent Go modules and should be tested from their own directory:

```sh
cd plugins/my-plugin
go test ./...
```

At minimum, cover descriptor declarations, valid and invalid configuration, successful execution, timeout and cancellation, external failures, serializable results, and prevention of sensitive log output.

## Packaging

Build an unsigned development package for the current platform:

```sh
scripts/package-plugins.sh --plugin ./plugins/my-plugin
```

Build a signed package:

```sh
scripts/package-plugins.sh \
  --plugin ./plugins/my-plugin \
  --sign-key ./keys/vendor.private.key \
  --key-id vendor-release-2026
```

Meerkit imports only `.zip` and `.tar.gz` packages; it never loads a manually copied raw executable. See `plugins/README.en.md` for key generation, cross-platform packaging, and the trust model.
