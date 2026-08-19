# 浏览器能力插件开发

浏览器能力插件是普通 Meerkit 监控插件。它不直接连接 Chrome，也不携带 WebDriver；插件通过宿主注入的 `sdk.BrowserClient` 请求原子操作，由 Meerkit 将请求转发给已配对的 Chrome 扩展。插件负责业务步骤、目标生命周期、超时、结果解释和 `Observation`，扩展只负责执行。

## 适用场景与边界

适合使用浏览器能力的场景包括：需要 JavaScript 渲染、登录态、DOM 状态、真实鼠标键盘输入、截图，或需要读取页面请求与响应。只需发 HTTP 请求时应继续使用普通 HTTP 客户端，避免引入浏览器节点、页面资源和调试器占用。

职责边界如下：

| 组件 | 负责 | 不负责 |
| --- | --- | --- |
| 监控插件 | 站点配置、步骤编排、超时、标签页所有权、结果 Schema 和 Observation | Chrome 连接、配对令牌、CDP attachment |
| Meerkit 宿主 | 插件身份、请求路由、校验、默认参数、捕获隔离和故障清理 | 站点业务语义、元素选择策略 |
| Chrome 扩展 | 执行 Chrome API/CDP、返回原子结果、产生网络事件 | 保存监控流程、解释结果、长期复用策略 |

插件不能假定始终存在在线 Agent。浏览器节点可能断线，标签页也可能被用户关闭；这些都必须作为正常执行错误处理。

## 创建插件

从模板或示例开始：

```bash
cp -R plugins/template plugins/my-browser-monitor
```

需要完整的标签页复用、导航前捕获和清理示例时，直接阅读仓库中的 `plugins/browser-example`。入口统一使用 `PluginRuntime`：

```go
func main() {
    runtime := sdk.NewPluginRuntime()
    browser := runtime.Browser()

    runtime.Serve(sdk.NewProvider(
        monitor.New(browser),
    ))
}
```

不要自行构造 `BrowserClient`，也不要从配置接收宿主地址或配对令牌。`runtime.Browser()` 返回的客户端会在宿主建立 `BrowserBridge.Session` 后自动可用；Session 断开时调用返回错误。

模块应通过构造函数显式接收依赖：

```go
type Module struct {
    browser sdk.BrowserClient
}

func New(browser sdk.BrowserClient) *Module {
    return &Module{browser: browser}
}
```

## BrowserClient 接口

```go
type BrowserClient interface {
    ListTargets(context.Context, string) (BrowserTargets, error)
    ExecuteAction(context.Context, BrowserActionRequest) (BrowserActionResult, error)
    StartNetworkCapture(context.Context, BrowserNetworkStartRequest) (BrowserCapture, error)
}
```

### 查询目标

`ListTargets(ctx, agentID)` 返回指定 Agent 的窗口和标签页。`agentID` 为空时由宿主选择一个在线 Agent；返回值中的 `AgentID` 是本次实际使用的节点，后续请求应沿用它。

```go
targets, err := browser.ListTargets(ctx, "")
if err != nil {
    return sdk.Observation{}, fmt.Errorf("list browser targets: %w", err)
}
```

窗口 ID 和标签页 ID 都是 Chrome 运行时标识。不能把它们写入长期模块配置；复用时应在每次执行前重新查询，并用受控的 URL、分组标题等业务特征重新识别目标。

### 执行 Action

```go
result, err := browser.ExecuteAction(ctx, sdk.BrowserActionRequest{
    Target: sdk.BrowserTarget{
        AgentID:  target.AgentID,
        WindowID: target.WindowID,
        TabID:    target.TabID,
    },
    TimeoutMS: 60_000,
    Action: sdk.BrowserAction{
        ID:   "read-status",
        Type: "dom.query",
        Params: map[string]any{
            "selector":   "[data-testid=status]",
            "max_length": 16_384,
        },
    },
})
if err != nil {
    return sdk.Observation{}, fmt.Errorf("query status: %w", err)
}
if !result.Success {
    return sdk.Observation{}, errors.New(result.Error)
}
```

`Action.ID` 是调用方自定义的步骤标识，最长 128 个字符，便于从结果列表中定位步骤；`Action.Type` 必须来自 [Action 参考](/reference/browser-actions)。`TimeoutMS` 默认 60 秒，范围为 1 秒到 5 分钟。调用 Context 的截止时间仍然生效，应略大于 Action 超时并覆盖整个执行流程。

