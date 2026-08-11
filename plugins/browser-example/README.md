# Browser Example Monitor

这是用于验证 Meerkit Browser Capability 链路的最小业务插件。它负责具体的站点配置和结果语义，不负责连接或控制 Chrome Extension。

配置项：

- 页面地址
- CSS Selector
- 可选的接口 URL 片段

执行时插件通过宿主注入的 Browser Capability gRPC 客户端请求平台打开页面、等待并读取元素，同时可捕获匹配的网络响应。页面文本、页面标题、最终 URL、元素标签、接口状态码和响应正文会作为结果集返回。
