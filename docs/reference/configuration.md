# 配置字段参考

## 启动字段

| YAML 路径 | 默认值 | 环境变量 | CLI |
| --- | --- | --- | --- |
| `server.address` | `0.0.0.0` | `MEERKIT_SERVER__ADDRESS` | `--listen` |
| `server.port` | `8080` | `MEERKIT_SERVER__PORT` | `--listen` |
| `storage.data_dir` | `./data` | `MEERKIT_STORAGE__DATA_DIR` | `--data-dir` |
| `storage.database.type` | `sqlite` | `MEERKIT_STORAGE__DATABASE__TYPE` | `--database-type` |
| `storage.database.dsn` | 空 | `MEERKIT_STORAGE__DATABASE__DSN` | `--database-dsn` |
| `storage.database.auto_migrate` | `true` | `MEERKIT_STORAGE__DATABASE__AUTO_MIGRATE` | `--database-auto-migrate` |
| `storage.database.max_open_conns` | `0` | `MEERKIT_STORAGE__DATABASE__MAX_OPEN_CONNS` | 无 |
| `storage.database.max_idle_conns` | `0` | `MEERKIT_STORAGE__DATABASE__MAX_IDLE_CONNS` | 无 |
| `storage.database.conn_max_lifetime` | 空 | `MEERKIT_STORAGE__DATABASE__CONN_MAX_LIFETIME` | 无 |
| `storage.database.conn_max_idle_time` | 空 | `MEERKIT_STORAGE__DATABASE__CONN_MAX_IDLE_TIME` | 无 |
| `logging.file.directory` | `./logs` | `MEERKIT_LOGGING__FILE__DIRECTORY` | `--log-dir` |
| `logging.file.filename` | `meerkit.log` | `MEERKIT_LOGGING__FILE__FILENAME` | `--log-filename` |
| `logging.file.max_size_mb` | `100` | `MEERKIT_LOGGING__FILE__MAX_SIZE_MB` | 无 |
| `logging.file.max_backups` | `7` | `MEERKIT_LOGGING__FILE__MAX_BACKUPS` | 无 |
| `logging.file.max_age_days` | `30` | `MEERKIT_LOGGING__FILE__MAX_AGE_DAYS` | 无 |
| `logging.file.compress` | `true` | `MEERKIT_LOGGING__FILE__COMPRESS` | 无 |
| `logging.file.access.filename` | `meerkit-access.log` | `MEERKIT_LOGGING__FILE__ACCESS__FILENAME` | `--access-log-filename` |
| `security.master_key_file` | `./data/master.key` | `MEERKIT_SECURITY__MASTER_KEY_FILE` | 无 |
| `security.allow_token_copy` | `false` | `MEERKIT_SECURITY__ALLOW_TOKEN_COPY` | 无 |
| `plugins.source_dir` | `./plugins` | `MEERKIT_PLUGINS__SOURCE_DIR` | 无 |
| `plugins.trusted_keys` | `{}` | 无稳定映射 | 无 |

`MEERKIT_CONFIG_FILE` 指定整个配置文件路径，不对应 YAML 字段。

### 校验规则

- 端口为 1 到 65535。
- 数据目录、日志目录和插件源码目录不能为空。
- 数据库类型只能是 `sqlite` 或 `mysql`；MySQL DSN 必填。
- 连接数量不能为负，空闲连接不能超过打开连接；MySQL 打开连接至少 2。
- 连接生命周期使用非负 Go duration。
- 日志文件名不能带目录，单文件大小必须为正，备份数量和天数不能为负。
- `trusted_keys` 的每个值必须是 Base64 Ed25519 公钥。
- MCP 由数据库动态配置 `mcp.enabled` 控制，默认关闭；开启时访问需要有效的数据库 MCP token。

数据库字段为 `0` 或空时，Store 会应用后端默认：SQLite 4/4 个连接；MySQL 25/10、30 分钟生命周期、5 分钟空闲时间。

`security.master_key_file` 用于加密保存 API token；`security.allow_token_copy` 开启后允许管理员重复查看 token 明文。

## 运行时字段

| 类型 | 路径 | 默认值 | 约束 |
| --- | --- | --- | --- |
| storage | `storage.retention` | `30d` | 正 duration，支持 `d` |
| storage | `storage.notification_retention` | `30d` | 正 duration |
| storage | `storage.cleanup_interval` | `1h` | 正 duration |
| scheduler | `scheduler.timezone` | `Local` | 系统可加载的 IANA 时区或 `Local` |
| scheduler | `scheduler.max_concurrency` | `16` | 正整数 |
| scheduler | `scheduler.poll_milliseconds` | `500` | 至少 100 |
| logging | `logging.level` | `info` | `debug`、`info`、`warn`、`error` |
| logging | `logging.format` | `simple` | `text`、`simple`、`json` |
| logging | `logging.add_source` | `true` | boolean |
| logging | `logging.console.enabled` | `true` | 至少控制台或文件之一启用 |
| logging | `logging.console.access` | `false` | boolean |
| logging | `logging.file.enabled` | `true` | boolean |
| logging | `logging.file.access.enabled` | `true` | boolean |
| plugins | `plugins.log_level` | `info` | 同宿主日志级别 |
| plugins | `plugins.log_format` | `simple` | 同宿主日志格式 |
| auth | `auth.session_ttl` | `720h` | 正 duration，支持 `d` |

运行时配置没有环境变量或 CLI 覆盖。使用设置页或 `/api/v1/system/config/runtime/:type` 修改，并通过数据库持久化。

## 配置来源值

`GET /api/v1/system/config` 对启动字段返回 `source`：

- `default`
- `config_file`
- `environment`
- `command_line`

运行时字段返回独立的 `version` 和 `is_default`。数据库 DSN 的元数据只返回空字符串或 `configured`。