宿主会按 Catalog 补齐省略的默认参数，再执行参数校验和 capability 检查。不要依赖错误类型或英文错误文本做业务分支；协议 v1 只保证返回可读错误。

### 目标模式

| 模式 | 请求要求 |
| --- | --- |
| `none` | 不需要目标；目标字段会被忽略 |
| `window_optional` | 可传 `window_id`；不传时由 Chrome 选择当前窗口 |
| `window_required` | 必须传正整数 `window_id` |
| `tab_required` | 必须传正整数 `tab_id`；同时传 `window_id` 时会校验归属 |

`tab.open` 是 `window_optional`，并且不能传已有 `tab_id`。打开或复制标签页后，应使用 `BrowserActionResult.Target` 取得新的目标；`Data` 中的 `tab_id/window_id` 只用于兼容结果处理。

## 推荐执行流程

一个独占临时标签页的执行流程通常是：

```go
runCtx, cancel := context.WithTimeout(ctx, 70*time.Second)
defer cancel()

opened, err := browser.ExecuteAction(runCtx, sdk.BrowserActionRequest{
    TimeoutMS: 60_000,
    Action: sdk.BrowserAction{
        ID:   "open",
        Type: "tab.open",
        Params: map[string]any{
            "url":    "https://example.com/status",
            "active": false,
            "wait":   true,
        },
    },
})
if err != nil {
    return sdk.Observation{}, err
}
target := opened.Target

defer func() {
    closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer closeCancel()
    _, _ = browser.ExecuteAction(closeCtx, sdk.BrowserActionRequest{
        Target: target,
        Action: sdk.BrowserAction{ID: "close", Type: "tab.close"},
    })
}()

queried, err := browser.ExecuteAction(runCtx, sdk.BrowserActionRequest{
    Target: target,
    Action: sdk.BrowserAction{
        ID: "query",
        Type: "dom.query",
        Params: map[string]any{"selector": "main"},
    },
})
```

清理使用独立的短 Context，因为主执行 Context 在失败路径中通常已经取消。只关闭当前执行创建或明确拥有的目标，不要关闭用户标签页。

## 网络捕获

网络捕获是有状态会话，不是 Action。需要捕获首次导航请求时，先打开 `about:blank`，启动捕获，再执行 `tab.navigate`：

```go
capture, err := browser.StartNetworkCapture(ctx, sdk.BrowserNetworkStartRequest{
    Target: target,
    DisableCache: true,
    Rules: []sdk.BrowserNetworkCaptureRule{
        {
            ID:           "status-api",
            URLContains:  "/api/status",
            ResourceType: "XHR",
            MaxBodyBytes: 256 * 1024,
        },
    },
})
if err != nil {
    return sdk.Observation{}, err
}

eventsDone := make(chan struct{})
go func() {
    defer close(eventsDone)
    for event := range capture.Events() {
        // 可实时消费；Stop 的结果仍会返回宿主缓存的完整事件列表。
        _ = event
    }
}()

// 在捕获建立后触发导航、点击或刷新。

stopped, err := capture.Stop(ctx)
<-eventsDone
if err != nil {
    return sdk.Observation{}, err
}
if captureErr := capture.Err(); captureErr != nil {
    return sdk.Observation{}, captureErr
}
for _, event := range stopped.Events {
    if event.CaptureID == "status-api" {
        // 解释匹配的请求/响应。
    }
}
```

每次捕获必须配置 1 到 32 条规则。`URLContains` 留空表示该规则不限制 URL，`ResourceType` 留空表示不限制类型；资源类型使用 CDP 名称（如 `Document`、`XHR`、`Fetch`、`Script`、`Image`）。`MaxBodyBytes` 最大 1 MiB。宿主每个会话最多缓存 2000 条事件。

`Events()` 的通道容量为 128，启动后应立即持续消费。消费者过慢会只终止该捕获，错误通过 `Err()` 暴露。无论成功还是失败都应调用 `Stop`；重复停止会返回第一次停止的结果或错误。

`BrowserNetworkResult` 主要字段包括：请求 URL、方法、请求头和请求体，响应状态、响应头、MIME、协议和远端地址，正文及 `body_base64/truncated`，缓存来源、耗时、CDP timing 和捕获错误。响应体无法读取或超过限制时，不应把整个监控执行视为协议损坏，应结合 `Error`、`Truncated` 和业务要求判断。

## 标签页复用

默认优先使用一次执行一个临时后台标签页，隔离最清楚。需要保留登录态或降低加载成本时，复用策略必须满足：

