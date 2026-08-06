# Meerkit

Meerkit 是一套支持独立监控插件的监控服务，并内置 Webhook、SMTP 和站内通知能力。HTTP 与 TCP 以官方插件形式发布，不再链接到宿主进程中。

## 运行

```bash
npm --prefix web run build
go run . serve --config config.yaml
```

默认管理界面地址为 `http://127.0.0.1:8080`。首次访问时需要设置管理员访问密钥，后续所有管理 API 和通知流都要求有效会话。本地重置命令：

```bash
meerkit --data-dir ./data admin reset-key --key '新的管理员访问密钥'
```

配置优先级依次为内置默认值、`config.yaml`、`MEERKIT_*` 环境变量和命令行参数。嵌套环境变量使用双下划线：

```bash
MEERKIT_SERVER__PORT=9090 MEERKIT_STORAGE__DATA_DIR=/var/lib/meerkit go run .
```

## 插件

插件包可通过管理页面上传、`meerkit plugin import` 导入，或复制到 `${data_dir}/plugins/inbox` 自动发现。仅支持 `.zip` 和 `.tar.gz`，不会发现或执行裸二进制文件。

使用 `scripts/package-plugins.sh` 打包全部正式插件，使用 `scripts/package.sh` 生成包含宿主和全部正式插件的平台发布包；Windows 对应脚本位于同一目录。更多信息参见 [`plugins/README.md`](plugins/README.md)。

## 目录

```text
main.go
internal/
  api/           HTTP API 与嵌入式前端处理
  app/           外部配置加载
  application/   服务依赖装配与生命周期
  auth/          管理员凭据与会话
  command/       Cobra 命令树与命令实现
  core/          监控、条件、结果和通知契约
  monitor/       带所有者的模块注册表与远程适配器
  plugin/        插件包校验、安装和进程管理
  runtime/       执行器与 Cron 调度器
  store/         SQLite 持久化
plugins/          官方插件、模板和打包工具
sdk/              公共监控插件 SDK 与 gRPC 协议
scripts/          项目与插件发布脚本
web/              React/Vite 前端与通用 UI 组件
```
