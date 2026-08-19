# Go 插件开发

官方插件使用 [`github.com/hanxuanyu/meerkit/sdk`](https://github.com/hanxuanyu/meerkit/tree/main/sdk)。从仓库模板创建插件，可以复用宿主握手、gRPC 适配和日志初始化。

## 从模板开始

```bash
cp -R plugins/template plugins/dns
```

随后修改：

- `go.mod`：独立模块路径和 SDK 依赖。
- `meerkit-plugin.yaml`：插件与模块身份，以及明确的进程启动方式、命令和参数。
- `main.go`：模块描述器与实现，复杂实现应拆到 `monitor/`。
- `conformance.json`：有效、无效、执行和迁移用例。
- README 与许可证。

开发宿主会跳过目录名 `template` 和模板 ID `example.monitor`。复制后必须更改 ID，才能被源码扫描加载。

## Module 接口

```go
type Module interface {
    Descriptor() ModuleDescriptor
    ValidateConfig(json.RawMessage) error
    Execute(context.Context, json.RawMessage) (Observation, error)
}
```

SDK 的默认 Provider 在配置版本相同的时候原样返回配置；如果要支持跨版本升级，可以实现完整 `Provider`，或在自己的聚合层分派迁移。

所有插件都使用统一的 `PluginRuntime`。HTTP、TCP 等不需要浏览器能力的插件也使用同一个入口，但不调用 `runtime.Browser()`：

```go
func main() {
    runtime := sdk.NewPluginRuntime()
    runtime.Serve(sdk.NewProvider(monitor.New()))
}
```

需要浏览器能力时，在构造模块前取得 `BrowserClient`：

```go
func main() {
    runtime := sdk.NewPluginRuntime()
    browser := runtime.Browser()

    runtime.Serve(sdk.NewProvider(monitor.NewModules(browser)...))
}
```

`BrowserClient` 提供目标查询、单 Action 和网络捕获。Session 未建立或已断开时会返回明确错误；调用方必须传播 Context、显式停止捕获，并关闭不需要持续复用的标签页。需要保留标签页时，应提供明确配置、限定识别范围，并处理标签页被用户关闭的情况。完整的执行流程、清理、复用、捕获和测试方式见[浏览器能力插件开发](/development/browser-plugin)，全部参数见[浏览器 Action 参考](/reference/browser-actions)。

## 模块描述器

`Descriptor` 同时驱动 UI、条件编辑器和结果解释：

- `Type`、`Version`、`ConfigVersion`、`ResultSchemaVersion` 与清单一致。
- `Name`、`Description` 用于管理界面。
- `ListSummary` 选择列表中用于辨识监控的配置字段。
- `Parameters` 声明输入字段和交互约束。
- `ResultSets` 声明结构化输出及条件操作符。
- `ConfigSchema`、`ResultSchema` 提供机器可读结构约束。

参数类型包括 `string`、`text`、`list`、`map`、`boolean`、`integer`、`number`、`url`、`json` 和 `duration`。可以声明必填、默认值、范围、步长、固定选项、条件选项、显示/启用条件、敏感标记、格式和单位。

::: warning 参数描述器必须完整
动态表单只消费 `Parameters`，不会从 `ConfigSchema` 推导控件。`ConfigSchema.properties` 与 `Parameters` 必须逐项对应，必填声明也必须一致；插件返回不一致的描述器时，SDK 会拒绝 `ListModules`。
:::

## 配置校验

`ValidateConfig` 应在网络或文件操作前检查所有可以静态确定的问题，并返回给用户可理解的错误。至少验证：

- 必填值、范围和枚举。
- URL、地址、正则和 JSON 格式。
- 互相依赖字段是否一致。
- 大小、超时和资源边界。

敏感字段可以标记 `Secret` 以改善界面输入，但插件仍必须主动避免日志泄漏。

## 执行与结果

`Execute` 必须尊重 Context 的取消与截止时间，并为外部操作设置有限超时。返回的 `Observation` 包含：

- `Success`：模块执行是否完成。
- `SchemaVersion`：与清单结果版本一致。
- `ResultSets`：按描述器键组织的结构化结果。
- `Result`：可选兼容平面结果。
- `Summary`：短且适合日志、列表和通知的摘要。
- `ErrorCode`、`ErrorMessage`：失败时的稳定代码和可读详情。

大正文和二进制应限制大小，并额外返回哈希、长度或截断标记。结果必须能被标准 JSON 编码，不能包含 NaN、Inf、通道或语言对象。

## 配置迁移

当已保存配置的结构发生变化：

1. 提升清单和描述器的 `config_version`。
2. 实现从每个仍支持旧版本到新版本的确定性迁移。
3. 对迁移前后数据运行 `ValidateConfig`。
4. 为已有监控添加升级测试。

不要仅通过插件包版本触发迁移。宿主比较的是模块配置版本。

## 日志

`PluginRuntime.Serve` 设置默认 `log/slog` Logger，并把插件 ID 与版本加入属性。宿主通过环境变量控制日志级别和格式。

```go
slog.InfoContext(ctx, "dns query completed",
    "server", server,
    "record_type", recordType,
    "duration_ms", elapsed.Milliseconds(),
)
```

业务日志写 stderr。stdout 第一行属于 go-plugin 握手，直接写 stdout 可能破坏启动。不要记录密码、Token、完整头、私有响应正文或完整模块配置。

## 本地验证

```bash
cd plugins/dns
go test ./...
go build -o /tmp/meerkit-dns-plugin .

cd ../..
go run ./cmd/plugincheck \
  --manifest ./plugins/dns/meerkit-plugin.yaml \
  --artifact /tmp/meerkit-dns-plugin \
  --suite ./plugins/dns/conformance.json
```

最后从仓库根目录运行 `go run . serve`，在实际管理界面验证表单、测试调用、执行结果、条件和状态看板。