1. 配置中明确允许保留标签页。
2. 用插件专属分组标题和规范化 URL 识别，不使用长期保存的 `tab_id`。
3. 复用前重新查询目标，并处理标签页已关闭、已导航或 Agent 已更换。
4. 对同一保留标签页串行执行，避免多个监控相互刷新、输入和读取。
5. 每次执行前明确刷新或导航，不能假定页面仍处于上次状态。

仓库的 `plugins/browser-example/monitor/shared.go` 实现了上述模式。

## DOM 操作与真实输入

`dom.*` 通过脚本直接操作元素，速度快，适合读取、设置表单值和触发常见事件。`input.*` 通过 CDP 产生真实鼠标、键盘和滚轮事件，更接近用户操作，适合依赖坐标、焦点或可信输入序列的页面。

选择器只作用于顶层文档，不自动穿透 iframe 或 shadow root。选择器应优先使用站点稳定属性，如 `data-testid`、`data-test`、`data-qa`、`name` 和 `aria-label`，避免依赖易变的深层 `nth-of-type` 路径。

`runtime.evaluate` 在页面主世界执行任意表达式，既能读取也能修改页面状态。它被标记为敏感且破坏性能力，只应执行插件内固定或严格约束的可信代码，不能把未经验证的用户输入直接拼入表达式。

## 结果与 Observation

Action 的 `Data` 是松散 JSON 对象，插件必须按 action 类型进行类型断言并为缺失字段提供明确错误。不要把整个 Action 原始响应直接写入 Observation；应转换为模块自己稳定、版本化的结果集：

```go
text, ok := queried.Data["text"].(string)
if !ok {
    return sdk.Observation{}, errors.New("dom.query did not return text")
}

value := map[string]any{
    "text":       text,
    "visible":    queried.Data["visible"],
    "duration_ms": queried.Duration,
}
return sdk.Observation{
    Success:       true,
    SchemaVersion: resultSchemaVersion,
    ResultSets:    map[string]map[string]any{"element": value},
    Summary:       "页面状态已读取",
}, nil
```

截图 Base64、页面 HTML、网络正文和 Cookie 等内容可能很大或敏感。模块应设置自己的长度上限，避免日志记录，并只返回监控判断真正需要的数据。

## 错误、取消与并发

- 每个 `Execute` 必须继承宿主 Context，并设置整个流程的截止时间。
- Action 错误立即结束当前业务步骤；清理错误通常只记录到 stderr，不能覆盖主要错误。
- Agent 断线、目标关闭、capability 缺失和 Chrome API 失败都是可重试的执行失败，不是配置校验失败。
- `ValidateConfig` 只能做静态检查，不能依赖在线浏览器。
- 不要无限重试。一次执行内最多做少量、有条件的恢复，例如重新查询被用户关闭的保留标签页。
- 扩展自身有最大并发限制；插件还应对共享标签页、登录态和可变页面资源加互斥。

## 安全要求

- 插件永远不接触扩展配对令牌。
- 不记录 Cookie、Storage、Authorization 头、响应正文、表达式内容或完整 Action 参数。
- `sensitive` 是展示和审计元数据，不会自动擦除插件日志或 Observation。
- `destructive` 只让管理调试界面二次确认；SDK 调用不会弹出确认框。
- 页面脚本和网络数据属于不可信输入，进入日志、通知、HTML 或命令前必须按目标上下文处理。

## 测试与发布检查

模块单元测试应注入假的 `BrowserClient`，验证 Action 顺序、目标传递、错误路径和清理；集成测试使用真实 `PluginRuntime` 和 BrowserBridge。至少覆盖：

- 无在线 Agent、Session 中断和 Context 取消。
- `tab.open` 失败、步骤中途失败后关闭临时标签页。
- 保留标签页被用户关闭后的恢复。
- 导航前启动捕获、事件消费、停止和队列溢出。
- 正文截断、Base64 正文、截图超限和 DOM 元素不存在。
- Action 返回字段缺失或类型不符合预期。

运行：

```bash
cd plugins/my-browser-monitor
go test ./...
go build ./...

cd ../..
go run ./cmd/plugincheck \
  --manifest ./plugins/my-browser-monitor/meerkit-plugin.yaml \
  --artifact ./plugins/my-browser-monitor/my-browser-monitor \
  --suite ./plugins/my-browser-monitor/conformance.json
```

插件进程与 BrowserBridge 的线协议见[跨语言插件协议](/development/plugin-protocol)，宿主到扩展的通信见[浏览器自动化架构](/development/browser-automation)，所有参数和返回结构见 [Action 参考](/reference/browser-actions)。
