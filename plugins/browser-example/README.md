# Browser Example Monitor

这是一个演示“单个插件提供多个监控模块”的 Browser Capability 示例插件。模块名称与类型均包含 `example`，用于明确提示这些模块只作为开发参考：

- `browser-example-html`：获取页面最终 DOM 的 HTML。
- `browser-example-css-text`：获取指定 CSS Selector 对应元素的文本。
- `browser-example-response`：获取 URL 包含指定文本的最近一次网络响应。

每次执行都先调用 `tab.open` 创建标签页，保存返回的 `window_id/tab_id`，然后把目标显式传给后续原子 Action；执行结束后关闭该标签页。响应模块先在空白页创建独立网络捕获会话，再导航到目标地址，以覆盖首次页面请求，最后显式停止捕获。

插件通过 `PluginRuntime.Browser()` 使用宿主浏览器能力。站点地址、选择器、操作顺序和结果解释均保留在监控插件中；通用 Chrome 扩展不包含这些业务逻辑。
