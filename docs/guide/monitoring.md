# 监控与条件

一个监控项把监控模块、执行计划、模块配置、条件规则和通知渠道绑定在一起。

```text
Cron / 手动执行
       |
       v
插件 ValidateConfig -> Execute -> Observation
                                  |
                                  v
                 保存结果 -> 条件计算 -> 触发/恢复事件 -> 通知
                                  |
                                  +-> 状态看板与趋势规则
```

## 执行计划

每个监控可以设置多个 Cron 表达式。系统选择所有表达式中最早的下一次运行时间，并使用运行时配置 `scheduler.timezone` 解释它们。

```text
*/5 * * * *       每 5 分钟
0 */2 * * *       每 2 小时整点
0 9 * * MON-FRI   工作日 09:00
@hourly            每小时
```

解析器支持可选秒字段，所以 `0 */5 * * * *` 也有效。表单会调用后端预览接口返回表达式说明和接下来三次时间。

调度器的全局并发上限默认为 16。达到上限时，本轮到期任务会被跳过并写入警告日志；同一个监控不会并发执行。插件模块不可用或版本不匹配时，定时任务暂停，模块恢复后自动继续。

## 模块配置

监控编辑器由插件返回的模块描述器生成。字段类型、默认值、数值范围、选项、显隐关系和敏感标记均由插件声明。

保存前，宿主会调用插件的 `ValidateConfig`。插件版本升级且 `config_version` 变化时，宿主会先调用 `MigrateConfig` 迁移已保存配置；迁移失败会中止新版本启用，避免在未知配置上继续执行。

## 结果结构

每次执行保存：

- 插件观测中的 `success`、`summary`、错误代码和错误信息。
- 宿主测量的总 `duration_ms`。
- 插件描述器定义的一个或多个结果集。
- `summary` 公共结果集，包括条件状态、事件类型、匹配数量和逐条详情。
- 结果 Schema 版本和整个结果的 SHA-256 哈希。

HTTP 与 TCP 插件的字段分别见其 [HTTP README](https://github.com/hanxuanyu/meerkit/blob/main/plugins/http/README.md) 和 [TCP README](https://github.com/hanxuanyu/meerkit/blob/main/plugins/tcp/README.md)。

## 条件规则

规则左值可以来自当前或上一次成功执行结果。比较值可以是固定值，也可以引用当前或上一次结果中的另一个字段。结构化字段可以继续指定 JSON 路径。

| 操作符 | 适用场景 |
| --- | --- |
| `equals` / `not_equals` | 精确比较 |
| `contains` / `not_contains` | 文本、列表或集合包含关系 |
| `regex` | 对文本执行 Go 正则匹配 |
| `gt` / `gte` / `lt` / `lte` | 数值比较 |
| `between` | 包含上下界的数值区间 |
| `length_gt` / `length_eq` | 字符串、数组或映射长度 |
| `is_true` / `is_false` | 布尔字段 |
| `exists` / `not_exists` | 字段或路径是否存在 |
| `changed` | 当前值与上次成功结果是否不同 |

插件只会把字段实际支持的操作符提供给界面。类型错误、字段缺失或无效路径会得到 `unknown`，而不是强制转换成 `false`。

## ALL、ANY 与 unknown

- `ALL`：任一规则为 `false` 则整体为 `false`；没有 `false` 但存在 `unknown` 时整体为 `unknown`。
- `ANY`：任一规则为 `true` 则整体为 `true`；没有 `true` 且仅有未知结果时为 `unknown`。
- 没有规则时，整体状态为 `false`。

`changed` 第一次执行只建立基线并返回 `false`。后续比较使用上一次成功执行的结果。

## 事件与通知策略

条件整体状态影响监控运行时状态：

| 变化 | 事件 |
| --- | --- |
| 非活动 -> `true` | `triggered` |
| 活动 -> `false` | `recovered` |
| 状态为 `unknown` | 不改变活动状态，不发事件 |

“仅状态变化时通知”对应 `once`，是默认策略。它只在首次触发和恢复时通知。“每次匹配都通知”对应 `every`，条件连续为 `true` 时每次执行都会产生 `triggered`，恢复仍只产生一次。

## 记录保留

默认执行记录保留 30 天，清理任务每小时运行。可在“设置”中修改 `storage.retention` 和 `storage.cleanup_interval`。删除某个监控的记录会同时影响它的历史对比和状态看板样本。
