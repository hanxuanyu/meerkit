# Meerkit

Meerkit 是一套支持独立监控插件的监控服务，并内置 Webhook、SMTP 和站内通知能力。HTTP 与 TCP 以官方插件形式发布，不再链接到宿主进程中。

## 运行

```bash
npm --prefix web run build
go run . serve --config config.yaml
```

默认管理界面地址为 `http://127.0.0.1:8080`。首次访问时需要设置管理员访问密钥，后续所有管理 API 和通知流都要求有效会话。本地重置命令：

```bash
meerkit --data-dir ./data admin reset-key --key '新的管理员访问密钥'
```

配置优先级依次为内置默认值、`config.yaml`、`MEERKIT_*` 环境变量和命令行参数。嵌套环境变量使用双下划线：

```bash
MEERKIT_SERVER__PORT=9090 MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit go run .
```

## 插件

插件包可通过管理页面上传、`meerkit plugin import` 导入，或复制到 `${data_dir}/plugins/inbox` 自动发现。仅支持 `.zip` 和 `.tar.gz`，不会发现或执行裸二进制文件。

### 生成官方签名密钥

官方插件使用 Ed25519 密钥签名。以下命令必须在仓库根目录执行，参数是输出文件的路径前缀：

```bash
scripts/package-plugins.sh --generate-key ./keys/meerkit-official
```

命令会创建两个 Base64 编码的文件，并拒绝覆盖已有密钥：

- `keys/meerkit-official.private.key`：签名私钥，只用于发布环境或 CI 密钥存储。
- `keys/meerkit-official.public.key`：公钥，可用于备份、公布指纹或配置可选的预置信任。

私钥不得提交到仓库、复制到 `dist/`，也不得包含在发布包中。签名包的 `meerkit-plugin.sig` 会自动携带由私钥推导出的公钥，因此打包时不需要再传入公钥文件。`key ID` 是便于识别发布批次的稳定名称，例如 `meerkit-official-2026`；实际信任身份由公钥的 SHA-256 指纹确定。

### 打包官方插件

设置私钥路径和 key ID 后，不带 `--plugin` 调用批量脚本。脚本会打包 `plugins/` 下所有包含清单的正式插件，目前包括 HTTP 和 TCP，并自动排除源码模板 `plugins/template`：

```bash
MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-official.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-official-2026 \
scripts/package-plugins.sh dist/plugins linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

每个平台生成独立插件包，Windows 使用 `.zip`，其他平台使用 `.tar.gz`。签名覆盖插件清单及其中记录的制品哈希，同时覆盖包内的 README 和 LICENSE。仅打包单个官方插件时，可直接传递签名参数：

```bash
scripts/package-plugins.sh \
  --plugin ./plugins/http \
  --targets linux/amd64 \
  --sign-key ./keys/meerkit-official.private.key \
  --key-id meerkit-official-2026
```

### 生成完整官方发布包

`scripts/package.sh` 会构建前端、宿主程序和目标平台的全部官方插件。签名环境变量会传递给内部插件打包步骤，因此同一发布包内的所有官方插件会使用同一密钥：

```bash
MEERKIT_VERSION=v0.1.0 \
MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-official.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-official-2026 \
scripts/package.sh dist/releases linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

生成的发布压缩包中，官方插件位于宿主可执行文件同级的 `plugins/` 目录。使用空数据目录首次启动时，宿主会扫描该目录、验证签名、启用插件，并将其公钥指纹登记为官方发布者；之后手工导入由同一密钥签名的新版本或其他插件时会自动验证。官方状态来自这次随发行包进行的首次引导，不需要把公钥另外写入 `plugins.trusted_keys`。插件清单中不存在可自行声明的“官方”字段，因此将单独生成的签名包导入尚未建立官方信任的环境时，仍需要像第三方签名包一样核对并确认公钥指纹。

### 开发模式直接运行插件源码

宿主未通过 `-ldflags "-X main.version=..."` 指定版本时默认为 `dev`，因此在仓库根目录执行 `go run .` 会使用 `plugins.mode: auto` 自动发现 `plugins/*/meerkit-plugin.yaml`（跳过 `plugins/template`），在每次启动时重新构建并运行源码插件。修改 HTTP、TCP 等内置插件后只需重新执行 `go run .`，无需先生成插件包、导入或升级；即使清单版本未变化，开发二进制也会被当前源码覆盖。

```yaml
plugins:
  mode: auto
  source_dir: ./plugins
  trusted_keys: {}
```

`mode` 可设为 `source` 以强制使用源码，或设为 `package` 以便在开发版本中验证发行包加载流程，也可分别使用 `MEERKIT_PLUGINS__MODE` 和 `MEERKIT_PLUGINS__SOURCE_DIR` 覆盖。开发构建的暂存文件和最终二进制只写入 `${storage.data_dir}/plugins`（默认即仓库根目录的 `data/plugins`），不会在插件源码目录中生成 `data` 或制品目录。源码插件在管理页标记为“开发源码”，没有可导出的签名包；切换到 `package` 或使用带正式版本号的发行程序后，宿主会清理开发安装记录并恢复使用可执行文件同级的插件包。

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
  store/         SQLite 持久化
plugins/          官方插件、模板和打包工具
sdk/              公共监控插件 SDK 与 gRPC 协议
scripts/          项目与插件发布脚本
web/              React/Vite 前端与通用 UI 组件
```
