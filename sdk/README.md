# Meerkit 插件 SDK

该模块定义监控插件的公共描述器、执行结果、Provider 接口以及 HashiCorp go-plugin gRPC 通信层。插件通过 `sdk.Serve(sdk.NewProvider(...))` 启动，不应依赖 Meerkit 的 `internal` 包。

Go SDK 是监控插件协议的官方 Go 实现。跨语言实现应以 [`PROTOCOL.md`](PROTOCOL.md)、[`proto/monitor.proto`](proto/monitor.proto) 和 [`schema/`](schema/) 下的 JSON Schema 为准，不需要复刻 Go 接口。
