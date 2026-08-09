# Meerkit

Meerkit 是一套支持独立监控插件的监控服务，并内置 Webhook、SMTP 和站内通知能力。HTTP 与 TCP 以官方插件形式发布，不再链接到宿主进程中。

## 开发

本地开发需要 Go、Node.js、npm 和 Make。首次拉取代码后安装依赖：

```bash
make deps
```

同时启动 Go 后端和 Vite 前端：

```bash
make dev
```

前端开发服务器地址为 `http://127.0.0.1:5173`，并将 API 和 WebSocket 请求代理到 `http://127.0.0.1:8080`。按 `Ctrl+C` 会同时停止两个进程。也可以分别启动，并通过参数变量透传额外选项：

```bash
make dev-frontend FRONTEND_ARGS="--host 0.0.0.0"
make dev-backend BACKEND_ARGS="--config config.yaml --listen 0.0.0.0:8080"
```

后端需要嵌入已构建的前端文件；`make dev` 和 `make dev-backend` 会在 `web/dist` 不存在时先构建一次。单独生成生产前端资源可执行 `make frontend-build`。使用 `make help` 可查看所有常用目标和可覆盖变量。

### 清理与重置

`make clean` 仅清理可重新生成的构建产物，包括 `dist/`、`web/dist/`、`.gocache/` 和根目录下的 `meerkit`/`meerkit.exe`。需要从空白运行状态重新启动项目时执行：

```bash
make reset
```

`make reset` 会先执行 `clean`，再删除默认的 `data/`、`logs/`、`config.yaml`，以及根目录下的 `*.db`、`*.db-shm` 和 `*.db-wal` SQLite 文件。该操作会永久删除本地运行数据和配置；`keys/`、`web/node_modules/` 及其他已安装依赖不会被删除。

## 运行

```bash
make frontend-build
make dev-backend BACKEND_ARGS="--config config.yaml"
```

默认管理界面地址为 `http://127.0.0.1:8080`。首次访问时需要设置管理员访问密钥，后续所有管理 API 和通知流都要求有效会话。本地重置命令：

```bash
meerkit --data-dir ./data admin reset-key --key '新的管理员访问密钥'
```

`config.yaml`、`MEERKIT_*` 环境变量和命令行参数只用于启动配置：监听地址和端口、数据目录、日志文件目录/文件名/轮转参数、主密钥文件、插件源码目录和可信签名公钥。优先级依次为内置默认值、配置文件、环境变量和命令行参数，嵌套环境变量使用双下划线：

```bash
MEERKIT_SERVER__PORT=9090 MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit go run .
```

数据库默认为 SQLite，`dsn` 留空时使用 `${storage.data_dir}/meerkit.db`。MySQL 需要预先创建数据库，Meerkit 负责自动创建和升级表、索引及内置数据：

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

也可以使用 `MEERKIT_STORAGE__DATABASE__TYPE`、`MEERKIT_STORAGE__DATABASE__DSN` 或对应命令行参数。DSN 可能包含凭据，不会出现在配置元数据和日志中。关闭 `auto_migrate` 后，启动时仍会校验数据库版本并在结构缺失时拒绝运行。

保留周期、清理周期、调度器参数、会话 TTL、宿主和插件日志级别/格式以及日志输出开关属于运行时配置，保存于当前数据库的 `system_configs` 表。它们只能通过设置页或运行时配置 API（`GET /api/v1/system/config`、`PATCH /api/v1/system/config/runtime/:type`）修改，Docker 重启时不会被环境变量覆盖；数据库中的值会在重启后继续生效。设置页可以按类型或全部恢复代码默认值。管理员密钥也保存在 `auth` 配置行中，但只通过初始化和 `admin reset-key --key` 流程管理，不会显示或参与恢复默认值。

## 插件

插件包可通过管理页面上传、`meerkit plugin import` 导入，或复制到 `${data_dir}/plugins/inbox` 自动发现。仅支持 `.zip` 和 `.tar.gz`，不会发现或执行裸二进制文件。

官方插件使用 Go SDK 开发和打包；第三方插件可以使用其他语言实现稳定的 gRPC + JSON 线协议。规范 proto、JSON Schema、制品运行配置和通用一致性测试工具见 [`sdk/PROTOCOL.md`](sdk/PROTOCOL.md)。

### 生成官方签名密钥

官方插件使用 Ed25519 密钥签名。以下命令必须在仓库根目录执行，参数是输出文件的路径前缀：

```bash
make generate-key KEY_PREFIX=./keys/meerkit-official
```

该目标调用现有的 `scripts/package-plugins.sh --generate-key` 流程。

命令会创建两个 Base64 编码的文件，并拒绝覆盖已有密钥：

- `keys/meerkit-official.private.key`：签名私钥，只用于发布环境或 CI 密钥存储。
- `keys/meerkit-official.public.key`：公钥，可用于备份、公布指纹或配置可选的预置信任。

私钥不得提交到仓库、复制到 `dist/`，也不得包含在发布包中。签名包的 `meerkit-plugin.sig` 会自动携带由私钥推导出的公钥，因此打包时不需要再传入公钥文件。`key ID` 是便于识别发布批次的稳定名称，例如 `meerkit-official-2026`；实际信任身份由公钥的 SHA-256 指纹确定。

### 打包官方插件

设置私钥路径和 key ID 后执行批量目标。它会打包 `plugins/` 下所有包含清单的正式插件，目前包括 HTTP 和 TCP，并自动排除源码模板 `plugins/template`：

