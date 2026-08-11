# 架构与仓库

Meerkit 是一个带独立插件进程的 Go 单体服务。宿主拥有调度、状态和数据一致性，插件只拥有监控模块的配置与执行逻辑。

## 运行时结构

```text
Admin Browser
  | HTTP / WebSocket
  v
Gin API + embedded React UI
  |
  +-- Auth service -------- admin hash, sessions, CSRF
  +-- Runtime config ------ database-backed live settings
  +-- Scheduler/Runner ---- cron, execution, condition state
  +-- Notification -------- in-app, Webhook, SMTP
  +-- Status board -------- sample mapping, trend state
  +-- Browser manager ----- extension pairing, agent routing, capabilities -- WebSocket --> Chrome Extension
  +-- Plugin manager ------ package trust, process lifecycle --------------- local gRPC --> plugin child process
  +-- Bun store ----------- SQLite or MySQL

plugin child process -- local capability gRPC --> Browser manager
```

HTTP API 负责校验输入和组织资源；应用服务在启动时装配所有依赖；核心包定义不依赖传输的领域契约；Store 用 Bun 共享领域模型并针对 SQLite/MySQL 处理连接与迁移。

## 所有权边界

| 组件 | 负责 | 不负责 |
| --- | --- | --- |
| 宿主 | 调度、持久化、条件、通知、认证、插件信任与进程 | 具体 HTTP/TCP 探测语义 |
| 监控插件 | 参数描述、结果描述、校验、执行、配置迁移 | Cron、数据库、通知投递 |
| Chrome Extension | 通用标签页、DOM、脚本、截图和 CDP 网络操作 | 站点地址、采集流程与结果语义 |
| Browser Manager | 扩展配对、节点会话、指令路由和插件能力授权 | 站点专有业务流程 |
| Web 前端 | 管理操作、动态表单、结果展示、实时流 | 独立业务状态或直接数据库访问 |
| 文档站点 | 使用、运维、开发和参考文档 | 运行时管理界面 |

## 仓库布局

```text
cmd/pluginpack/            Go 官方插件构建、组包和签名
cmd/plugincheck/           语言无关制品黑盒检查入口
docs/                      VitePress 文档站点
internal/api/              Gin 路由、认证中间件、HTTP/WebSocket
internal/app/              启动与运行时配置定义
internal/application/      服务装配和生命周期
internal/auth/             管理密钥与会话
internal/browser/          浏览器节点管理与插件能力服务
internal/command/          Cobra 命令树
internal/core/             领域模型、条件和结果契约
internal/monitor/          模块注册表与远程插件适配器
internal/notification/     内置通知器
internal/plugin/           包导入、签名信任和子进程管理
internal/pluginconformance/ 黑盒协议检查实现
internal/runtime/          执行器、调度器和清理任务
internal/runtimeconfig/    数据库运行时配置管理
internal/statusboard/      看板样本、趋势与实时 Hub
internal/store/            SQLite/MySQL repository 与迁移
plugins/                   HTTP、TCP、浏览器示例与模板独立 Go 模块
sdk/                       Go SDK、Proto、JSON Schema
web/                       React/Vite 管理界面
browser-extension/         平台维护的通用 Chrome 执行端
```

## Go Workspace

根 `go.work` 包含根模块、SDK 和四个插件模块。这样本地插件会使用工作区里的 SDK 代码，同时每个插件仍能独立测试与发布。

```bash
go test ./...
(cd sdk && go test ./...)
(cd plugins/browser-example && go test ./...)
(cd plugins/http && go test ./...)
(cd plugins/tcp && go test ./...)
(cd plugins/template && go test ./...)
```

根 `go test ./...` 不会自动递归测试其他 Go module，因此 CI 或本地提交前需要显式运行每个模块。

## 关键数据流

监控执行首先读取最近一次成功结果，调用插件，再由宿主补充公共执行字段和条件结果。监控记录、监控运行时状态、状态看板趋势状态和待发送事件在一个存储提交中更新；提交后才异步发送通知。

插件升级先启动候选进程、健康检查、获取并保存描述器，再迁移已有监控配置。只有全部完成后才替换注册表中的旧模块和进程。

## 版本概念

项目存在四个独立版本轴：

- Meerkit 发布版本：宿主和完整发布包。
- 插件包版本：插件 ID 下的一次不可变发布，使用语义版本。
- 模块版本：模块实现兼容性。
- 配置版本与结果 Schema 版本：分别控制迁移和持久化结果解释。

插件协议还有独立整数版本，当前为 `1`。不要用插件包版本替代协议或数据 Schema 版本。
