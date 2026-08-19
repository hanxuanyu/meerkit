# 浏览器 Action 参考

本页记录协议 v1 当前实现的 56 个浏览器原子 Action。运行时 Catalog 由 `GET /api/v1/browser/actions` 返回，是 UI 参数描述器和 Agent capability 的事实来源；本页用于插件开发和代码审查。

## 通用请求与响应

```json
{
  "target": {
    "agent_id": "agent-id",
    "window_id": 4,
    "tab_id": 21
  },
  "timeout_ms": 60000,
  "action": {
    "id": "step-id",
    "type": "dom.query",
    "params": { "selector": "main" }
  }
}
```

```json
{
  "id": "step-id",
  "type": "dom.query",
  "success": true,
  "target": {
    "agent_id": "agent-id",
    "window_id": 4,
    "tab_id": 21
  },
  "duration_ms": 12,
  "data": {}
}
```

`timeout_ms` 默认 60000，最小 1000，最大 300000。`params` 省略时视为空对象；宿主先用 Catalog 默认值补齐，再校验并发送。参数中的整数和数字必须是 JSON number，布尔值必须是 JSON boolean，不能用字符串代替。

目标模式：`none` 不需要目标，`window_optional` 可选窗口，`window_required` 必选窗口，`tab_required` 必选标签页。所有目标 ID 必须为正整数；同时提供窗口和标签页时必须属于同一窗口。

下表中的参数记法为：`*` 表示必填，`=` 后是默认值，`[min..max]` 是闭区间，`a|b` 是枚举。返回列只描述 `data`；外层标准响应字段始终存在。

## 通用结果对象

多个 Action 复用以下对象：

| 名称 | 字段 |
| --- | --- |
| `tab` | `tab_id`、`window_id`、`index`、`url`、`title`、`status`、`active`、`pinned`、`muted`、`audible`、`discarded`、`auto_discardable` |
| `window` | `window_id`、`focused`、`type`、`state`、`left`、`top`、`width`、`height`、`tabs: tab[]` |
| `group` | `group_id`、`window_id`、`title`、`color`、`collapsed`，`tab.group` 额外返回 `reused` |
| `cookie` | `name`、`value`、`domain`、`path`、`secure`、`http_only`、`same_site`、`session`、`expiration_date`、`store_id` |

JSON `omitempty` 字段或 Chrome 未提供的值可能缺失。插件应做字段存在性和类型检查。

## 窗口

| Action | 目标 | 参数 | `data` 返回 | 标记 |
| --- | --- | --- | --- | --- |
| `window.open` | `none` | `url=about:blank`；`type=normal` (`normal|popup`)；`state=normal` (`normal|minimized|maximized|fullscreen`)；`width,height[100..10000]`；`left,top[-10000..10000]` | 新建的 `window`；外层 `target` 同时包含首个标签页 | - |
| `window.focus` | `window_required` | 无 | 更新后的 `window` | - |
| `window.state` | `window_required` | `state*=normal` (`normal|minimized|maximized|fullscreen`) | 更新后的 `window` | - |
| `window.resize` | `window_required` | `width*=1280[100..10000]`；`height*=800[100..10000]`；可选 `left,top[-10000..10000]` | 状态切换为 `normal` 后的 `window` | - |
| `window.close` | `window_required` | 无 | `window_id`、`closed:true`；外层目标被清空 | destructive |

`window.open` 只有 `state=normal` 时应用宽、高和位置。关闭窗口会同时关闭其中全部标签页。

## 标签页

