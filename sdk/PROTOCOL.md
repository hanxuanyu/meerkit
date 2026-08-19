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

字段约束：

| 字段 | 约束 |
| --- | --- |
| `type` | 必填；取值为 `ready`、`request`、`response`、`event` 或 `cancel` |
| `id` | `request` 必填，在当前 Session 内唯一；建议使用单调序号或 UUID |
| `reply_to` | `response` 和 `cancel` 必填，等于被关联的 request `id` |
| `operation` | `request` 和 `event` 必填；`response` 应原样带回请求 operation |
| `payload` | operation 对应的 JSON 值；成功响应必须符合下表结构 |
| `error` | 失败响应的非空可读字符串；与成功 payload 互斥 |

#### 建连与时序

`BrowserBridge` 由插件提供服务，宿主是 gRPC 客户端。宿主健康检查成功后调用 `Session`，插件发送的第一条消息必须是 `{"type":"ready"}`。首帧不能是事件或请求；宿主只有收到 `ready` 后才认为插件启用完成。每个插件进程只允许一个活动 Session，断线后本 Session 的 request ID、挂起调用和捕获关联全部失效。

插件发起调用的时序是：

```text
plugin                         host
  |---- ready ----------------->|
  |---- request(id=browser-1) ->|
  |                             | execute operation
  |<--- response(reply_to=...)--|
```

响应可以乱序返回。插件必须使用 `reply_to` 关联请求，不能依赖发送顺序。未知、重复或已经取消的 `reply_to` 应忽略。宿主可能在 `browser.network.start` 响应到达前发送该会话的网络事件，客户端必须按 `session_id` 暂存，取得 start 响应后再交给对应消费者。

#### 请求 operation

| operation | request `payload` | 成功 response `payload` |
| --- | --- | --- |
| `browser.targets` | `{"agent_id":"可选"}` | `BrowserTargets`：`agent_id`、`windows[]` 及其 `tabs[]` |
| `browser.action` | `BrowserActionRequest` | `BrowserActionResult` |
| `browser.network.start` | `BrowserNetworkStartRequest` | `BrowserNetworkSession` |
| `browser.network.stop` | `{"id":"capture-session-id"}` | `BrowserNetworkStopResult` |

`BrowserActionRequest`：

```json
{
  "target": {"agent_id":"agent-id","window_id":4,"tab_id":21},
  "timeout_ms": 60000,
  "action": {
    "id": "step-id",
    "type": "dom.query",
    "params": {"selector":"main","max_length":65536}
  }
}
```

成功的 `BrowserActionResult` 包含 `id`、`type`、`success:true`、执行后的 `target`、`duration_ms` 和 action 特定的 `data`。`timeout_ms` 默认 60000，范围 1000 到 300000。页面、DOM、输入、Cookie、Storage 和 Runtime Action 要求正整数 `tab_id`；窗口操作按其 Catalog 要求 `window_id`；`tab.open` 只接受可选 `window_id`，不能携带已有 `tab_id`。同时提供窗口和标签页时必须属于同一窗口。

Action type、参数、默认值和返回 Data 的完整列表见主仓库 `docs/reference/browser-actions.md`。宿主负责补齐 Catalog 默认值、参数校验和 Agent capability 检查；插件仍应验证业务所需返回字段。

`BrowserNetworkStartRequest`：

```json
{
  "target": {"agent_id":"agent-id","window_id":4,"tab_id":21},
  "disable_cache": false,
  "rules": [
    {
      "id":"api",
      "url_contains":"/api/",
      "resource_type":"XHR",
      "max_body_bytes":262144
    }
  ]
}
```

目标标签页必填。规则数为 1 到 32，`max_body_bytes` 最大 1048576；会话固定绑定启动时的 Agent、窗口和标签页。成功返回 `BrowserNetworkSession`，包含 `id`、规范化后的 `target`、`status:"running"`、`started_at`、`count` 和可选 `error`。

停止操作只能停止当前插件拥有的捕获。成功返回：

```json
{
  "session": {
    "id":"capture-id",
    "target":{"agent_id":"agent-id","window_id":4,"tab_id":21},
    "status":"stopped",
    "count":1
  },
  "events": []
}
```

#### 宿主事件

| operation | payload | 发送条件 |
| --- | --- | --- |
| `browser.network` | `BrowserNetworkResult`，必须含 `session_id` | 捕获到匹配请求的响应或失败结果 |
| `browser.network.status` | `BrowserNetworkSession` 风格对象，至少含 `id/status` | 会话启动、停止、目标关闭或故障清理 |
| `browser.targets.changed` | 当前为空对象；后续可增加可选字段 | Agent、窗口、标签页或分组变化 |

网络事件只发送给创建该捕获的插件 Session。结果可包含 URL、方法、请求/响应头、请求体、状态、MIME、资源类型、协议、远端地址、响应正文、Base64 标记、截断标记、缓存来源、耗时、timing 和错误。实现必须忽略未知可选字段。

#### 取消、错误与断线

调用 Context 取消时插件发送：

```json
{"type":"cancel","reply_to":"browser-1"}
```

宿主取消对应 operation，但底层 Chrome 命令可能已经完成。取消与响应存在竞态：插件应立即向调用者返回取消错误，并忽略之后到达的响应；清理逻辑不能假定被取消的 `tab.open` 一定没有创建标签页。网络捕获应显式调用 stop；插件 Session 断开、插件退出或禁用时，宿主会停止其全部捕获作为兜底。

operation 业务失败使用 `response.error`，gRPC stream 保持可用。非法信封可能被忽略；写入失败、Session Context 取消、无法投递控制事件等连接级错误会结束 stream。Session 结束后，客户端应让全部挂起请求返回断线错误并关闭所有捕获事件通道。

#### 并发与背压

gRPC stream 不允许多个 goroutine/thread 同时调用 `Send`。插件和宿主各自必须使用一个写协程及有界发送队列；读取循环不能等待业务消费者。参考 Go SDK 的发送队列容量为 256，单捕获事件通道容量为 128：

- 单个捕获消费者过慢时，只停止该捕获，并让捕获的 `Err()` 返回队列溢出。
- 普通目标变化事件无法入队时可以终止 Session；不能无限增长内存。
- 捕获停止状态属于控制事件。若控制事件持续无法投递，应终止 Session，而不是留下看似运行中的捕获。
- Monitor 一元 RPC、其他插件和其他捕获不能被慢消费者阻塞。

网络捕获是独立会话，不属于 Action 工作流。正常流程必须消费事件、显式停止并释放目标；宿主故障清理不能替代插件自己的生命周期管理。

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
