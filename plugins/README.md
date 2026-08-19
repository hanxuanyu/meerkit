# Meerkit 监控插件

[English](README.en.md) · [插件开发文档](../docs/development/plugin-go.md) · [跨语言协议](../sdk/PROTOCOL.md)

此目录包含官方监控插件与源码模板。监控插件负责描述配置和结果、校验配置、执行探测以及迁移配置；调度、条件计算、结果持久化和通知仍由宿主负责。

## 目录内容

| 目录 | 模块类型 | 当前能力 |
| --- | --- | --- |
| [`http`](http/README.md) | `http` | HTTP/HTTPS 请求、代理、请求体、重定向、TLS、文本/JSON 响应解析 |
| [`tcp`](tcp/README.md) | `tcp` | TCP 连接、可选数据发送、单次响应读取、文本与 Base64 数据 |
| [`browser-example`](browser-example/README.md) | `browser-example-element`、`browser-example-response` | 浏览器标签页复用、DOM 读取和网络响应捕获的完整示例 |
| [`template`](template/README.md) | `example` | Go 插件最小实现和一致性测试套件 |

每个插件是独立 Go 模块。宿主开发模式会扫描 `plugins.source_dir` 下的清单，跳过 `template`，构建插件并从 `${storage.data_dir}/plugins/development` 运行。正式版本只运行已安装的插件包。

## 生命周期

1. 宿主读取并校验 `meerkit-plugin.yaml`，选择当前 `GOOS/GOARCH` 的唯一制品。
2. 宿主校验制品大小和 SHA-256，并在有签名时校验 `meerkit-plugin.sig`。
3. 启用前处理发布者信任：官方、已信任、待信任或未签名。
4. 宿主按清单级 `runtime` 和可选的 `artifacts[].runtime` 覆盖启动子进程，完成 go-plugin 握手和两级健康检查。
5. 插件返回模块描述器；宿主核对其与清单的类型和版本是否一致。
6. 宿主迁移已保存的监控配置，并原子替换该插件拥有的监控模块。

同一插件 ID 同时只启用一个版本。禁用插件会停止进程并移除模块；依赖该模块的监控会保留，但在模块恢复前不能执行。

## 包格式

Meerkit 只导入 `.zip` 或 `.tar.gz`，不会扫描裸可执行文件。包根目录包含：

```text
meerkit-plugin.yaml
meerkit-plugin.sig       # 可选 Ed25519 签名信封
README.md                # 可选，显示在插件详情中
README.en.md             # 可选
LICENSE                  # 推荐
bin/<goos>-<goarch>/...  # 清单声明的制品
```

清单 Schema 位于 [`manifest.schema.json`](manifest.schema.json)。源码清单必须声明顶层 `runtime`，并允许 `artifacts: []`；仓库打包器会保留启动配置并为 Go 插件写入平台、路径、大小和 SHA-256。第三方语言自行构建制品和组包，格式必须遵循同一清单和线协议。

## 信任模型

- 随正式 Meerkit 发布包首次引导的官方插件会建立 `official` 信任。
- 已签名但发布者未知的插件在导入后不能启用；用户必须核对并确认公钥 SHA-256 指纹。
- 同一公钥后续签名的插件可自动识别为已信任发布者。
- 未签名插件只能在明确确认风险后启用。
- `plugins.trusted_keys` 可以把 Base64 Ed25519 公钥预置为可信发布者。

签名覆盖清单、清单内的制品哈希，以及包中的 README 和 LICENSE。签名证明包由相应私钥持有者生成，不代表插件进程受到隔离。

## 开发与测试

```bash
# 运行所有独立插件模块测试
(cd plugins/http && go test ./...)
(cd plugins/tcp && go test ./...)
(cd plugins/browser-example && go test ./...)
(cd plugins/template && go test ./...)

# 让开发宿主自动构建并运行 HTTP/TCP 源码插件
go run . serve
```

新 Go 插件从 [`template`](template/README.md) 开始。第三方语言实现以 [`sdk/PROTOCOL.md`](../sdk/PROTOCOL.md)、[`monitor.proto`](../sdk/proto/monitor.proto) 和 [`sdk/schema`](../sdk/schema/) 为准，不应依赖 Go 接口。

打包、签名与发布命令见[插件打包与发布](../docs/development/releasing.md)和 [`scripts/README.md`](../scripts/README.md)。

## 许可证

仓库内官方插件随 Meerkit 以 [Apache License 2.0](../LICENSE) 开源。第三方插件可以采用自己的许可证，并应在包内携带对应 `LICENSE`。
