# 日志与排障

Meerkit 把宿主业务日志、HTTP 访问日志和每个插件版本的进程日志分开保存。管理界面的“系统日志”页可以查看并实时跟随这些来源。

## 默认路径

| 日志 | 默认路径 |
| --- | --- |
| 主应用 | `./logs/meerkit.log` |
| HTTP 访问 | `./logs/meerkit-access.log` |
| 插件进程 | `./data/plugins/logs/<id>-<version>.log` |

主日志默认同时输出到控制台和轮转文件；访问日志默认只写文件。文件达到 100 MiB 后轮转，最多保留 7 个旧文件、30 天，并压缩旧文件。

运行时可以切换 `debug`、`info`、`warn`、`error` 级别，以及 `text`、`simple`、`json` 格式。至少要启用控制台或业务文件之一。

## 启动失败

先直接运行并观察 stderr：

```bash
./meerkit --config /etc/meerkit/config.yaml serve
```

常见原因：

- 配置文件路径错误或 YAML 无法解析。
- 端口已占用，或监听地址无效。
- 数据目录、日志目录不可写。
- MySQL DSN 无法连接、缺少数据库或权限不足。
- `auto_migrate: false` 但数据库 Schema 尚未安装。
- `plugins.trusted_keys` 不是合法 Base64 Ed25519 公钥。

使用 `GET /readyz` 区分 HTTP 进程存活与数据库可用性。

## 插件无法启用

在插件详情中检查 `trust_state`、`status` 和 `error`，再查看插件日志。

| 错误阶段 | 检查项 |
| --- | --- |
| 导入 | 包格式、清单、当前平台制品、大小和 SHA-256 |
| 信任 | 签名公钥指纹是否已确认；未签名包是否明确确认风险 |
| 启动 | 制品可执行权限；解释器命令是否在服务账户 `PATH` 中 |
| 握手 | stdout 第一行是否为 go-plugin 握手，业务日志是否误写 stdout |
| 健康 | 标准 gRPC Health 的 `plugin` 服务是否为 `SERVING` |
| 模块 | `ListModules` 是否与清单类型和版本一致 |
| 迁移 | `MigrateConfig` 是否能处理已保存的配置版本 |

第三方制品先离线运行：

```bash
./meerkit-plugincheck \
  --manifest ./meerkit-plugin.yaml \
  --artifact ./plugin \
  --suite ./conformance.json
```

## 监控不执行

1. 确认监控已启用。
2. 确认其模块与指定模块版本当前可用。
3. 在编辑器中预览 Cron 和系统时区。
4. 检查主日志是否出现 `schedule is invalid`、`module is unavailable` 或 `concurrency limit reached`。
5. 手动执行一次，区分调度问题和插件执行问题。

达到全局并发上限时，到期任务不会进入等待队列，而是记录警告并跳过该次。需要根据监控耗时和频率调整 `scheduler.max_concurrency`。

## 条件没有触发

- `changed` 第一次执行只建立基线。
- 条件字段应来自插件描述器；JSON 路径或类型错误会得到 `unknown`。
- 上一次比较只使用最近一次成功执行结果。
- `ALL` 中的未知值会让整体为 `unknown`，除非另有规则已经为 `false`。
- 修改监控会清空运行时状态，下一次执行重新建立活动状态。

打开执行详情查看每条规则的 `actual`、`expected`、`state` 和 `message`。

## 通知未送达

先查看执行记录中的投递状态：

- `skipped`：渠道已禁用。
- `error`：打开消息查看目标响应、网络、认证或模板错误。
- `pending` 长时间不变：检查宿主是否在记录提交后退出，或数据库更新是否失败。

Webhook 只有 2xx 成功。SMTP 端口 465 使用隐式 TLS；其他端口使用标准 SMTP 流程。模板中的未知占位符会让发送失败。

## 日志流断开

系统与插件日志流使用 HTTP 流式响应，站内通知和状态看板使用 WebSocket。通过反向代理时要允许 Upgrade、关闭不合适的响应缓冲，并设置足够长的空闲超时。页面会在流断开后重连或重新拉取快照，但代理配置仍决定实时体验。
