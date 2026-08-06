# 打包脚本

- `package-plugins.sh --plugin <目录> [pluginpack 参数]`：打包单个插件；使用 `--generate-key` 时生成签名密钥。
- `package-plugins.sh [输出目录] [目标平台]`：不传 `--plugin` 时打包 `plugins/` 下全部正式插件，目标格式为 `GOOS/GOARCH`，多个目标用逗号分隔。
- `package.sh [输出目录] [目标平台]`：构建前端、宿主程序和全部正式插件，生成完整发布压缩包。
- 同目录 PowerShell 脚本提供 Windows 等效流程。

```sh
scripts/package-plugins.sh --plugin plugins/http --targets current
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
scripts/package.sh dist/releases darwin/arm64,linux/amd64,windows/amd64
```

批量生成可信签名包时，先使用 `pluginpack --generate-key` 生成密钥，再为脚本设置签名私钥和 key ID。`package.sh` 会将这两个环境变量传递给内部的插件打包流程：

```sh
scripts/package-plugins.sh --generate-key ./keys/meerkit-release
MEERKIT_PLUGIN_SIGN_KEY=./keys/meerkit-release.private.key \
MEERKIT_PLUGIN_KEY_ID=meerkit-release-2026 \
scripts/package-plugins.sh dist/plugins linux/amd64,windows/amd64
```

`package.sh` 使用相同环境变量为完整发布包中的所有内置插件签名。内置插件会在空数据目录首次启动时建立官方信任；第三方签名由用户首次核对指纹后保存。`plugins.trusted_keys` 仅用于可选的预置信任。私钥不得提交到仓库或包含在发布包中。完整说明参见 [`plugins/README.md`](../plugins/README.md)。

`plugins/template` 是源码模板而非可发布监控插件，因此是自动发布流程唯一排除的目录。
