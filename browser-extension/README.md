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

- 创建、导航、关闭和分组标签页；支持按稳定关联键复用并刷新已有标签页
- 等待页面、时间或 CSS Selector
- 获取完整 DOM 或选择器对应元素
- 点击元素和填写表单控件
- 在页面主世界执行 JavaScript
- 截取当前页面可见区域
- 通过 `chrome.debugger` 和 CDP 捕获匹配的网络请求、响应正文、标头、连接、缓存与时序信息

业务 URL、选择器、页面流程和监控结果语义由独立的 Meerkit 监控插件维护。

`tab.open` 可通过 `reuse` 与 `reuse_key` 复用标签页。扩展会同时在 session storage 和持久化 local storage 中保存关联键、标签页 ID、目标分组和最终重定向地址；后台 Service Worker 重启或页面跳转后仍可找回原标签页。相同复用标识的并发任务会串行等待，避免因标签页暂时占用而创建副本。

`tab.open` 的 `group_title` 可让新标签页优先创建到已有同名分组所在窗口。`tab.group` 的 `reuse_group` 参数会保留标签页当前的同名分组，或复用窗口中已有的分组，不再为每次执行重复创建分组。
