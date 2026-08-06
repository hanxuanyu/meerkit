# Meerkit plugin SDK

This module defines public monitor descriptors, observations, the Provider interface, and the HashiCorp go-plugin gRPC transport. Plugins start with `sdk.Serve(sdk.NewProvider(...))` and must not depend on Meerkit `internal` packages.
