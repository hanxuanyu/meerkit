<p align="center">
  <img src="web/public/brand-mark.png" width="72" height="72" alt="Meerkit">
</p>

# Meerkit

[English](README.en.md) · [使用文档](docs/index.md) · [插件协议](sdk/PROTOCOL.md) · [Apache-2.0](LICENSE)

Meerkit 是一个自托管监控服务。它负责定时执行监控、保存结果、判断条件并发送通知；具体探测能力由独立插件进程提供。仓库当前提供 HTTP 与 TCP 官方插件，以及 Webhook、SMTP 和站内通知渠道。

## 当前能力

- 通过一个二进制提供管理界面、HTTP API、Cron 调度器、清理任务和插件宿主。
- 创建、测试、启停和手动执行监控；查看执行历史、结构化结果及通知投递结果。
- 使用 `ALL`/`ANY` 组合条件，比较当前值、上次值或固定值，支持变化、文本、正则、数值和存在性判断。
- 使用状态看板聚合条件或结果字段，配置布尔映射、数值阈值、历史窗口和趋势规则。
- 使用 SQLite（默认）或 MySQL 保存配置、执行记录、通知和插件状态。
- 在 Web 页面中管理启动配置来源、运行时配置、系统日志和插件日志。
- 以 `.zip` 或 `.tar.gz` 安装插件，校验制品哈希与 Ed25519 签名，并管理发布者信任。
- 通过 gRPC + JSON 线协议支持第三方语言插件，提供 JSON Schema、Proto 和黑盒一致性检查工具。

## 快速开始

源码开发需要 Go 1.26、Node.js 22、npm 和 Make。

```bash
make deps
make dev
```

打开 `http://127.0.0.1:5173`。开发前端会把 API 和 WebSocket 请求代理到 `http://127.0.0.1:8080`。首次访问需要设置至少 12 个字符的管理员访问密钥。

开发模式下，Vite 负责前端热更新，Air 会在 Go 源码变化后自动重新编译并重启后端。

也可以只运行嵌入式生产界面：

```bash
make frontend-build
go run . serve --listen 127.0.0.1:8080
```

首次启动会在工作目录生成 `config.yaml`，默认数据和日志分别写入 `./data` 与 `./logs`。默认数据库是 `${storage.data_dir}/meerkit.db`。

> `make reset` 会删除默认数据目录、日志、配置和本地 SQLite 文件。它适合重置开发环境，不适合生产环境。

完整安装、首次配置和监控创建流程见[快速开始](docs/guide/getting-started.md)。

## 插件模型

插件是由宿主启动和监管的本地子进程。宿主与插件通过 HashiCorp go-plugin 握手建立本地 gRPC 连接，业务负载使用 UTF-8 JSON；Unix 使用 Unix Domain Socket，Windows 使用回环 TCP。插件不是远程 HTTP 服务，也不运行在安全沙箱中。

官方插件继续使用 Go SDK 开发。第三方插件可以使用任意能实现 gRPC 服务并产出单文件制品的语言：

- Go 开发入口：[plugins/template](plugins/template/README.md) 与 [sdk](sdk/README.md)
- 跨语言规范：[sdk/PROTOCOL.md](sdk/PROTOCOL.md)
- 机器可读契约：[Proto](sdk/proto/monitor.proto) 与 [JSON Schema](sdk/schema/)
- 黑盒检查：`go run ./cmd/plugincheck --manifest ... --artifact ... --suite ...`

Meerkit 的打包器只构建仓库内的 Go 插件。第三方语言维护者自行构建制品，并通过清单级 `runtime` 选择直接执行或宿主解释器启动；单个平台可以用 `artifacts[].runtime` 覆盖。

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `make dev` | 同时启动 Go 后端和 Vite 前端 |
| `make frontend-build` | 构建嵌入宿主的管理界面 |
| `make docs-dev` | 启动 VitePress 文档站点 |
| `make docs-build` | 构建静态文档站点 |
| `go test ./...` | 运行根 Go 模块测试 |
| `make package-plugins` | 打包全部可发布的官方插件 |
| `make package-release VERSION=v0.1.0` | 构建宿主、检查工具与官方插件发布包 |
| `meerkit admin reset-key --key '...'` | 重置管理员密钥并撤销现有会话 |

运行 `make help` 或 `go run . --help` 查看完整参数。

## 文档导航

- 使用：[快速开始](docs/guide/getting-started.md)、[监控与条件](docs/guide/monitoring.md)、[通知](docs/guide/notifications.md)、[状态看板](docs/guide/status-board.md)、[插件管理](docs/guide/plugins.md)
- 运维：[配置](docs/operations/configuration.md)、[部署与升级](docs/operations/deployment.md)、[安全边界](docs/operations/security.md)、[日志与排障](docs/operations/troubleshooting.md)
- 开发：[架构](docs/development/overview.md)、[后端](docs/development/backend.md)、[前端](docs/development/frontend.md)、[Go 插件](docs/development/plugin-go.md)、[跨语言协议](docs/development/plugin-protocol.md)
- 参考：[CLI](docs/reference/cli.md)、[HTTP API](docs/reference/http-api.md)、[插件清单](docs/reference/plugin-manifest.md)、[配置字段](docs/reference/configuration.md)

## 仓库结构

```text
cmd/              插件打包与一致性检查工具
docs/             VitePress 文档站点
internal/         API、调度、存储、认证、插件与运行时实现
plugins/          HTTP、TCP 官方插件与 Go 插件模板
scripts/          Unix 与 Windows 打包脚本
sdk/              Go SDK、Proto、JSON Schema 与协议文档
web/              React/Vite 管理界面
```

## 开源协议

Meerkit 的代码与项目文档以 [Apache License 2.0](LICENSE) 开源。第三方依赖和第三方插件仍适用各自的许可证。
