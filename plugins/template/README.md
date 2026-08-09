# Meerkit 监控插件模板

这是一个最小但可运行的监控插件骨架，用于创建独立 Go 模块并通过 Meerkit SDK 暴露监控能力。`template` 目录不会被开发宿主自动加载，也不会被批量打包脚本发布。

## 创建插件

1. 复制整个目录，例如复制为 `plugins/dns`。
2. 修改 `go.mod` 中的模块路径，以及 `main.go` 中的包名和模块实现。
3. 修改 `meerkit-plugin.yaml` 中的插件身份、协议和模块版本。
4. 在 `Descriptor` 中声明配置参数、结果集和可用比较操作。
5. 实现配置校验、执行逻辑和必要的配置迁移。
6. 增加契约测试，然后使用开发宿主验证界面和执行行为。

## 目录结构

```text
my-plugin/
├── go.mod
├── go.sum
├── main.go
├── conformance.json
├── meerkit-plugin.yaml
├── README.md
├── README.en.md
└── monitor/
    ├── module.go
    └── module_test.go
```

复杂插件建议把模块实现放入独立 `monitor` 包，让 `main.go` 只负责创建模块并调用 `sdk.Serve`：

```go
func main() {
    sdk.Serve(sdk.NewProvider(monitor.New()))
}
```

一个插件可以向 `sdk.NewProvider` 传入多个模块，但清单中的模块声明必须与运行时 `Descriptor` 完全一致。

## 清单

源码清单必须符合 `plugins/manifest.schema.json`。关键字段包括：

- `id`：稳定且全局唯一的插件 ID，建议使用反向域名或组织前缀。
- `version`：插件包的语义化版本。相同 ID 和版本不能导入不同的发行内容。
- `vendor`：发布者名称。
- `desp`：插件管理页面使用的功能摘要。
- `url`：源码仓库或可信发布页面。
- `protocol.min/max`：支持的 Meerkit 插件协议范围。
- `modules`：插件提供的模块类型及版本。
- `artifacts`：源码中保持空数组，由打包工具写入平台、路径、大小和 SHA-256。

打包后的 artifact 可以带可选 `runtime`。官方 Go 插件省略该字段并直接执行；第三方单文件脚本、zipapp 或 JAR 可以使用 `interpreter` 模式。该配置按平台附着在 artifact 上，具体约束见 `sdk/PROTOCOL.md`。

`version`、模块 `version`、`config_version` 和 `result_schema_version` 含义不同：

- 插件 `version` 表示整个发行包版本。
- 模块 `version` 表示模块实现的兼容版本。
- `config_version` 变化时，宿主会调用配置迁移。
- `result_schema_version` 表示持久化执行结果的结构版本。

## 模块描述

`Descriptor` 是插件能力的主要契约，也会驱动监控项编辑器和插件详情弹窗。

- `Type` 和 `Version` 必须匹配清单声明。
- `Name` 和 `Description` 用于界面展示。
- `ListSummary` 指定监控列表标题下方显示哪些配置值，例如 HTTP 使用 URL、TCP 使用 `host:port`。
- `Parameters` 声明输入字段的类型、顺序、默认值、范围、选项以及显隐条件。
- `ResultSets` 将输出字段按业务含义分组，并声明类型、格式、单位、路径能力和比较操作。
- `ConfigSchema` 和 `ResultSchema` 提供机器可读的结构约束。

不要只填写 JSON Schema。Meerkit 的动态表单和紧凑能力展示主要依赖 `Parameters` 与 `ResultSets`。

## 输入参数

参数类型包括字符串、多行文本、列表、Map、布尔值、整数、数值、URL、JSON 和时长。常用约束包括：

- `Required`：必填。
- `Default`：默认值。
- `Minimum`、`Maximum` 和 `Step`：数值范围。
- `Options`：固定选项。
- `OptionsWhen`：根据其他参数动态切换选项。
- `VisibleWhen` 和 `EnabledWhen`：控制字段显示或启用状态。
- `Secret`：将参数标记为敏感信息；插件日志仍应主动避免输出其值。
- `FullWidth`、`Rows`、`Placeholder`、`Format` 和 `Unit`：控制编辑体验。

## 返回结果

`Execute` 返回 `sdk.Observation`：

- `Success` 表示模块执行是否成功。
- `SchemaVersion` 应与清单的 `result_schema_version` 一致。
- `ResultSets` 保存按描述器分组的结构化结果。
- `Result` 可保留兼容的扁平结果。
- `Summary` 提供适合直接阅读的简短执行摘要，不应复制大段正文或敏感数据。
- `ErrorCode` 和 `ErrorMessage` 可表达稳定的机器错误和用户可读信息。

结果字段应声明真正支持的比较操作。只有 JSON 等结构化字段需要启用 `Path`。长文本、二进制内容和大型对象应提供哈希、长度或摘要字段，避免条件和通知携带过多内容。

## 校验、迁移和执行

- `ValidateConfig` 必须在网络或文件操作之前拒绝无效配置，并返回可理解的错误。
- `Execute` 必须遵守传入的 `context.Context`，为外部操作设置合理超时，并在失败时返回有意义的 Observation。
- 修改配置结构时提升 `config_version` 并实现 `MigrateConfig`；不能安全迁移时应返回错误，而不是静默丢弃字段。
- 不要依赖插件进程内的长期可变状态。插件可能因启用、升级、健康检查失败或宿主退出而重启。

## 日志

SDK 自动记录插件进程启动、模块发现、健康检查、配置校验、执行和迁移生命周期。模块可以使用标准 `log/slog` 添加业务日志：

```go
slog.InfoContext(ctx, "dns query completed",
    "server", server,
    "record_type", recordType,
    "duration_ms", duration,
)
```

只记录排障所需的元数据。不要输出密码、Token、请求头、完整请求/响应正文或其他敏感配置。插件日志格式和级别由宿主的 `plugins.log_format` 与 `plugins.log_level` 控制。

## 本地开发

在仓库根目录执行：

```sh
go run .
```

`dev` 宿主会扫描 `plugins.source_dir`，跳过目录名为 `template` 或插件 ID 为 `example.monitor` 的模板，并构建其他源码插件。构建暂存目录、二进制和日志均位于 `${storage.data_dir}/plugins`。

插件是独立 Go 模块，应在插件目录单独测试：

```sh
cd plugins/my-plugin
go test ./...
```

建议至少覆盖描述器声明、合法和非法配置、成功执行、超时与取消、外部错误、结果可序列化以及日志不泄漏敏感数据。

构建出可执行制品后，使用模板自带的 `conformance.json` 结构进行线协议黑盒测试：

```sh
go run ./cmd/plugincheck \
  --manifest ./plugins/my-plugin/meerkit-plugin.yaml \
  --artifact ./build/plugin \
  --suite ./plugins/my-plugin/conformance.json
```

## 打包发布

生成当前平台的未签名开发包：

```sh
scripts/package-plugins.sh --plugin ./plugins/my-plugin
```

生成签名包：

```sh
scripts/package-plugins.sh \
  --plugin ./plugins/my-plugin \
  --sign-key ./keys/vendor.private.key \
  --key-id vendor-release-2026
```

Meerkit 只导入 `.zip` 和 `.tar.gz` 插件包，不会加载手工复制的裸可执行文件。发布密钥生成、跨平台打包和信任模型参见上级目录的 `plugins/README.md`。
