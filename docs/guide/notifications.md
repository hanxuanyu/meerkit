# 通知

Meerkit 当前提供站内通知、Webhook 和 SMTP 三种通知器。监控条件和状态看板趋势规则都可以向一个或多个渠道发送事件。

## 事件类型

| 来源 | 触发 | 恢复 |
| --- | --- | --- |
| 监控条件 | `triggered` | `recovered` |
| 状态趋势 | `trend_triggered` | `trend_recovered` |

每个事件包含监控、模块、执行记录、触发时间、摘要和当前结果；条件事件还包含上一次结果与规则详情，趋势事件包含看板项、趋势规则和测量明细。

发送在执行记录提交后异步进行。每个渠道最多尝试 3 次，单次超时 20 秒，失败重试前分别等待 500 ms 和 1 s。执行详情中会持续记录各渠道的 `pending`、`sent`、`skipped` 或 `error` 状态。

## 站内通知

数据库初始化时会创建内置站内通知渠道。它支持标题和正文模板，并通过 WebSocket 实时更新通知中心和未读数量。

站内通知可以逐条或全部标记已读，也可以删除所有已读通知。默认保留 30 天，由 `storage.notification_retention` 控制。浏览器系统通知是管理界面的可选增强，需要用户授予浏览器权限；它依赖当前登录页面的站内通知流。

## Webhook

Webhook 支持 `GET` 和 `POST`：

- 查询参数和自定义请求头。
- 1 到 300 秒的请求超时。
- POST 请求体：完整事件 JSON、URL 编码表单、Raw JSON、Raw 文本或无请求体。
- GET 会把 `event_type`、`monitor_name`、`module_type`、`summary` 和 `triggered_at` 附加到查询参数。
- 仅 2xx 响应视为发送成功。

使用“测试”按钮可以在保存前验证 URL、请求格式和目标响应。

## SMTP

SMTP 渠道必填主机、发件人和一个或多个收件人，收件人使用逗号分隔。端口默认 `587`；端口 `465` 使用隐式 TLS，最低 TLS 1.2。填写用户名后会使用 SMTP PLAIN 认证。

主题与正文都是纯文本模板。SMTP 实现不添加 HTML 邮件、附件或自定义 CA 配置。

## 模板占位符

模板使用双花括号路径。常用值包括：

```text
{{event.type}}
{{event.triggered_at}}
{{event.summary}}
{{monitor.name}}
{{monitor.module_type}}
{{result}}
{{result.response.status_code}}
{{previous.response.body_hash}}
```

`result` 表示当前完整结果，`previous` 只在存在上一次结果时可用。配置监控渠道时，界面会提示当前模块无法提供的结果占位符；真正发送时仍会严格检查，缺失占位符会使该次投递失败并写入错误详情。

::: warning 敏感信息
通知配置存储在数据库中。Webhook 请求头和 SMTP 密码不应出现在模板、日志或截图中。请限制数据目录或数据库账户权限，并通过 TLS 保护外部通知连接。
:::

## 测试与排障

1. 在“通知渠道”中先使用独立测试，确认基础连接和认证。
2. 在监控编辑器中检查占位符兼容提示。
3. 手动运行监控并打开该执行记录的通知投递明细。
4. 查看主应用日志中的 `notification delivery` 项。
5. 如果渠道被禁用，投递会显示 `skipped`，不会发送。
