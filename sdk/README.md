# Meerkit Go 插件 SDK

[English](README.en.md) · [协议规范](PROTOCOL.md) · [Go 插件指南](../docs/development/plugin-go.md) · [浏览器能力插件](../docs/development/browser-plugin.md)

`github.com/hanxuanyu/meerkit/sdk` 是 Meerkit 监控插件协议的官方 Go 实现。它提供模块描述器、参数与结果契约、`Provider` 聚合器、gRPC 适配层、进程握手和结构化日志初始化。

Go SDK 是协议实现，不是跨语言插件必须复刻的 API。其他语言应直接实现 [`proto/monitor.proto`](proto/monitor.proto) 并遵循 [`schema`](schema/) 中的 JSON Schema。

## 最小插件

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

插件只应依赖 SDK 的公开包，不应导入 Meerkit 的 `internal` 包。清单中的模块类型、模块版本、配置版本和结果 Schema 版本必须与运行时描述器及执行结果一致。

## 浏览器能力

需要控制 Chrome 的插件使用统一运行时取得宿主提供的客户端：

```go
func main() {
    runtime := sdk.NewPluginRuntime()
    browser := runtime.Browser()
    runtime.Serve(sdk.NewProvider(monitor.New(browser)))
}
```

`BrowserClient` 支持目标查询、单 Action 和网络捕获。插件不直接连接 Chrome，不接触扩展配对令牌，并负责自己创建的标签页和捕获会话的完整生命周期。开发流程见[浏览器能力插件开发](../docs/development/browser-plugin.md)，56 个 Action 见[浏览器 Action 参考](../docs/reference/browser-actions.md)，跨语言流协议见 [`PROTOCOL.md`](PROTOCOL.md#browserbridge-双向流)。

## 契约文件

| 路径 | 用途 |
| --- | --- |
| [`PROTOCOL.md`](PROTOCOL.md) | 进程、握手、RPC、版本和启动方式规范 |
| [`proto/monitor.proto`](proto/monitor.proto) | 语言无关的 gRPC 服务定义 |
| [`schema/request.schema.json`](schema/request.schema.json) | RPC 请求 JSON |
| [`schema/response.schema.json`](schema/response.schema.json) | RPC 响应与业务错误信封 |
| [`schema/module-descriptor.schema.json`](schema/module-descriptor.schema.json) | 模块能力描述器 |
| [`schema/observation.schema.json`](schema/observation.schema.json) | 执行观测结果 |
| [`schema/conformance-suite.schema.json`](schema/conformance-suite.schema.json) | 黑盒测试套件 |

## 测试

```bash
cd sdk
go test ./...
```

完整插件制品还应使用根模块的 `cmd/plugincheck` 做黑盒检查。该工具故意不通过 Go `Provider` 接口调用插件，而是检查真实进程握手、gRPC Health、原始 JSON、清单一致性和可选测试用例。

## 兼容性

当前应用协议版本为 `1`。新增可选 JSON 字段可以保持协议版本；删除字段、改变既有语义、RPC 服务名或方法签名需要提升协议版本。包清单通过 `protocol.min` 与 `protocol.max` 声明兼容范围。

## 许可证

SDK 随 Meerkit 以 [Apache License 2.0](../LICENSE) 开源。
