# 命令行参考

宿主使用 Cobra 命令树。直接运行 `meerkit` 与 `meerkit serve` 等价。

## 全局参数

这些参数可以放在子命令前，并覆盖配置文件和环境变量。

| 参数 | 说明 |
| --- | --- |
| `--config <path>` | 配置文件路径 |
| `--data-dir <path>` | 运行数据目录和默认 SQLite 位置 |
| `--database-type <sqlite|mysql>` | 数据库类型 |
| `--database-dsn <dsn>` | 数据库 DSN |
| `--database-auto-migrate=<bool>` | 是否自动应用数据库迁移，默认 `true` |

## serve

```bash
meerkit serve [flags]
meerkit [flags]
```

| 参数 | 说明 |
| --- | --- |
| `--listen <host:port>` | 同时覆盖 `server.address` 和 `server.port` |
| `--log-dir <path>` | 日志文件目录 |
| `--log-filename <name>` | 主应用日志文件名，只允许文件名 |
| `--access-log-filename <name>` | HTTP 访问日志文件名，只允许文件名 |

示例：

```bash
./meerkit \
  --config /etc/meerkit/config.yaml \
  --data-dir /var/lib/meerkit \
  serve --listen 127.0.0.1:8080
```

未指定配置文件且默认 `config.yaml` 不存在时，`serve` 会生成默认配置。

## admin reset-key

```bash
meerkit [database flags] admin reset-key --key <new-key>
```

新密钥至少 12 个字符。命令更新当前数据库中的管理员哈希并撤销所有会话。它不会启动 HTTP 服务。

## plugin import

```bash
meerkit [database flags] plugin import <archive> [flags]
```

| 参数 | 说明 |
| --- | --- |
| `--enable` | 导入成功后立即启用 |
| `--confirm-unverified` | 允许启用未签名插件；不允许绕过签名不可信状态 |

输出为安装记录 JSON。支持 `.zip` 和 `.tar.gz`。

```bash
meerkit --data-dir ./data plugin import \
  ./dist/plugins/example.monitor-0.1.0-linux-amd64.tar.gz \
  --enable --confirm-unverified
```

## plugin scan

```bash
meerkit [database flags] plugin scan
```

扫描 `${storage.data_dir}/plugins/inbox`，导入支持的包，并输出导入数量。失败包会移到插件数据根的 `rejected/`。

## version

```bash
meerkit version
```

源码直接构建且未注入 ldflags 时输出 `dev`。发布脚本通过 `-X main.version=<version>` 注入版本。

## plugincheck

```bash
meerkit-plugincheck \
  --manifest <meerkit-plugin.yaml> \
  --artifact <file> \
  [--suite <conformance.json>] \
  [--timeout 10s]
```

`--manifest` 与 `--artifact` 必填。检查失败退出码为 1；成功逐项输出 `PASS`。

## pluginpack

源码工具通过 `go run ./cmd/pluginpack` 使用：

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--plugin <dir>` | 必填 | Go 插件源码目录 |
| `--output <dir>` | `dist/plugins` | 输出目录 |
| `--targets <list>` | `current` | 逗号分隔 `GOOS/GOARCH` |
| `--combined` | `false` | 把所有目标放入一个 ZIP |
| `--sign-key <file>` | 空 | Base64 Ed25519 私钥 |
| `--key-id <id>` | 空 | 与私钥同时提供的发布标签 |
| `--generate-key <prefix>` | 空 | 生成密钥后退出 |

该工具固定使用 `go build`，只适用于 Go 插件。通常优先使用根 Make 目标和 `scripts/` 包装脚本。

## 退出码

宿主命令成功返回 0，运行错误返回 1，参数、未知命令和用法错误返回 2。
