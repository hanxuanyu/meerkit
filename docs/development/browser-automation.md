# 浏览器自动化架构

浏览器自动化由 Meerkit 主进程内的 `BrowserManager` 统一协调。Chrome 扩展是无业务状态的通用执行端；监控插件保存站点配置、调用顺序和结果语义；管理前端只提供设置与原子能力调试。

## 组件与通信路径

```text
管理前端
  | HTTP：快照、Catalog、命令
  | WebSocket：目标变化、网络事件、捕获状态
  v
Meerkit 主进程
  +-- internal/api
  +-- internal/plugin/Manager
  +-- internal/browser/Manager
        | WebSocket protocol 1
        v
  Chrome Extension

Meerkit 主进程
  | 同一条 HashiCorp go-plugin gRPC/HTTP2 连接
  |-- Monitor：一元 RPC
  |-- BrowserBridge.Session：双向流
  v
监控插件子进程
```

`BrowserManager` 不是独立进程，不监听额外 Capability 端口。插件身份来自 `PluginManager` 管理的子进程和 gRPC 连接，不需要第二套 endpoint、token 或回调鉴权。前端不使用 SSE；命令走 HTTP，增量事件走项目现有 WebSocket 模式。

## BrowserManager 职责

`internal/browser/manager.go` 维护：

- 扩展配对、Agent 连接、心跳、能力声明和替换连接。
- 窗口、标签页与分组目标查询。
- 单 Action 校验、能力检查、请求 ID 和响应关联。
- 网络捕获注册、事件缓存、所有权和停止清理。
- 前端订阅与插件 Session 的事件隔离。

所有 Chrome 命令最终经同一个 Agent WebSocket 写锁发送。待响应请求以随机 ID 关联；HTTP Context 取消后宿主停止等待，不会让其他请求相互影响。

## Chrome WebSocket 协议

扩展端点是 `/api/v1/browser/extension/ws`。该路由不使用管理会话认证，第一条消息必须是协议版本 `1` 的 `hello`：

```json
{
  "protocol": 1,
  "type": "hello",
  "token": "pairing-token",
  "payload": {
    "id": "stable-agent-id",
    "name": "Local Chrome",
    "version": "0.2.1",
    "capabilities": ["browser.targets", "tab.open", "network.start"]
  }
}
```

宿主校验令牌和 Agent 元数据后返回 `welcome`。运行期信封字段为 `protocol`、`type`、`id`、`command`、`payload` 和 `error`。

| 方向 | 类型 | 命令/用途 |
| --- | --- | --- |
| 宿主到扩展 | `command` | `browser.targets`、`browser.action`、`browser.network.start`、`browser.network.stop` |
| 扩展到宿主 | `response` | 使用相同 `id` 返回较小的 payload 或 error |
| 扩展到宿主 | `response_chunk` | 使用相同 `id`、`sequence`、`total` 和 `chunk` 顺序返回大结果 |
| 扩展到宿主 | `event` | `browser.targets.changed`、`browser.network`、`browser.network.status` |
| 双向 | `ping` / `pong` | 应用层连接活性；宿主还发送 WebSocket Ping frame |

协议版本保持 `1`，当前实现不解析旧 `browser.run` 工作流消息。新增命令时应直接更新宿主、扩展和测试，不增加旧结构兼容分支。

完整页面截图等大结果不会作为一个超大 WebSocket frame 发送。扩展将超过 512 KiB 的序列化响应拆为有序 `response_chunk`，并在浏览器发送缓冲区超过 4 MiB 时等待背压释放；宿主按请求 ID 重组。单 frame 最大 8 MiB，重组结果最大 64 MiB、最多 128 块；扩展主动把结果限制为 60 MiB。超过上限时只终止当前请求，并提示改用 WebP/JPEG，不会断开整个 Agent。

## 目标与原子 Action

Action Catalog 的注册表位于 `internal/browser/actions.go`，具体定义按 Action 文件维护，并通过 `GET /api/v1/browser/actions` 提供给前端。参数直接使用监控模块和通知渠道共用的 `sdk.ParameterDescriptor`，支持默认值、范围、选项、单位、全宽、显隐和启用条件；前端统一交给 `DynamicFields` 渲染。后端先校验 Action，再检查 Agent 声明的 capability，扩展仍需校验 Chrome 中的真实目标状态。

当前 Action：

| 分类 | Action | 目标 | 结果重点 |
| --- | --- | --- | --- |
| 标签页 | `tab.open` | 可选窗口 | 新窗口/标签页 ID、URL、标题 |
| 标签页 | `tab.navigate`、`tab.group`、`tab.close` | 必选标签页 | 更新后的目标或状态 |
| 页面 | `page.wait`、`page.screenshot` | 必选标签页 | 等待状态或 PNG/JPEG/WebP 图片数据 |
| DOM | `dom.document`、`dom.query`、`dom.click`、`dom.input` | 必选标签页 | HTML、元素信息或操作状态 |
| 运行时 | `runtime.evaluate` | 必选标签页 | 可 JSON 序列化的表达式结果 |