```bash
make package-plugins \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026 \
  TARGETS=linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

每个平台生成独立插件包，Windows 使用 `.zip`，其他平台使用 `.tar.gz`。签名覆盖插件清单及其中记录的制品哈希，同时覆盖包内的 README 和 LICENSE。仅打包单个官方插件时，可直接传递签名参数：

```bash
make package-plugin \
  PLUGIN=./plugins/http \
  TARGETS=linux/amd64 \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026
```

### 生成完整官方发布包

`scripts/package.sh` 会构建前端、宿主程序和目标平台的全部官方插件。签名环境变量会传递给内部插件打包步骤，因此同一发布包内的所有官方插件会使用同一密钥：

```bash
make package-release \
  VERSION=v0.1.0 \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026 \
  TARGETS=linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

默认输出目录分别为 `dist/plugins` 和 `dist/releases`，可通过 `PLUGIN_OUTPUT` 和 `RELEASE_OUTPUT` 覆盖。`TARGETS` 默认为当前平台；不设置 `SIGN_KEY` 和 `KEY_ID` 时会生成未签名包，这两个变量用于签名时必须同时设置。Make 目标仍由 `scripts/package-plugins.sh` 和 `scripts/package.sh` 执行实际打包，因此也可以直接调用脚本处理更细粒度的参数。

生成的发布压缩包中，官方插件位于宿主可执行文件同级的 `plugins/` 目录。使用空数据目录首次启动时，宿主会扫描该目录、验证签名、启用插件，并将其公钥指纹登记为官方发布者；之后手工导入由同一密钥签名的新版本或其他插件时会自动验证。官方状态来自这次随发行包进行的首次引导，不需要把公钥另外写入 `plugins.trusted_keys`。插件清单中不存在可自行声明的“官方”字段，因此将单独生成的签名包导入尚未建立官方信任的环境时，仍需要像第三方签名包一样核对并确认公钥指纹。

发布压缩包还包含独立的 `meerkit-plugincheck`（Windows 为 `.exe`），用于对已经构建好的第三方插件制品执行协议一致性检查；它不负责构建或打包第三方语言源码。

### 开发模式直接运行插件源码

宿主未通过 `-ldflags "-X main.version=..."` 指定版本时默认为 `dev`，因此在仓库根目录执行 `go run .` 会自动发现 `plugins/*/meerkit-plugin.yaml`（跳过 `plugins/template`），在每次启动时重新构建并运行源码插件。修改 HTTP、TCP 等内置插件后只需重新执行 `go run .`，无需先生成插件包、导入或升级；即使清单版本未变化，开发二进制也会被当前源码覆盖。

```yaml
plugins:
  source_dir: ./plugins
  trusted_keys: {}
```

插件运行模式由宿主版本自动决定，不再需要 `plugins.mode`：`dev` 版本优先构建源码插件，未发现源码时回退发行包；带正式版本号的宿主只加载发行包。`MEERKIT_PLUGINS__SOURCE_DIR` 可覆盖源码目录。开发构建的暂存文件和最终二进制只写入 `${storage.data_dir}/plugins`（默认即仓库根目录的 `data/plugins`），不会在插件源码目录中生成 `data` 或制品目录。源码插件在管理页标记为“开发源码”，没有可导出的签名包；正式版本启动后会清理开发安装记录并恢复使用可执行文件同级的插件包。

宿主和插件日志级别、格式以及输出开关在设置页的运行时配置中调整，支持 `text`、`simple` 和 `json`，默认使用 `simple`。插件日志输出形如 `[09:08:07] [INFO] plugin activated plugin_id=meerkit.http`，修改后会应用到后续启动的插件，必要时宿主会重启当前插件进程。

Windows PowerShell 使用等效脚本：

```powershell
.\scripts\package-plugins.ps1 -GenerateKey .\keys\meerkit-official
$env:MEERKIT_PLUGIN_SIGN_KEY = ".\keys\meerkit-official.private.key"
$env:MEERKIT_PLUGIN_KEY_ID = "meerkit-official-2026"
.\scripts\package.ps1 -Output dist/releases -Targets "windows/amd64" -Version "v0.1.0"
```

正式发布应长期保管并备份私钥。更换密钥会产生新的公钥指纹，已有部署不会将其视为原发布者，需要通过新版本发布流程完成密钥轮换并重新建立信任。更多插件开发、第三方签名和信任模型说明参见 [`plugins/README.md`](plugins/README.md)，脚本参数说明参见 [`scripts/README.md`](scripts/README.md)。

## 目录

```text
Makefile          开发、打包和密钥生成快捷入口
main.go
internal/
  api/           HTTP API 与嵌入式前端处理
  app/           外部配置加载
  application/   服务依赖装配与生命周期
  auth/          管理员凭据与会话
  command/       Cobra 命令树与命令实现
  core/          监控、条件、结果和通知契约
  monitor/       带所有者的模块注册表与远程适配器
  plugin/        插件包校验、安装和进程管理
  runtime/       执行器与 Cron 调度器
  runtimeconfig/ 运行时配置默认值、校验和热应用
  store/         Bun repository、SQLite/MySQL 方言与版本迁移
plugins/          插件协议定义、官方插件和示例插件
cmd/pluginpack/   插件打包与签名工具
sdk/              公共监控插件 SDK 与 gRPC 协议
scripts/          项目与插件发布脚本
web/              React/Vite 前端与通用 UI 组件
```
