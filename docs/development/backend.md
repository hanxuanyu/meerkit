# 后端开发

后端使用 Go 1.26。根命令通过 Cobra 解析参数，`internal/application` 负责装配数据库、插件、通知、调度和 HTTP Server。

## 启动

```bash
make deps
make dev-backend
```

`make dev-backend` 会在 `web/dist/index.html` 缺失时先构建前端，然后执行 `go run . serve`。传递参数：

```bash
make dev-backend \
  BACKEND_ARGS="--config config.yaml --listen 127.0.0.1:8080"
```

没有通过 ldflags 注入版本时，宿主版本为 `dev`。开发宿主会扫描 `plugins.source_dir`，构建除模板外的源码插件，并把产物写到数据目录。

## 分层约定

- `internal/core` 保持传输和数据库无关。公共领域字段先在这里定义。
- `internal/store` 实现 repository 合约，新增持久化字段时同时处理模型转换、SQLite/MySQL Schema 和索引。
- `internal/runtime` 组织具有状态变化的监控流程，不能把通知或条件状态只留在 HTTP Handler。
- `internal/api` 负责请求解码、状态码和错误信封，不直接持有 SQL。
- `internal/plugin` 负责不可信包输入与进程生命周期，所有文件路径和外部制品都要在使用前验证。

## 数据库变化

迁移在 `internal/store/migrate.go` 中按单调递增版本声明。新增迁移需要：

1. 同时支持 SQLite 和 MySQL。
2. 可在已有数据上执行，不依赖空表。
3. 在 MySQL 上使用现有迁移锁，避免并发应用。
4. 为 `auto_migrate: false` 保留明确的缺失版本错误。
5. 添加两种数据库可共用的 repository 测试；外部 MySQL 集成测试按环境条件运行。

不要修改已经发布迁移的语义，应追加新版本。

## API 变化

当前管理 API 前缀为 `/api/v1`。新增写路由默认经过会话和 CSRF 中间件。公开端点仅限认证初始化、登录和健康检查，扩展公开面需要显式安全评估。

统一错误结构：

```json
{
  "code": "validation_error",
  "message": "human-readable detail"
}
```

涉及资源状态的更新应返回稳定 HTTP 状态码并保留已有前端消费字段。配置更新需要继续使用版本号做乐观并发控制。

## 并发与取消

- HTTP 请求 Context 要传到数据库和插件调用。
- 同一监控通过进程内锁避免重叠执行。
- 调度器的并发上限是全局计数，不是工作队列。
- 插件替换前通过 `ExecutionGate` 阻止新调用并等待当前调用完成。
- 关闭流程由信号 Context 驱动，HTTP 优雅关闭上限 10 秒。

新增后台循环必须支持 Context 取消，并在运行时配置变化时正确重置计时器。

## 测试

```bash
go test ./...
go test -race ./...
```

共享行为优先使用临时 SQLite 测试真实 repository。API 测试使用 `httptest` 并覆盖认证、CSRF、状态码和错误体。插件包输入应覆盖路径穿越、重复内容、哈希、签名和目标平台选择。

在提交前还要运行独立 SDK 和插件模块测试，参见[架构与仓库](/development/overview#go-workspace)。
