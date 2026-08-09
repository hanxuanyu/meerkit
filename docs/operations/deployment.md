# 部署与升级

Meerkit 的正式发布物是单个平台压缩包：宿主可执行文件内嵌管理界面，同级 `plugins/` 提供官方插件包，并附带 `meerkit-plugincheck`、示例配置、README 和 Apache-2.0 许可证。

## 构建发布包

从源码构建需要 Go 1.26、Node.js 22、npm 和平台打包工具。

```bash
make deps
make package-release \
  VERSION=v0.1.0 \
  TARGETS=linux/amd64,linux/arm64
```

输出位于 `dist/releases`。未设置签名变量会生成未签名官方插件；正式分发应按[打包与发布](/development/releasing)配置 Ed25519 私钥和 key ID。

## 目录规划

建议把只读发布内容和可写运行数据分开：

```text
/opt/meerkit/
  meerkit
  meerkit-plugincheck
  plugins/
  LICENSE
/etc/meerkit/config.yaml
/var/lib/meerkit/
/var/log/meerkit/
```

```yaml
server:
  address: 127.0.0.1
  port: 8080
storage:
  data_dir: /var/lib/meerkit
logging:
  file:
    directory: /var/log/meerkit
plugins:
  source_dir: /opt/meerkit/plugins
```

正式宿主不会从 `plugins.source_dir` 构建源码；它会从可执行文件同级的 `plugins/` 引导发行包。`source_dir` 只对版本为 `dev` 的宿主生效。

## 进程管理

Meerkit 在收到 `SIGINT` 或 `SIGTERM` 后停止调度、关闭插件，并给 HTTP 服务最多 10 秒完成优雅退出。可以交给 systemd、runit、s6 或容器编排器管理。

下面只是部署方可采用的 systemd 示例，仓库不会生成或安装该单元：

```ini
[Unit]
Description=Meerkit monitoring service
After=network-online.target

[Service]
User=meerkit
Group=meerkit
WorkingDirectory=/opt/meerkit
ExecStart=/opt/meerkit/meerkit --config /etc/meerkit/config.yaml serve
Restart=on-failure
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

插件是宿主的子进程，服务账户必须能执行已安装制品，并能在数据目录创建 Unix Domain Socket、插件日志和安装文件。

## 健康检查

无需认证的端点：

| 端点 | 成功 | 含义 |
| --- | --- | --- |
| `GET /healthz` | `200 {"status":"ok"}` | HTTP 进程正在响应 |
| `GET /readyz` | `200 {"status":"ready"}` | 数据库 Ping 成功 |

`readyz` 不检查每个插件或外部监控目标。插件健康状态应从插件管理页或 API 获取。

## 反向代理

应用当前只启动普通 HTTP Server，不直接配置 TLS。跨主机或公网访问时应：

1. 让 Meerkit 监听回环或受保护的内部地址。
2. 由反向代理终止 HTTPS。
3. 转发普通 HTTP、WebSocket Upgrade 和长连接日志流。
4. 限制源站端口只对代理开放。
5. 为登录端点增加边缘限速和访问日志保护。

## 单实例约束

当前调度器没有分布式任务租约。同一个数据库同时运行多个 Meerkit 实例会让各实例分别调度同一监控，可能产生重复执行和通知。因此当前部署应保持单个活动宿主；数据库高可用不等于应用可以水平扩容。

## 备份

至少备份：

- SQLite：停机后的整个 `storage.data_dir`。
- MySQL：数据库的一致性备份。
- 已导入插件包和信任状态：它们分别位于数据目录和数据库，两者应保持同一恢复点。
- 外部配置文件。
- 官方插件签名私钥仅在发布系统单独备份，绝不能放入应用备份或发布包。

日志通常不参与业务恢复，可按审计要求另行归档。

## 升级流程

1. 读取新版本说明并备份数据库和数据目录。
2. 停止旧宿主，避免 SQLite 快照和任务重复。
3. 替换宿主可执行文件、`meerkit-plugincheck` 和同级 `plugins/`。
4. 保留原配置、数据目录和日志目录。
5. 启动新宿主；默认 `auto_migrate` 会应用数据库迁移。
6. 检查 `/readyz`、系统日志、插件状态和一个手动监控执行。

不要用同一个插件 ID 和版本发布不同内容，导入器会拒绝这种替换。插件升级应提升包版本，并在配置结构变化时提升 `config_version` 和实现迁移。
