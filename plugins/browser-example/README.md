# Browser Example Monitor

这是一个演示“单个插件提供多个监控模块”的 Browser Capability 示例插件。模块名称与类型均包含 `example`，用于明确提示这些模块只作为开发参考：

- `browser-example-html`：获取页面最终 DOM 的 HTML。
- `browser-example-css-text`：获取指定 CSS Selector 对应元素的文本。
- `browser-example-response`：获取 URL 包含指定文本的最近一次网络响应。

三个模块都提供 boolean 参数 `always_new_tab`：

- 默认 `false`。首次执行找不到关联标签页时创建标签页并保留；后续执行刷新同一标签页，以便继续使用用户完成登录后的会话。
- 设置为 `true` 时，每次执行都创建新的临时标签页，执行结束后关闭。

默认复用标识由模块类型和原始页面地址组成，避免不同示例模块互相抢占标签页。同一模块、同一地址需要维护多个独立登录会话时，可填写 `tab_reuse_key` 显式区分。

扩展会同时记录稳定复用标识、标签页 ID 和页面最终重定向地址。即使打开页面发生登录跳转或常规重定向，后续执行仍刷新关联的原标签页；插件创建的标签页会放入同一个 `Meerkit` 标签页分组。
