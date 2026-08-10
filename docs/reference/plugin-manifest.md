# 插件清单参考

每个插件包根目录必须有 `meerkit-plugin.yaml`。机器可读 Schema 位于 [`plugins/manifest.schema.json`](https://github.com/hanxuanyu/meerkit/blob/main/plugins/manifest.schema.json)。未知字段会被 Schema 拒绝。

## 源码清单

```yaml
# yaml-language-server: $schema=../manifest.schema.json
schema_version: 1
id: example.monitor
name: Example Monitor
version: 0.1.0
vendor: Example
desp: 一个示例监控插件。
url: https://example.com/meerkit-plugin
protocol:
  min: 1
  max: 1
modules:
  - type: example
    name: Example
    version: "1"
    config_version: "1"
    result_schema_version: "1"
runtime:
  mode: direct
  args: []
artifacts: []
```

仓库 Go 打包器接受空 `artifacts` 并写入构建结果。最终分发包必须至少包含当前平台制品。

## 顶层字段

| 字段 | 约束 | 含义 |
| --- | --- | --- |
| `schema_version` | 当前固定 `1` | 清单格式版本 |
| `id` | 小写字母/数字，可用 `.` `_` `-` 分段 | 全局稳定插件 ID |
| `name` | 非空 | 展示名称 |
| `version` | 语义版本 | 完整插件包版本，同 ID+版本内容不可变 |
| `vendor` | 非空 | 发布者展示名，不作为信任身份 |
| `desp` | 非空 | 能力摘要 |
| `url` | 绝对 HTTP/HTTPS URL | 源码或可信发布页面 |
| `protocol.min/max` | 正整数范围 | 支持的 Meerkit 应用协议 |
| `modules` | 至少一个，类型不重复 | 插件提供的监控模块 |
| `runtime` | 必填 | 源码构建和全部制品的默认启动配置 |
| `artifacts` | 目标不重复、路径不重复 | 可运行制品 |

## 模块字段

| 字段 | 含义 |
| --- | --- |
| `type` | 模块稳定 ID，必须与 `ListModules` 描述器一致 |
| `name` | 模块展示名 |
| `version` | 模块实现兼容版本 |
| `config_version` | 已保存配置结构版本，变化时触发迁移 |
| `result_schema_version` | 持久化结果结构版本 |

包 `version`、模块 `version`、配置版本和结果版本各自独立。它们都使用字符串，但只有包版本强制语义版本格式。

## 制品字段

```yaml
artifacts:
  - goos: linux
    goarch: amd64
    path: bin/linux-amd64/plugin
    size: 12345678
    sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
    runtime:
      mode: direct
      args: []
```

| 字段 | 约束 |
| --- | --- |
| `goos` / `goarch` | 小写字母数字；使用 Go 目标命名 |
| `path` | 包内相对规范路径，无反斜杠、绝对路径或 `..` |
| `size` | 正整数，精确字节数 |
| `sha256` | 64 位十六进制 |
| `runtime` | 可选；覆盖清单级默认启动配置 |

宿主优先使用 `artifacts[].runtime`，未配置制品级覆盖时使用必填的清单级 `runtime`。源码构建、黑盒测试和发布包因此使用同一启动契约；仅在某个平台需要不同解释器或参数时填写制品级覆盖。

宿主要求当前运行平台恰好匹配一个制品。制品最终安装为数据目录中的 `plugin` 或 `plugin.exe`，清单路径仍用于导入校验。

## 直接执行

```yaml
runtime:
  mode: direct
  args: ["serve"]
```

`direct` 固定执行宿主解析出的当前平台制品，因此不能声明 `command`。`args` 必填；没有参数时写 `[]`。参数最多 32 项，每项非空、最长 4096 字节，不能包含 NUL，也不能使用 `{artifact}`。

## 解释器执行

```yaml
runtime:
  mode: interpreter
  command: python3
  args: ["-I", "{artifact}"]
```

`command` 是从 `PATH` 查找的命令名，只允许字母、数字、`.`、`_`、`+`、`-`，不能包含目录。`args` 必须恰好包含一次独立 `{artifact}`。宿主不调用 shell。

```yaml
# Java 单文件 JAR
runtime:
  mode: interpreter
  command: java
  args: ["-jar", "{artifact}"]
```

## 包级可选文件

- `meerkit-plugin.sig`：Ed25519 签名信封。
- `README.md` / `README.en.md`：插件详情内容。
- `LICENSE`：插件许可证。

签名负载绑定清单，以及存在的 README 与 LICENSE；制品内容通过清单中的 SHA-256 间接绑定。签名信封包含版本、key ID、公钥和 Base64 签名。

## 清单与运行时一致性

启用时，宿主要求 `ListModules` 返回相同模块数量、类型和 `version`。宿主会把清单中的 `config_version` 与 `result_schema_version` 关联到运行描述器；执行 Observation 仍应主动返回正确结果版本。