| Action | 目标 | 参数 | `data` 返回 | 标记 |
| --- | --- | --- | --- | --- |
| `tab.open` | `window_optional` | `url=about:blank`；`active=true`；`wait=true` | 新标签页 `tab`，外层目标更新为新标签页 | - |
| `tab.activate` | `tab_required` | 无 | 激活后的 `tab`，并聚焦窗口 | - |
| `tab.navigate` | `tab_required` | `url*`（HTTP/HTTPS）；`wait=true` | 导航后的 `tab` | - |
| `tab.reload` | `tab_required` | `bypass_cache=false`；`wait=true` | 刷新后的 `tab` | - |
| `tab.back` | `tab_required` | 无 | 后退后的 `tab`；无历史记录时 Chrome 返回错误 | - |
| `tab.forward` | `tab_required` | 无 | 前进后的 `tab`；无历史记录时 Chrome 返回错误 | - |
| `tab.duplicate` | `tab_required` | `active=true` | 副本 `tab`，外层目标切换到副本 | - |
| `tab.move` | `tab_required` | 可选 `destination_window_id`；`index=-1[-1..100000]` | 移动后的 `tab` | - |
| `tab.pin` | `tab_required` | `pinned=true` | 更新后的 `tab`；传 `false` 取消固定 | - |
| `tab.mute` | `tab_required` | `muted=true` | 更新后的 `tab`；传 `false` 取消静音 | - |
| `tab.discard` | `tab_required` | 无 | 卸载后的 `tab`；活动标签页可能被 Chrome 拒绝 | - |
| `tab.auto_discardable` | `tab_required` | `auto_discardable=true` | 更新后的 `tab` | - |
| `tab.detect_language` | `tab_required` | 无 | `tab_id`、`language`（Chrome 检测的语言代码） | - |
| `tab.group` | `tab_required` | `title*=Meerkit`（1..128 字符）；`color=blue`；`collapsed=false`；`reuse_group=true` | `group` 和 `reused` | - |
| `tab.ungroup` | `tab_required` | 无 | 移出分组后的 `tab` | - |
| `tab.zoom` | `tab_required` | `factor*=1[0.25..5]` | `tab_id`、实际 `factor` | - |
| `tab.close` | `tab_required` | 无 | `tab_id`；外层目标移除标签页 | destructive |

分组颜色可选 `grey|blue|red|yellow|green|pink|purple|cyan|orange`。`tab.open` 不接受已有 `tab_id`；它先创建 `about:blank`，再按需导航，因此插件可以可靠取得新标签页目标。

## 页面

| Action | 目标 | 参数 | `data` 返回 |
| --- | --- | --- | --- |
| `page.info` | `tab_required` | 无 | `url`、`title`、`ready_state`、`viewport{width,height,device_pixel_ratio}`、`document{width,height}`、`scroll{x,y}` |
| `page.wait` | `tab_required` | `mode=load`；条件参数见下文 | `ready:true`、最终 `mode` |
| `page.scroll` | `tab_required` | `mode=relative` (`absolute|relative|top|bottom`)；`x=0,y=600[-10000000..10000000]`；`behavior=auto` (`auto|smooth`) | `mode`、`behavior`、`requested{x,y}`、当前 `scroll{x,y}` |
| `page.stop_loading` | `tab_required` | 无 | `tab_id`、`stopped:true` |
| `page.performance` | `tab_required` | 无 | `url`、`time_origin`、`navigation`、`paints`、`resources{count,transfer_size,encoded_body_size,decoded_body_size}` |
| `page.screenshot` | `tab_required` | `format=png` (`png|jpeg|webp`)；`quality=90[1..100]`；`full_page=false` | `data_url`、`format`、`full_page`、估算 `size_bytes` |

`page.wait` 模式：

| `mode` | 参数 | 完成条件 |
| --- | --- | --- |
| `load` | 无 | 标签页状态为加载完成 |
| `selector` | `selector*`；`timeout_ms=60000[100..300000]` | 元素存在 |
| `visible` | 同上 | 元素存在且有尺寸、非 `display:none/visibility:hidden` |
| `hidden` | 同上 | 元素不存在或不可见 |
| `text` | `value*`；`timeout_ms=60000[100..300000]` | 页面正文包含文本 |
| `url` | 同上 | 当前 URL 包含文本 |
| `title` | 同上 | 页面标题包含文本 |
| `duration` | `duration_ms=1000[0..300000]` | 固定等待结束 |

