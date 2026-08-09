# 插件一致性测试

`meerkit-plugincheck` 是面向所有语言的黑盒工具。它启动真实制品并检查宿主真正依赖的协议行为，不要求插件导入 Go SDK。

## 基础检查

```bash
go run ./cmd/plugincheck \
  --manifest ./meerkit-plugin.yaml \
  --artifact ./build/plugin
```

基础检查包括：

1. 清单与当前协议兼容。
2. 制品是普通文件，启动方式有效。
3. go-plugin 握手与 gRPC 协商成功。
4. 标准 gRPC Health 服务 `plugin` 为 `SERVING`。
5. Meerkit 应用 Health 成功。
6. `ListModules` 响应通过 JSON Schema。
7. 运行时模块与清单数量、类型和版本一致。

每项成功会输出 `PASS`，任何失败返回非零退出码并保留插件 stderr。

## 测试套件

通过 `--suite` 增加配置、执行和迁移用例：

```json
{
  "$schema": "./sdk/schema/conformance-suite.schema.json",
  "cases": [
    {
      "name": "valid request succeeds",
      "module_type": "example",
      "config": {},
      "execute": { "success": true },
      "migration": {
        "from_version": "1",
        "to_version": "1"
      }
    },
    {
      "name": "invalid request is rejected",
      "module_type": "example",
      "config": { "timeout": -1 },
      "expect_validation_error": true
    }
  ]
}
```

```bash
go run ./cmd/plugincheck \
  --manifest ./meerkit-plugin.yaml \
  --artifact ./build/plugin \
  --suite ./conformance.json \
  --timeout 15s
```

工具会验证套件 Schema、请求与响应 Schema、业务错误预期、Observation Schema 和结果版本。成功迁移必须返回存在的 `config` 字段，即使值是 `null`。

## 应覆盖的用例

- 最小合法配置和完整合法配置。
- 每个必填字段缺失、类型错误、越界与冲突字段。
- 正常外部响应、协议错误、超时、取消和大小上限。
- 结果包含所有描述字段，且可以标准 JSON 序列化。
- 敏感配置不会进入摘要、错误和 stderr。
- 支持的每条配置迁移路径，以及明确拒绝未知路径。
- 解释器缺失或版本不兼容时给出可诊断启动错误。

黑盒套件不会替代语言内单元测试，也不会检查插件是否恶意、是否泄漏网络数据或所有业务语义是否正确。

## CI 组合

推荐第三方插件 CI：

```text
language unit tests
  -> lint / static analysis
  -> build one artifact per target
  -> validate manifest and JSON Schema
  -> plugincheck on runnable targets
  -> assemble archive
  -> verify archive hash and signature
```

Meerkit 完整发布包已包含对应平台的独立 `meerkit-plugincheck`。第三方仓库可以下载该工具执行测试，无需拉取 Meerkit 源码。
