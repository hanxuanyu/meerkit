# 打包与发布

仓库脚本构建 Meerkit 宿主和官方 Go 插件。第三方语言插件只复用包格式与测试工具，自行负责源码到制品的构建。

## 生成签名密钥

```bash
make generate-key KEY_PREFIX=./keys/meerkit-official
```

生成两个 Base64 文件，并拒绝覆盖已有路径：

- `meerkit-official.private.key`：Ed25519 私钥，权限 `0600`。
- `meerkit-official.public.key`：Ed25519 公钥。

私钥不得提交、放入 `dist/` 或部署到应用主机。key ID 是人类可读标签；真实身份是公钥 SHA-256 指纹。

## 官方插件

```bash
make package-plugins \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026 \
  TARGETS=linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

脚本扫描 `plugins/*/meerkit-plugin.yaml` 并跳过 `template`。每个目标独立构建，Windows 输出 `.zip`，其他平台输出 `.tar.gz`。

只构建一个插件：

```bash
make package-plugin \
  PLUGIN=./plugins/http \
  TARGETS=linux/amd64 \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026
```

底层 `cmd/pluginpack` 只执行 `go build`，所以不适合第三方语言。它会把源码清单中的空 `artifacts` 替换为制品路径、大小和 SHA-256，再选择性生成签名信封。

## 完整 Meerkit 发布

```bash
make package-release \
  VERSION=v0.1.0 \
  SIGN_KEY=./keys/meerkit-official.private.key \
  KEY_ID=meerkit-official-2026 \
  TARGETS=linux/amd64,linux/arm64,windows/amd64,darwin/arm64
```

流程会：

1. 构建 `web/dist`。
2. 为目标平台构建带版本的宿主。
3. 构建独立 `meerkit-plugincheck`。
4. 使用相同密钥打包全部官方插件。
5. 加入 README、`config.example.yaml` 和根 `LICENSE`。
6. 生成平台压缩包。

`TARGETS` 默认当前平台。`SIGN_KEY` 与 `KEY_ID` 必须同时出现；同时省略会生成未签名插件。

## 官方信任引导

正式宿主在启动时扫描可执行文件同级 `plugins/`。空数据目录首次导入随发布包提供的签名插件时，会把其发布者记录为官方并启用。之后相同公钥签名的新包可以自动验证。

清单没有 `official: true` 字段。把单独插件包导入另一个尚未建立官方信任的环境时，仍需要像第三方发布者一样确认指纹。

## 第三方语言包

第三方维护者自行：

1. 为每个目标生成单文件制品。
2. 写入最终 `artifacts`，包括运行方式、大小和 SHA-256。
3. 在包根加入清单、README 和许可证。
4. 按 `meerkit-plugin-signature-v1` 规则可选生成 Ed25519 签名。
5. 使用 `meerkit-plugincheck` 对每个可运行目标做黑盒测试。
6. 输出 `.zip` 或 `.tar.gz`。

当前仓库没有提供跨语言通用组包 CLI；包格式和签名负载以清单 Schema、协议文档和 `internal/plugin/signature.go` 为准。

## 发布检查表

- 所有 Go module 测试通过。
- 前端和文档站点构建通过。
- HTTP/TCP 插件黑盒检查通过。
- 版本号没有复用旧内容。
- 发布包包含 Apache-2.0 `LICENSE`。
- 私钥不在 Git、日志或构建产物中。
- 对外公布发布文件 SHA-256 与签名公钥指纹。
- 在全新数据目录执行首次启动和一个监控闭环。
