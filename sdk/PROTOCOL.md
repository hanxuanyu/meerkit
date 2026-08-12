# Meerkit 监控插件协议 v1

本文档是第三方监控插件实现者使用的线协议规范。Go SDK 是该协议的一种实现，不是协议本身。

## 进程与握手

宿主将插件作为独立子进程启动。插件必须校验环境变量 `MEERKIT_MONITOR_PLUGIN=meerkit-monitor-v1`，启动本地 gRPC Server，并向 stdout 输出一行 HashiCorp go-plugin 握手信息：

```text
1|1|unix|/path/to/socket|grpc
```

五段依次是 go-plugin 核心协议版本、Meerkit 协议版本、网络类型、监听地址和 RPC 协议。Unix 系统应使用仅当前用户可访问的 Unix Domain Socket；Windows 使用 `127.0.0.1` TCP。stdout 的第一行保留给握手，业务日志应写 stderr。

插件还必须注册标准 gRPC Health Checking 服务，并让服务名 `plugin` 返回 `SERVING`。宿主不会把插件端口作为远程网络接口使用，也不会把插件放入安全沙箱。

## gRPC 服务

规范源文件为 [`proto/monitor.proto`](proto/monitor.proto)。`meerkit.sdk.Monitor` 包含以下一元 RPC：

| 方法 | 请求 JSON | 成功响应 JSON |
| --- | --- | --- |
| `ListModules` | `BytesValue.value` 为空 | `modules` |
| `ValidateConfig` | `module_type`, `config` | `{}` |
| `Execute` | `module_type`, `config` | `observation` |
| `MigrateConfig` | `module_type`, `from_version`, `to_version`, `config` | `config`，字段必须存在，值可为任意 JSON |
| `Health` | `{}` | `{}` |

所有非空 `BytesValue.value` 都必须是 UTF-8 JSON。请求、响应、模块描述器和观测结果分别由 [`schema/request.schema.json`](schema/request.schema.json)、[`schema/response.schema.json`](schema/response.schema.json)、[`schema/module-descriptor.schema.json`](schema/module-descriptor.schema.json) 和 [`schema/observation.schema.json`](schema/observation.schema.json) 定义。

业务错误使用 HTTP 风格之外的统一信封返回：gRPC 调用本身成功，响应 JSON 包含非空 `error`。无法解析请求、无法编码响应等线协议错误才使用非 OK gRPC status。插件必须传播 gRPC context 的取消和截止时间。

`ListModules` 返回的模块集合必须与 `meerkit-plugin.yaml` 的 `modules` 在类型、数量和模块版本上完全一致。`Execute` 返回的 `observation.schema_version` 应与对应模块清单中的 `result_schema_version` 一致。

### BrowserBridge 双向流

插件必须在同一个 go-plugin gRPC Server 注册 `meerkit.sdk.BrowserBridge`。宿主完成健康检查后，在同一条 gRPC/HTTP2 连接上建立长期 `Session(stream BytesValue) returns (stream BytesValue)`；不会创建第二个 Capability 端口，也不会注入 Capability endpoint 或 token。

每个 `BytesValue.value` 是下列 JSON 信封：

```json
{
  "type": "ready|request|response|event|cancel",
  "id": "message-id",
  "reply_to": "request-id",
  "operation": "browser.action",
  "payload": {},
  "error": ""
}
```

插件建立 Session 后先发送 `ready`。插件以 `request` 调用 `browser.targets`、`browser.action`、`browser.network.start` 和 `browser.network.stop`；宿主以 `response.reply_to` 关联结果。调用 Context 取消时插件发送 `cancel.reply_to`。宿主可发送 `browser.network`、`browser.network.status` 和 `browser.targets.changed` 事件。

页面类 Action 必须显式指定 `tab_id`，`tab.open` 只接受可选 `window_id`。网络捕获是独立、固定绑定标签页的会话，不属于 Action 工作流。实现必须保证单个 Session 只有一个 gRPC 发送协程，使用有界队列，并按插件和捕获会话隔离事件；慢捕获消费者只能终止自己的捕获，不能阻塞 Monitor RPC 或其他插件。

## 版本

- go-plugin 核心协议版本固定为 `1`。
- 当前 Meerkit 应用协议版本为 `1`。
- 清单的 `protocol.min` 和 `protocol.max` 声明插件接受的 Meerkit 协议范围。
- 当前开发阶段直接以新的 Monitor + BrowserBridge 契约替换协议 `1`，不提供旧插件兼容分支。契约稳定发布后，新增可选 JSON 字段可以保持协议版本不变；删除字段、修改字段语义、服务名或方法签名必须提升协议版本。

## 制品运行方式

清单必须声明顶层 `runtime`，作为源码构建和全部制品的默认启动配置。每个 `artifacts` 条目可以声明自己的 `runtime` 进行平台级覆盖。

直接执行配置为：

```yaml
runtime:
  mode: direct
  args: []
```

`direct` 固定以 artifact 本身作为可执行文件，不能声明 `command`。`args` 必填，可以声明最多 32 个固定参数，例如 `args: ["serve"]`。

第三方单文件制品可以交给宿主环境中的解释器启动：

```yaml
runtime:
  mode: interpreter
  command: python3
  args: ["-I", "{artifact}"]
```

解释器模式的 `command` 必须是不含路径分隔符的命令名，由宿主从 `PATH` 查找；`args` 必须包含且只包含一个独立的 `{artifact}` 参数。宿主直接构造参数数组，不执行 shell、不展开环境变量或通配符。解释器及其版本属于部署前置条件；插件包仍需为每个目标 OS/架构提供一个经过 SHA-256 校验的单文件制品。

## 一致性检查

仓库根目录提供 `plugincheck` 黑盒工具。它按宿主相同的方式启动制品，检查握手、标准 gRPC Health、应用 Health、响应 Schema、模块清单一致性，并可通过测试套件执行配置校验、监控和迁移：

```bash
go run ./cmd/plugincheck \
  --manifest ./meerkit-plugin.yaml \
  --artifact ./build/plugin \
  --suite ./conformance.json
```

测试套件格式见 [`schema/conformance-suite.schema.json`](schema/conformance-suite.schema.json)。
