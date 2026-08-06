# Meerkit 插件 SDK

该模块定义监控插件的公共描述器、执行结果、Provider 接口以及 HashiCorp go-plugin gRPC 通信层。插件通过 `sdk.Serve(sdk.NewProvider(...))` 启动，不应依赖 Meerkit 的 `internal` 包。
