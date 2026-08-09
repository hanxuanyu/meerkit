# HTTP API 参考

Meerkit 管理 API 前缀为 `/api/v1`，请求与响应默认使用 JSON。该 API 同时服务仓库内管理界面。

## 认证与 CSRF

只有以下端点无需会话：

- `GET /healthz`
- `GET /readyz`
- `GET /api/v1/auth/status`
- `POST /api/v1/auth/setup`
- `POST /api/v1/auth/login`

登录或初始化成功后，服务设置 `meerkit_session` HttpOnly Cookie，并返回：

```json
{
  "csrf_token": "...",
  "expires_at": "2026-09-08T12:00:00Z"
}
```

除 `GET`、`HEAD`、`OPTIONS` 外的请求必须同时带 Cookie 和 `X-CSRF-Token`。错误响应统一为：

```json
{
  "code": "authentication_required",
  "message": "authentication required"
}
```

## 分页

监控、记录、通知和插件列表使用：

```json
{
  "items": [],
  "page": 1,
  "page_size": 20,
  "total": 0,
  "total_pages": 0
}
```

`page` 和 `page_size` 从 1 开始；插件页大小最多 100，Store 列表也会规范化分页值。

## 认证路由

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/auth/status` | 是否已初始化管理员 |
| POST | `/auth/setup` | 首次设置 `access_key` 与 `confirm` |
| POST | `/auth/login` | 使用 `access_key` 登录 |
| GET | `/auth/session` | 获取 CSRF Token 与过期时间 |
| POST | `/auth/logout` | 注销当前会话 |
| POST | `/auth/change-key` | 使用当前密钥修改密钥并撤销会话 |

## 监控与描述器

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/modules` | 当前活动监控模块描述器 |
| GET | `/modules/:type` | 单个模块描述器 |
| GET | `/monitors` | 监控分页列表 |
| POST | `/monitors` | 创建监控 |
| GET | `/monitors/:id` | 监控详情 |
| PATCH/PUT | `/monitors/:id` | 更新监控 |
| DELETE | `/monitors/:id` | 删除监控及关联记录/看板项 |
| POST | `/monitors/:id/run` | 手动执行并保存记录 |
| POST | `/monitors/test` | 校验并执行模块，不保存记录 |
| POST | `/schedules/preview` | 校验 Cron 并返回三次时间 |
| GET | `/monitors/:id/next-runs` | 返回五次计划时间 |
| GET | `/monitors/:id/records` | 记录分页列表 |
| GET | `/monitors/:id/records/:record_id` | 记录及描述器快照 |
| DELETE | `/monitors/:id/records` | 清空监控记录并重置看板趋势状态 |

监控列表查询：`page`、`page_size`、`q`、`module_type`、`status`。状态支持 `enabled`、`disabled`、`triggered`、`healthy`、`waiting`、`unavailable`。

记录查询：`page`、`page_size`、`q`、`status`、`event_type`。

创建监控示例：

```json
{
  "name": "Production API",
  "module_type": "http",
  "schedules": ["*/5 * * * *"],
  "enabled": true,
  "module_config": {
    "url": "https://example.com/health",
    "method": "GET"
  },
  "condition_config": {
    "logic": "ALL",
    "notification_policy": "once",
    "rules": [
      {
        "id": "status-code",
        "field": "response.status_code",
        "source": "current",
        "operator": "not_equals",
        "value_source": "literal",
        "value": "200"
      }
    ]
  },
  "notification_channel_ids": ["builtin-inapp"]
}
```

## 通知

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/notifiers` | 通知器描述器 |
| GET/POST | `/notification-channels` | 列表或创建渠道 |
| GET/PATCH/PUT/DELETE | `/notification-channels/:id` | 渠道详情与管理 |
| POST | `/notification-channels/test` | 测试未保存配置 |
| POST | `/notification-channels/:id/test` | 测试已保存渠道 |
| GET | `/in-app-notifications` | 站内通知分页列表 |
| GET | `/in-app-notifications/unread-count` | 未读数量 |
| GET | `/in-app-notifications/:id` | 通知详情 |
| PATCH | `/in-app-notifications/:id/read` | 标记已读 |
| POST | `/in-app-notifications/read-all` | 全部已读 |
| DELETE | `/in-app-notifications/read` | 删除全部已读 |
| GET WebSocket | `/in-app-notifications/ws` | 实时通知事件 |

站内列表查询：`page`、`page_size`、`q`、`unread=true`。

## 状态看板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/status-board` | 按监控分组的完整快照 |
| GET | `/status-board/sources?monitor_id=...` | 可选条件与结果来源 |
| POST | `/status-board/items` | 创建看板项 |
| GET/PATCH/DELETE | `/status-board/items/:id` | 看板项管理 |
| GET WebSocket | `/status-board/ws` | 新记录和配置变更事件 |

创建项至少需要 `name`、`monitor_id` 和 `source`。`history_limit` 范围为 20 到 200。

## 插件

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/plugins` | 插件分页与筛选 |
| POST | `/plugins/import` | Multipart 字段 `package` 上传，最多约 260 MiB |
| POST | `/plugins/scan` | 扫描 inbox |
| GET | `/plugins/:id/:version` | 安装与模块描述器详情 |
| POST | `/plugins/:id/:version/trust` | 确认 `fingerprint` |
| POST | `/plugins/:id/:version/enable` | 启用，可传 `confirm_unverified` |
| POST | `/plugins/:id/:version/disable` | 禁用 |
| DELETE | `/plugins/:id/:version` | 卸载 |
| GET | `/plugins/:id/:version/export` | 下载原插件包 |
| GET | `/plugins/:id/:version/logs` | 最近 128 KiB 日志 |
| GET SSE | `/plugins/:id/:version/logs/stream` | 日志快照流 |

插件列表查询：`page`、`page_size`、`q`、`status`、`trust_state`。

## 系统与配置

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/system` | 监听、保留周期和时区摘要 |
| GET | `/system/config` | 启动配置来源与运行时配置元数据 |
| PATCH | `/system/config/runtime/:type` | 按路径或整类更新，使用 `version` |
| POST | `/system/config/runtime/:type/reset` | 恢复一类默认值 |
| POST | `/system/config/runtime/reset` | 恢复全部默认值 |
| GET | `/system/logs?source=business|access` | 最近 128 KiB 日志 |
| GET SSE | `/system/logs/stream?source=...` | 日志快照流 |

按路径更新运行时配置：

```json
{
  "version": 1,
  "path": "scheduler.max_concurrency",
  "value": 24
}
```

版本冲突返回 HTTP 409 和 `config_version_conflict`。

## 实时连接

WebSocket 只允许无 Origin 或与请求 Host 相同的 Origin，每 30 秒发送 Ping。SSE 日志流每秒检查快照，15 秒发送 heartbeat，并设置 `X-Accel-Buffering: no`。
