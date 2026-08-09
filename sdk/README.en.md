# Meerkit plugin SDK

This module defines public monitor descriptors, observations, the Provider interface, and the HashiCorp go-plugin gRPC transport. Plugins start with `sdk.Serve(sdk.NewProvider(...))` and must not depend on Meerkit `internal` packages.

The Go SDK is the official Go implementation of the monitor plugin protocol. Language-independent implementations should use [`PROTOCOL.md`](PROTOCOL.md), [`proto/monitor.proto`](proto/monitor.proto), and the JSON Schemas under [`schema/`](schema/) as their contract instead of reproducing Go interfaces.