截图 Base64 位于 Data URL 中。大结果会在扩展协议层分块，但插件仍收到完整 JSON；序列化结果超过 60 MiB 时失败。完整页面优先使用 WebP/JPEG。

## DOM

除特别说明外，`selector` 必填，长度为 1 到 4096 字符，只查询顶层文档，不穿透 iframe 或 shadow root。

| Action | 参数 | `data` 返回 | 标记 |
| --- | --- | --- | --- |
| `dom.document` | `max_length=262144[1024..1048576]` | `url`、`title`、截断后的 `html`、`truncated` | - |
| `dom.query` | `selector*`；`max_length=65536[256..1048576]` | `url`、`title`、`selector`、`tag_name`、`text`、`html`、`attributes`、可选 `value/checked/disabled`、`visible`、`bounding_rect`、`truncated` | - |
| `dom.query_all` | `selector*`；`limit=50[1..500]`；`max_length=4096[64..65536]` | `selector`、总匹配数 `total`、`elements[]`、`truncated` | - |
| `dom.focus` | `selector*` | `selector`、`focused` | - |
| `dom.blur` | `selector*` | `selector`、`focused`（成功时通常为 false） | - |
| `dom.click` | `selector*` | `selector`、`clicked:true` | - |
| `dom.input` | `selector*`；`value=""`（最大 1 MiB） | `selector`、实际 `value`、`focused`、`updated` | - |
| `dom.check` | `selector*`；`checked=true` | `selector`、实际 `checked` | - |
| `dom.select` | `selector*`；`value*`（最大 1 MiB） | `selector`、实际 `value` | - |
| `dom.submit` | `selector*` | `selector`、`submitted:true` | - |
| `dom.set_attribute` | `selector*`；`name*`（最大 256 字符）；`value`（最大 1 MiB） | `selector`、`name`、实际 `value` | destructive |
| `dom.remove_attribute` | `selector*`；`name*`（最大 256 字符） | `selector`、`name`、`removed` | destructive |
| `dom.dispatch_event` | `selector*`；`event*=change`；`bubbles=true`；`cancelable=true` | `selector`、`event`、`bubbles`、`cancelable`、`default_prevented` | - |
| `dom.scroll_into_view` | `selector*`；`block=center`；`inline=nearest`；`behavior=auto` | `selector`、`block`、`inline`、`behavior` | - |

`dom.dispatch_event.event` 可选 `input|change|blur|focus|submit|reset`。滚动对齐的 `block/inline` 可选 `start|center|end|nearest`，行为可选 `auto|smooth`。

`dom.input/check/select` 使用页面主世界中的原生 setter，并派发冒泡的 `input` 和 `change` 事件。`dom.click` 调用元素的 `click()`；它与下一节通过 CDP 发送坐标事件的 `input.click` 不同。

## 真实输入

真实输入 Action 全部要求标签页，并在同一标签页内串行执行。它们使用 CDP debugger；目标被其他 DevTools/CDP 客户端独占时可能失败。

| Action | 参数 | `data` 返回 |
| --- | --- | --- |
| `input.click` | `selector*`；`button=left` (`left|right|middle`)；`click_count=1[1..3]` | `selector`、坐标 `x/y`、`button`、`click_count` |
| `input.hover` | `selector*` | `selector`、坐标 `x/y`、`hovered:true` |
| `input.type` | `selector*`；`text=""`（最大 1 MiB）；`clear=false`；`interval_ms=20[0..5000]` | `selector`、输入 Unicode 字符数 `characters`、`cleared` |
| `input.key` | `key*`；可选 `code`、`text`、`modifiers`；`repeat=1[1..100]` | `key`、`code`、CDP 位掩码 `modifiers`、`repeat` |
| `input.wheel` | 可选 `selector`；`delta_x=0,delta_y=600[-1000000..1000000]` | `selector`、坐标 `x/y`、`delta_x/delta_y` |

