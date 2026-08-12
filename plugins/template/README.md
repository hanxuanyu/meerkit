# Meerkit Go 插件模板

[English](README.en.md) · [Go 插件指南](../../docs/development/plugin-go.md) · [协议规范](../../sdk/PROTOCOL.md)

此目录是最小可编译的监控插件源码模板。它提供一个 `example` 模块、源码清单和黑盒一致性测试套件。开发宿主和批量发布脚本都会跳过本目录。

## 创建插件

1. 复制目录，例如 `cp -R plugins/template plugins/dns`。
2. 修改 `go.mod` 的模块路径。
3. 更新 `meerkit-plugin.yaml` 的插件 ID、名称、发布者、版本和模块声明。
4. 在 `Descriptor` 中声明参数、结果集、字段类型和支持的条件操作符。
5. 实现 `ValidateConfig`、`Execute` 和需要的 `MigrateConfig`。
6. 为配置校验、成功与失败执行、超时和取消添加单元测试。
7. 更新 `conformance.json`，构建制品后运行黑盒检查。

复杂插件建议使用以下布局：

```text
my-plugin/
├── go.mod
├── go.sum
├── main.go
├── conformance.json
├── meerkit-plugin.yaml
├── README.md
└── monitor/
    ├── module.go
    └── module_test.go
```

`main.go` 只负责启动 Provider：

```go
func main() {
    sdk.Serve(sdk.NewProvider(monitor.New()))
}
```

一个 Provider 可以包含多个模块，但每个运行时描述器的类型和版本必须与清单完全一致。

## 描述器原则

- `Parameters` 驱动管理界面的动态表单，应声明类型、默认值、约束、选项和条件显示规则。
- `ResultSets` 驱动结果展示、条件编辑器和状态看板，应只声明字段实际支持的操作符。
- 结构化 JSON 字段可启用 `Path`；大文本或二进制结果应同时提供哈希、长度或摘要字段。
- `Parameters` 是动态表单的唯一字段来源，必须与 `ConfigSchema.properties` 逐项对应且必填声明一致；`ResultSets` 同理负责结果界面能力。
- `Execute` 必须尊重 `context.Context` 的截止时间和取消信号，并避免记录密钥、Token、完整请求头或敏感正文。

## 测试与运行

```bash
cd plugins/template
go test ./...
go build -o /tmp/meerkit-example-plugin .

cd ../..
go run ./cmd/plugincheck \
  --manifest ./plugins/template/meerkit-plugin.yaml \
  --artifact /tmp/meerkit-example-plugin \
  --suite ./plugins/template/conformance.json
```

将复制后的插件放在 `plugins.source_dir` 下，从仓库根目录运行 `go run . serve`，开发宿主会自动构建和加载它。

## 许可证

模板随 Meerkit 以 [Apache License 2.0](../../LICENSE) 开源。基于模板创建独立第三方插件时，请明确选择并随包提供自己的许可证。
