# 配置

Meerkit 把配置分成两类：启动配置决定进程如何连接外部资源，运行时配置保存在数据库中并可从设置页修改。

## 启动配置优先级

从低到高依次是：

1. 代码默认值。
2. `config.yaml`。
3. `MEERKIT_*` 环境变量。
4. 命令行参数。

默认读取工作目录的 `config.yaml`；文件不存在时，`serve` 会写入默认文件。显式指定 `--config` 或 `MEERKIT_CONFIG_FILE` 后，文件不存在会直接报错。

嵌套环境变量使用双下划线：

```bash
MEERKIT_SERVER__PORT=9090 \
MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit \
./meerkit serve
```

设置页会显示每个启动字段的当前值、默认值和来源。数据库 DSN 只显示是否已配置，不返回原文。

## 配置文件

```yaml
server:
  address: 0.0.0.0
  port: 8080

storage:
  data_dir: ./data
  database:
    type: sqlite
    dsn: ""
    auto_migrate: true
    max_open_conns: 0
    max_idle_conns: 0
    conn_max_lifetime: ""
    conn_max_idle_time: ""

logging:
  file:
    directory: ./logs
    filename: meerkit.log
    max_size_mb: 100
    max_backups: 7
    max_age_days: 30
    compress: true
    access:
      filename: meerkit-access.log

security:
  master_key_file: ./data/master.key

plugins:
  source_dir: ./plugins
  trusted_keys: {}
```

完整字段与环境变量映射见[配置字段参考](/reference/configuration)。

## SQLite

SQLite 是默认数据库。`dsn` 为空时使用 `${storage.data_dir}/meerkit.db`，并启用：

- WAL 日志模式。
- 外键。
- 5 秒 busy timeout。
- `synchronous=NORMAL`。

连接池默认最多 4 个打开连接和 4 个空闲连接。单机部署和本地开发不需要额外数据库服务。

::: warning 备份
直接复制正在运行的 WAL 数据库可能得到不一致快照。最简单可靠的方式是先停止 Meerkit，再备份整个数据目录。
:::

## MySQL

先创建数据库和有建表权限的账户，再配置 DSN：

```yaml
storage:
  data_dir: /var/lib/meerkit
  database:
    type: mysql
    dsn: meerkit:secret@tcp(mysql:3306)/meerkit?tls=true
    auto_migrate: true
    max_open_conns: 25
    max_idle_conns: 10
    conn_max_lifetime: 30m
    conn_max_idle_time: 5m
```

驱动会启用 `parseTime`、UTC 和默认 `utf8mb4`。未指定连接池时，上述数值就是 MySQL 默认值。`max_open_conns` 至少为 2。

`auto_migrate: true` 在启动时持有 MySQL 命名锁并应用缺失迁移；设为 `false` 时仍会验证所有 Schema 版本，缺失就拒绝启动。

## 运行时配置

运行时配置保存在当前数据库的 `system_configs` 表中。修改后立即通知相关组件，并在重启后继续生效，不会被同名环境变量覆盖。

| 类型 | 默认值 |
| --- | --- |
| 记录保留 | `storage.retention = 30d` |
| 通知保留 | `storage.notification_retention = 30d` |
| 清理周期 | `storage.cleanup_interval = 1h` |
| 调度时区 | `scheduler.timezone = Local` |
| 最大并发 | `scheduler.max_concurrency = 16` |
| 扫描间隔 | `scheduler.poll_milliseconds = 500` |
| 宿主日志 | `info`、`simple`、带源码位置 |
| 插件日志 | `info`、`simple` |
| 会话 TTL | `auth.session_ttl = 720h` |

日志格式支持 `text`、`simple`、`json`，级别支持 `debug`、`info`、`warn`、`error`。持续时间使用 Go 格式，并额外支持 `30d` 这样的天数。

设置页可以按类型或全部恢复默认值。更新 API 使用版本号做乐观并发控制，多个页面同时编辑时旧版本会被拒绝。

## 静态与动态边界

需要重启：监听地址、数据库、数据目录、日志文件路径与轮转、开发插件目录、预置信任公钥。

可以热更新：保留周期、清理周期、调度参数、日志输出与格式、插件日志、会话 TTL。插件日志配置变化会重启当前活动插件进程。

`security.master_key_file` 当前只是已加载的启动配置字段；现有运行时代码没有使用它加密数据库内容。不要把该字段理解为已提供静态密钥或通知凭据加密。
