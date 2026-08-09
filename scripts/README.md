# 构建与发布脚本

[English](README.en.md) · [发布文档](../docs/development/releasing.md)

此目录提供 Unix Shell 与 Windows PowerShell 的等效发布流程。脚本只构建 Meerkit 宿主、通用检查工具和仓库内的 Go 官方插件，不负责构建第三方语言插件。

## 脚本

| Shell | PowerShell | 用途 |
| --- | --- | --- |
| `package-plugins.sh` | `package-plugins.ps1` | 构建单个或全部官方插件，可生成密钥和签名 |
| `package.sh` | `package.ps1` | 构建前端、宿主、`meerkit-plugincheck` 和官方插件发布包 |

脚本应从仓库根目录运行。默认目标是当前 `GOOS/GOARCH`，多个目标用逗号分隔。

```bash
scripts/package-plugins.sh --plugin plugins/http --targets current
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
scripts/package.sh dist/releases darwin/arm64,linux/amd64,windows/amd64
```

`plugins/template` 是源码模板，批量打包时会跳过。

## 签名

```bash
scripts/package-plugins.sh --generate-key ./keys/meerkit-release

MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-release.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-release-2026 \
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
```

私钥与 key ID 必须同时设置。私钥只应保存在发布环境或 CI 密钥系统中；脚本生成的公钥可用于发布指纹或配置 `plugins.trusted_keys`。密钥生成拒绝覆盖已有文件。

## 完整发布包

```bash
MEERKIT_VERSION=v0.1.0 \
MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-release.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-release-2026 \
scripts/package.sh dist/releases linux/amd64,linux/arm64
```

每个平台输出一个独立压缩包，包含：

```text
meerkit
meerkit-plugincheck
plugins/
README.md
README.en.md
LICENSE
config.example.yaml
```

Windows 输出 `.zip` 和 `.exe`；其他平台输出 `.tar.gz`。所有 Go 构建使用 `CGO_ENABLED=0` 与 `-trimpath`。完整包中的官方插件共享同一签名密钥。

常用流程也可通过根 `Makefile` 调用：`make generate-key`、`make package-plugin`、`make package-plugins` 和 `make package-release`。

## 许可证

脚本随 Meerkit 以 [Apache License 2.0](../LICENSE) 开源，完整发布包会携带根 `LICENSE`。
