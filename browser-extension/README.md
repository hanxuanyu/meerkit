# Meerkit Browser Agent

Meerkit Browser Agent 是平台维护的通用 Chrome 执行端。它不包含任何特定网站的监控逻辑，只负责接收 Browser Capability 指令并调用 Chrome APIs 或 CDP 完成浏览器操作。

## 开发加载

1. 打开 `chrome://extensions`。
2. 开启开发者模式。
3. 选择“加载已解压的扩展程序”，加载本目录。
4. 在 Meerkit 的系统设置中取得 WebSocket 地址和配对令牌。
5. 打开扩展设置，填写地址和令牌。

扩展会持久化执行节点 ID 和连接设置，并自动重连 Meerkit。配对令牌只保存在 Chrome 本地扩展存储中。

## 能力

- 枚举 Chrome 窗口、标签页和标签分组
- 创建、导航、关闭和分组标签页
- 等待页面、时间或 CSS Selector
- 获取完整 DOM 或选择器对应元素
- 点击元素和填写表单控件
- 在页面主世界执行 JavaScript
- 以 PNG、JPEG 或 WebP 截取当前可见区域或完整页面
- 通过 `chrome.debugger` 和 CDP 捕获匹配的网络请求、响应正文、标头、连接、缓存与时序信息

业务 URL、选择器、操作顺序和监控结果语义由独立的 Meerkit 监控插件维护。扩展不保存站点流程或标签页复用关系。

页面类 Action 必须显式传入 `tab_id`；同时传入 `window_id` 时扩展会校验标签页归属。`tab.open` 只接受可选的 `window_id`，成功后返回新标签页目标。网络捕获不是 Action，而是固定绑定启动时标签页的独立会话，通过 `browser.network.start` 和 `browser.network.stop` 管理；切换管理页当前选择不会迁移捕获目标。

扩展与 Meerkit 使用协议版本 `1` 的 WebSocket 长连接。连接或目标标签页断开时，扩展会停止相关 CDP 会话并释放 debugger attachment。

完整页面截图等超过 512 KiB 的结果会自动拆成有界 `response_chunk`，并根据 WebSocket 发送缓冲区施加背压。结果超过 60 MiB 时只会终止当前请求，不会主动断开 Agent；超长页面应优先使用 WebP 或 JPEG 并降低图片质量。
