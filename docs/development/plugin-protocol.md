# 跨语言插件协议

第三方插件可以使用 Python、Java、Rust 或其他语言实现，但必须遵循同一个本地进程与 gRPC 协议。规范事实来源是仓库中的 [`sdk/PROTOCOL.md`](https://github.com/hanxuanyu/meerkit/blob/main/sdk/PROTOCOL.md)、[`monitor.proto`](https://github.com/hanxuanyu/meerkit/blob/main/sdk/proto/monitor.proto) 和 [`sdk/schema`](https://github.com/hanxuanyu/meerkit/tree/main/sdk/schema)。

## 进程启动

宿主根据清单选择当前平台制品，并设置：

```text
MEERKIT_MONITOR_PLUGIN=meerkit-monitor-v1
MEERKIT_PLUGIN_ID=<plugin id>
MEERKIT_PLUGIN_NAME=<plugin name>
MEERKIT_PLUGIN_VERSION=<plugin version>
MEERKIT_PLUGIN_LOG_LEVEL=<debug|info|warn|error>
MEERKIT_PLUGIN_LOG_FORMAT=<text|simple|json>
```

插件校验 Magic Cookie，创建只允许当前用户连接的本地 gRPC listener，然后把握手写到 stdout：

```text
1|1|unix|/path/to/socket|grpc
```

字段依次是 go-plugin 核心版本、Meerkit 应用协议版本、网络类型、地址和 RPC 协议。Unix 使用 Unix Domain Socket，Windows 使用 `127.0.0.1` 回环 TCP。业务日志写 stderr。

::: danger 不要暴露监听器
插件 RPC 是宿主与子进程之间的本地 IPC，不是带认证的远程 API。不要监听公网或局域网地址。
:::

## gRPC 服务

`meerkit.sdk.Monitor` 的五个一元 RPC 都使用 `google.protobuf.BytesValue`：

| 方法 | 请求 JSON | 成功响应 JSON |
| --- | --- | --- |
| `ListModules` | 空 BytesValue | `{"modules":[...]}` |
| `ValidateConfig` | `module_type`, `config` | `{}` |
| `Execute` | `module_type`, `config` | `{"observation":{...}}` |
| `MigrateConfig` | `module_type`, `from_version`, `to_version`, `config` | `{"config":...}` |
| `Health` | `{}` | `{}` |

所有非空 Value 都是 UTF-8 JSON。插件还必须注册标准 gRPC Health Checking 服务，并让服务名 `plugin` 返回 `SERVING`。

调用顺序通常是：

```text
process start
  -> handshake
  -> grpc.health.v1.Health/Check("plugin")
  -> Monitor/Health
  -> BrowserBridge/Session (long-lived stream, plugin sends ready)
  -> Monitor/ListModules
  -> [Monitor/MigrateConfig]
  -> Monitor/ValidateConfig / Monitor/Execute ...
```

## 浏览器能力流

插件在同一个 go-plugin gRPC Server 上注册 `meerkit.sdk.BrowserBridge`。宿主不会另外监听 Capability 端口；Monitor 一元 RPC 和 BrowserBridge 双向流复用同一条 HTTP/2 连接。Go 插件使用统一运行时：

```go
runtime := sdk.NewPluginRuntime()
browser := runtime.Browser()
runtime.Serve(sdk.NewProvider(...))
```

`BrowserClient` 提供目标查询、单个原子 Action 和独立网络捕获会话。浏览器操作通过带请求 ID 的 JSON 信封关联响应、取消和持续事件。Session 不可用时调用立即返回能力不可用错误；事件队列有界，并以插件及捕获 ID 隔离。具体信封和 operation 见 SDK 协议文档，宿主、扩展和网络捕获的完整生命周期见[浏览器自动化架构](/development/browser-automation)。

## 错误语义

业务错误通过响应 JSON 的非空 `error` 返回，同时 gRPC status 保持 OK。例如配置校验失败：

```json
{
  "error": "timeout_seconds must be between 1 and 30"
}
```

只有无法解析 BytesValue、JSON 不是有效 UTF-8/结构或无法编码响应等线协议错误才返回非 OK gRPC status。实现必须传播调用的取消和 deadline。

## 描述器与观测

`ListModules` 返回的模块数量、类型和模块版本必须与清单完全一致。描述器还必须提供：

- UI 参数和条件关系。
- 结果集、字段类型、单位、格式、路径能力和操作符。
- 配置与结果 JSON Schema。

`Execute` 的 `observation.schema_version` 应与清单的 `result_schema_version` 一致。JSON Schema 对字段和附加属性有明确约束，第三方实现应在自己的单元测试中直接验证输出。

## 制品启动能力

直接执行：

```yaml
runtime:
  mode: direct
  args: ["serve"]
```

清单级 `runtime` 是所有制品的默认启动配置；`artifacts[].runtime` 可以针对单个平台覆盖。`direct` 固定执行当前平台制品，不能声明 `command`；只有解释器模式需要明确启动命令。

解释器执行：

```yaml
runtime:
  mode: interpreter
  command: python3
  args: ["-I", "{artifact}"]
```

解释器 `command` 只能是从 `PATH` 查找的命令名，不能包含路径分隔符。`args` 最多 32 项，必须恰好有一个独立 `{artifact}`。宿主直接构造 argv，不运行 shell。

JAR 可以使用 `command: java` 和 `args: ["-jar", "{artifact}"]`；Python 可以把 zipapp 作为单文件制品。无论语言如何，包仍要为每个目标 `goos/goarch` 声明唯一制品、大小和 SHA-256。

## 版本演进

当前协议版本为 `1`，清单以范围声明兼容性：

```yaml
protocol:
  min: 1
  max: 1
```

可以在 v1 内新增可选 JSON 字段。删除字段、改变字段既有语义、修改服务名或 RPC 签名都需要新的协议版本，并让宿主与插件通过范围拒绝不兼容组合。

下一步使用[一致性测试](/development/plugin-testing)检查真实制品。