`input.key.modifiers` 是逗号分隔的 `Alt,Control,Ctrl,Meta,Command,Shift`，组合后映射为 CDP 位掩码。`input.wheel` 未提供 selector 时使用视口中心。

## Cookie

Cookie Action 都是 sensitive；修改、删除和清空同时是 destructive。操作范围以目标标签页当前 URL 为基础。

| Action | 参数 | `data` 返回 | 标记 |
| --- | --- | --- | --- |
| `cookie.list` | 可选 `name` 过滤 | `url`、`count`、`cookies: cookie[]` | sensitive |
| `cookie.set` | `name*`、`value*`；可选 `domain`；`path=/`；`same_site=unspecified`；`secure=false`；`http_only=false`；可选 `expiration_date>=0` | 创建后的 `cookie` | sensitive, destructive |
| `cookie.delete` | `name*`；可选 `store_id` | `removed`、`name`、`url` | sensitive, destructive |
| `cookie.clear` | 无 | `url`、`removed_count` | sensitive, destructive |

`same_site` 可选 `unspecified|no_restriction|lax|strict`。省略过期时间创建会话 Cookie。Cookie Value 可能包含凭据，不应进入日志或无必要的 Observation。

## 页面存储

Storage Action 都是 sensitive；写入、删除和清空同时是 destructive。`area` 可选 `local|session`，默认 `local`。

| Action | 参数 | `data` 返回 | 标记 |
| --- | --- | --- | --- |
| `storage.get` | `area=local`；可选 `key`；`max_value_length=65536[1..1048576]` 字节 | `area`、`count`、`values`、`truncated` | sensitive |
| `storage.set` | `area=local`；`key*`；`value*`（最大 1 MiB） | `area`、`key`、`written:true`、JS 字符长度 `size` | sensitive, destructive |
| `storage.remove` | `area=local`；`key*` | `area`、`key`、`removed` | sensitive, destructive |
| `storage.clear` | `area=local` | `area`、`removed_count` | sensitive, destructive |

`storage.get` 的单值截断按 UTF-8 字节计算，整个返回对象另有约 4 MiB 上限。指定不存在的 Key 时返回空 `values` 和 `count:0`。

## JavaScript 运行时

| Action | 目标 | 参数 | `data` 返回 | 标记 |
| --- | --- | --- | --- | --- |
| `runtime.evaluate` | `tab_required` | `expression*`，1 到 100000 字符 | `value`：表达式的可 JSON 序列化结果 | sensitive, destructive |

表达式在页面主世界执行，可以访问页面全局变量，也可以修改页面。循环引用、函数、DOM 节点等不能稳定 JSON 序列化的结果不应作为返回值。不要拼接不可信输入。

## Selector 候选接口

Selector 候选不是 Action，不在 56 项列表内。插件通常不直接调用它；它由管理界面的 `css_selector` 参数编辑器使用：

```json
{
  "target": { "agent_id": "agent-id", "tab_id": 21 },
  "queries": ["button", "a[href]", "[role=button]"],
  "limit": 50
}
```

查询数为 1 到 16，单条最长 4096 字符，`limit` 默认 50、最大 200。响应包含 `items`、`total` 和 `truncated`；每项包含 `selector`、`tag_name`、短文本、白名单属性、`visible`、`unique`。

## Capability 与兼容性

每个 Action 的 capability 当前与 Action type 相同。插件无需也不能声明自己需要的 capability；宿主执行时检查目标 Agent 是否声明对应字符串。新增可选返回字段可在协议 v1 内演进，因此插件应忽略未知字段。删除字段、改变字段语义或 Action 行为需要同步协议版本和文档。

开发流程和资源清理见[浏览器能力插件开发](/development/browser-plugin)，插件与宿主之间的流协议见[跨语言插件协议](/development/plugin-protocol)，宿主与 Chrome 扩展之间的协议见[浏览器自动化架构](/development/browser-automation)。
