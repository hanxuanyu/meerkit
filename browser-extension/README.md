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
- 创建、聚焦、调整状态/尺寸和关闭窗口
- 创建、激活、导航、刷新、历史前进/后退、复制、移动、固定、静音、卸载、语言检测、缩放、分组和关闭标签页
- 获取页面信息和性能快照，按加载、时间、元素状态或文本条件等待，并滚动或停止加载页面
- 获取完整 DOM、单个或多个选择器对应元素，操作焦点、表单、属性和受限 DOM 事件
- 按参数声明的查询规则发现页面元素，并生成可手动覆盖的 CSS Selector 候选
- 通过 DOM 快速操作或 CDP 发送真实鼠标、键盘和滚轮输入
- 查询和修改当前标签页 URL 的 Cookie、localStorage 与 sessionStorage
- 在页面主世界执行 JavaScript
- 以 PNG、JPEG 或 WebP 截取当前可见区域或完整页面
- 通过 `chrome.debugger` 和 CDP 捕获匹配的网络请求、响应正文、标头、连接、缓存与时序信息

业务 URL、选择器、操作顺序和监控结果语义由独立的 Meerkit 监控插件维护。扩展不保存站点流程或标签页复用关系。

页面类 Action 必须显式传入 `tab_id`；同时传入 `window_id` 时扩展会校验标签页归属。`tab.open` 只接受可选的 `window_id`，成功后返回新标签页目标。网络捕获不是 Action，而是固定绑定启动时标签页的独立会话，通过 `browser.network.start` 和 `browser.network.stop` 管理；切换管理页当前选择不会迁移捕获目标。

`browser.selector_candidates` 每次最多接收 16 条查询并返回 200 个元素摘要；结果不包含 DOM HTML，选择器优先使用唯一 ID 和稳定属性。参数未声明候选查询时，管理界面仍显示普通的手动输入框。

扩展与 Meerkit 使用协议版本 `1` 的 WebSocket 长连接。连接或目标标签页断开时，扩展会停止相关 CDP 会话并释放 debugger attachment。

截图、真实输入和网络捕获按标签页共享同一个 CDP attachment，并使用引用计数释放。同一标签页的输入序列串行执行；多个网络捕获共享 Network 域，停止一个捕获不会中断其他捕获。

Cookie、页面存储和页面主世界 JavaScript 属于敏感能力，只作用于显式选择的标签页。扩展不会把读取值写入本地存储或日志。页面存储单值最多返回 1 MiB，单次总响应最多返回 4 MiB，超出部分会标记截断。

完整页面截图等超过 512 KiB 的结果会自动拆成有界 `response_chunk`，并根据 WebSocket 发送缓冲区施加背压。结果超过 60 MiB 时只会终止当前请求，不会主动断开 Agent；超长页面应优先使用 WebP 或 JPEG 并降低图片质量。

扩展直接使用主应用的 2048×2048 品牌原图，不再生成缩放图或叠加角标。修改 `web/MeerKit.png` 后，可在项目根目录执行 `go run ./browser-extension/tools/generate_icons.go`，将原图原样同步到 `browser-extension/icons/meerkit.png`。