`tab.open` 不能携带已有 `tab_id`。页面 Action 必须携带 `tab_id`；若同时携带 `window_id`，扩展通过 `chrome.tabs.get` 校验归属。目标 ID 是 Chrome 运行时标识，不应写入长期插件配置。

新增 Action 时需要：

1. 新增一个 Action 定义文件，并在 `internal/browser/actions.go` 的显式注册表添加一项。
2. 在扩展 `CAPABILITIES` 与 `executeAction` 中实现。
3. 明确目标模式、超时、返回结构和资源上限。
4. 为后端 Catalog/校验和扩展执行补测试。
5. 参数优先使用 `sdk.ParameterDescriptor`；只有无法用 `DynamicFields` 或通用结果预览承接时才增加前端适配。

## 网络捕获生命周期

网络捕获与 Action 分离，启动请求包含目标和 1 到 32 条规则。每条规则可以按 URL 包含文本和资源类型过滤，`max_body_bytes` 最大为 1 MiB。

```text
StartNetworkCapture
  -> BrowserManager 预注册 session 与 owner
  -> browser.network.start
  -> 扩展 attach chrome.debugger + Network.enable
  -> 扩展持续发送 browser.network 事件
  -> BrowserManager 缓存并分发到 owner/前端
StopNetworkCapture
  -> browser.network.stop
  -> 扩展 flush 正在读取的 response body
  -> detach debugger
  -> 返回 session 摘要与缓存事件
```

捕获会话固定绑定 `agent_id/window_id/tab_id`。扩展将 CDP 的 request/response/loading 事件按 request ID 关联，并在 `loadingFinished` 后读取正文。宿主最多为一个会话保留 2000 条事件；SDK 的单捕获事件通道容量为 128。

## 插件 BrowserBridge

插件统一使用：

```go
runtime := sdk.NewPluginRuntime()
browser := runtime.Browser()

runtime.Serve(sdk.NewProvider(
    monitor.New(browser),
))
```

`PluginRuntime` 在 go-plugin gRPC Server 上同时注册 `Monitor` 和 `BrowserBridge`。宿主通过已有 `grpcClient.Conn` 建立 `Session`，等待插件先发送 `ready`，再允许插件启用完成。

流内 JSON 信封为：

```json
{
  "type": "ready|request|response|event|cancel",
  "id": "browser-1",
  "reply_to": "browser-1",
  "operation": "browser.action",
  "payload": {},
  "error": ""
}
```

支持的 operation 是 `browser.targets`、`browser.action`、`browser.network.start`、`browser.network.stop`、`browser.network`、`browser.network.status` 和 `browser.targets.changed`。一个 Session 只有一个 gRPC 写协程；请求通过 ID 关联，Context 取消发送 `cancel`，早于 start 响应到达的网络事件会按 session ID 暂存。

## 隔离、背压与清理

宿主为每个捕获记录插件 owner，网络事件只进入对应插件 Session；前端订阅只能查看管理 API 已接收的事件，不参与插件业务执行。

有界队列规则：

- 插件 Session 发送队列容量为 256。
- SDK 单捕获事件通道容量为 128。
- 单捕获消费者过慢时，只停止该捕获并通过 `BrowserCapture.Err()` 返回原因。
- 无法投递控制状态时终止对应插件 BrowserBridge，不阻塞其他插件或扩展读取循环。

清理触发点包括插件禁用、升级或退出，Agent 替换或断线，标签页关闭，显式停止以及 Context 取消。插件业务代码仍应使用 `defer` 关闭自己创建的标签页，并显式停止捕获；宿主清理是故障兜底，不是正常控制流。

## 调试 API 与前端

浏览器工具先通过 HTTP 获取状态、目标和 Catalog，再分别调用单 Action 与捕获 API。桌面端使用可滚动的分组 Action 列表、参数配置和结果三栏布局；移动端自动改用 Action 下拉列表。三栏标题保持相同高度；Action 标题右侧的目标下拉默认选择“无”并继承顶部目标，也可以直接切换到具体标签页或窗口。结果区提供 UI 预览、原始请求和原始响应，原始数据支持快速复制。`/api/v1/browser/debug/ws` 推送目标变化、网络事件与捕获状态，连接时先发送当前捕获快照，并每 20 秒发送 WebSocket Ping。

捕获事件必须按 `session_id` 过滤。前端切换顶部标签页不会修改已启动捕获；扩展断线或目标关闭产生的 stopped 状态会同步结束界面中的活动状态。完整端点和载荷见 [HTTP API 参考](/reference/http-api#浏览器自动化)。

## 验证

浏览器改动至少执行：

```bash
go test ./...
(cd sdk && go test -race ./...)
(cd plugins/browser-example && go test ./...)
(cd browser-extension && node --check background.js && node --test background.test.js)
(cd web && npm run build)
```

这些命令不启动宿主、前端开发服务器或 Chrome。人工验证时再加载扩展，检查目标变化、Action 结果、捕获启停、标签页关闭和连接恢复。
