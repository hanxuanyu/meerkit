# Meerkit 监控插件

每个插件都是独立 Go 模块，通过公共 SDK 和 HashiCorp go-plugin gRPC 通道与 Meerkit 通信。Meerkit 仅导入 `.zip` 和 `.tar.gz` 插件包，将裸二进制放入运行目录不会触发加载。

## 插件开发流程

1. 从 `plugins/template` 创建独立 Go 模块，实现 SDK `Module` 接口及契约测试。仓库内开发时可直接执行 `go run .`，宿主会构建并运行 `plugins` 下除 `template` 外的源码插件。
2. 在 `meerkit-plugin.yaml` 中填写唯一 ID、语义化版本、开发者、`desp` 功能描述、`url` 源码或发布地址、协议范围及模块版本。
3. 使用 `scripts/package-plugins.sh --plugin` 构建压缩包。工具会交叉编译制品，并自动写入平台、大小和 SHA-256。
4. 正式发布时使用长期保管的 Ed25519 私钥签名；开发期可以生成独立测试密钥。
5. 用户首次导入该公钥签名的插件时核对指纹并信任发布者，之后相同公钥签名的其他插件会自动验证。

## 信任模型

- 官方发布：正式发布流程使用固定官方私钥签名 HTTP、TCP 等内置插件。空数据目录首次启动时，随应用分发的官方插件会建立本地官方信任，因此后续手工导入的同一公钥签名包自动可信，不需要公共密钥服务或修改配置。
- 第三方发布：签名包携带公钥。Meerkit 先独立验证清单签名，再显示 key ID 和 SHA-256 公钥指纹；用户确认一次后，信任记录保存在本地数据库。
- 自行部署：不对外分发时可以继续使用未签名包。未签名包可以导入，但每次启用都需要明确确认风险，也无法证明包的发布者身份。
- 预置信任：无人值守部署可选择在 `plugins.trusted_keys` 中配置 Base64 Ed25519 公钥。该配置不是普通用户导入插件的必要步骤。

签名只证明“包由持有该私钥的人发布且内容未被修改”，不表示插件代码安全。首次信任第三方公钥时，应从其独立源码或发布页面核对指纹。

## 打包

当前平台、多个平台和整合包示例：

```sh
scripts/package-plugins.sh --plugin ./plugins/http
scripts/package-plugins.sh --plugin ./plugins/http --targets linux/amd64,linux/arm64,windows/amd64,darwin/arm64
scripts/package-plugins.sh --plugin ./plugins/http --targets linux/amd64,windows/amd64 --combined
```

生成 Ed25519 密钥对。私钥文件仅用于打包签名，不要放入插件包或提交到仓库：

```sh
scripts/package-plugins.sh --generate-key ./keys/meerkit-release
```

使用长期稳定的 key ID 和私钥打包：

```sh
scripts/package-plugins.sh \
  --plugin ./plugins/http \
  --targets current \
  --sign-key ./keys/meerkit-release.private.key \
  --key-id meerkit-release-2026
```

插件包可复制到 `${data_dir}/plugins/inbox`，也可以从管理页面导入。签名覆盖清单、清单中的制品哈希以及 README/许可证文档；未知公钥显示为“待信任”，已确认、预配置或由官方包引导的公钥显示为“已验证”。相同插件 ID 和版本如果已有不同内容，需要先卸载旧版本或提升清单版本。

源码开发模式不受最后一项限制：默认的 `plugins.mode: auto` 在 `dev` 宿主中会在每次启动时覆盖同 ID、同版本的开发二进制。需要验证真实导入和签名行为时，将 `plugins.mode` 临时改为 `package`。

官方完整发布包应在执行 `scripts/package.sh` 时设置 `MEERKIT_PLUGIN_SIGN_KEY` 和 `MEERKIT_PLUGIN_KEY_ID`，确保所有内置插件使用同一发布密钥。私钥丢失或更换后，现有用户需要重新确认新指纹，因此应备份私钥并制定密钥轮换发布计划。插件进程与 Meerkit 具有相同 OS 用户权限，不属于安全沙箱。
