# 前端开发

管理界面位于 `web/`，使用 React 18、Vite、Tailwind CSS 4、Radix UI 基础组件和 Lucide 图标。它是单页应用，生产构建由 Go `embed` 编入宿主二进制。

## 启动与构建

```bash
npm --prefix web ci
npm --prefix web run dev
```

Vite 固定在 `5173`，把 `/api`、`/healthz` 和 WebSocket 代理到 `127.0.0.1:8080`。另一个终端运行：

```bash
go run . serve
```

生产构建：

```bash
npm --prefix web run build
```

输出为 `web/dist`。修改前端后，直接 `go run .` 不会自动重建已存在的 `web/dist`；嵌入式界面测试前应重新执行构建。

## 页面结构

| 路由 | 页面 |
| --- | --- |
| `/` | 总览 |
| `/monitors` | 监控列表 |
| `/monitors/:id/records/:record?` | 监控记录与深链接 |
| `/status-board` | 状态看板 |
| `/notifications/:id?` | 站内通知中心 |
| `/notification-channels` | 通知渠道 |
| `/plugins` | 插件管理 |
| `/browser-debug` | 后端 Catalog 驱动的原子浏览器操作与网络捕获调试台 |
| `/logs` | 系统与插件日志 |
| `/settings` | 启动与运行时配置、管理员密钥 |

后端对非 `/api/` 路径回退到嵌入式 `index.html`，因此刷新深链接仍能加载 SPA。

## 页签保活

工作区页签由 `App` 按 `openTabs` 渲染为稳定的 keyed panel。切换页签只隐藏非当前 panel，不卸载其 React 树，因此搜索词、筛选条件、表单草稿、Action 参数、已打开的详情和局部加载状态都会保留；关闭页签时才卸载 panel 并清理对应的滚动和深链接缓存。主内容滚动位置按页签保存，切回时恢复。新增页面不需要额外接入状态缓存，但必须保持页面组件内部状态可复用。

页签内的 `PageHeader` 使用 `PageChromeScope` 限定当前顶栏标题，隐藏页签不会覆盖当前页面的标题。通知详情和执行记录的 URL 参数只同步到当前页签，后台保活页签的详情选择不会因切换而被清空。

## 数据访问

所有请求经过 `src/lib/api.js`，自动携带同源 Cookie，并在写请求中附加当前 CSRF Token。收到未授权响应时，应用回到登录流程。

实时数据分两类：

- WebSocket：站内通知、状态看板、浏览器目标与网络捕获事件。
- HTTP 流：系统日志、插件日志。

新增实时页面时应同时实现初始快照、增量事件和断线重建，不能只依赖内存事件。

## 动态模块表单

监控、通知和浏览器 Action 表单不为具体类型硬编码。`DynamicFields` 根据后端描述器渲染参数：类型、默认值、范围、选项、显隐和启用条件均来自插件、通知器或 Action Catalog。单行输入、选择和时间控件使用一致高度，字段描述与标题同行；布尔字段使用左右布局的切换卡片，标题和描述在左、Switch 在右；多行文本、JSON 和 Map 按内容自然扩展。

结果展示、条件编辑和状态看板同样读取 `ResultSets`。新增字段类型时要同步检查：

- `lib/parameterSchema.js`
- `lib/resultSchema.js`
- `components/forms/`
- `components/results/`
- 状态看板字段类型推断

浏览器调试页从 `GET /api/v1/browser/actions` 获取 action 分类、目标要求、参数、默认值、显隐规则和结果类型。页面通过 HTTP 执行单个 Action，通过 WebSocket 接收目标变化与网络事件；网络捕获拥有独立的启停 API，不属于 Action Catalog。新增浏览器 action 时，应创建独立的 `internal/browser/action_*.go` 定义文件，为每个参数提供清晰的用途、边界和副作用描述，并加入 `actions.go` 的显式注册表和校验逻辑，再在扩展执行端实现；前端只为新的参数控件或特殊结果类型增加适配，不维护 action 清单。

调试 WebSocket 断开后页面自动重连，并在目标变化时重新获取 Agent 与目标快照。网络事件必须按当前 `session_id` 过滤；捕获启动后保留返回的目标，不能因顶部选择变化而迁移。完整通信和清理规则见[浏览器自动化架构](/development/browser-automation)。

## 样式与交互

共享基础组件放在 `src/components/ui`，业务组合放在 `features`，页面容器放在 `pages`。保持键盘访问、`aria-label`、工具提示、加载/空/错误状态和移动端布局。

应用支持浅色与深色主题。新增颜色时同时检查两种主题，不要让状态只依赖颜色表达；状态看板仍需保留标签和等级文本。

## 验证

当前 `web/package.json` 没有自动化测试脚本。前端变更至少执行：

```bash
npm --prefix web run build
```

然后在开发服务器中手动验证登录、目标页面、移动宽度、深色主题、错误状态和受影响的实时连接。涉及 API 结构时补后端 `httptest`，避免前后端契约只靠人工观察。
