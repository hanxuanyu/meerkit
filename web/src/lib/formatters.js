export function labelFor(key, schema = {}) {
  return schema.title || ({
    url: "URL",
    method: "请求方法",
    headers: "请求头",
    body: "请求体",
    host: "主机",
    port: "端口",
    from: "发件人",
    to: "收件人",
    username: "用户名",
    password: "密码",
    token: "Token",
    timeout_seconds: "超时(秒)",
    response_mode: "响应模式",
    normalize: "内容规范化",
    verify_tls: "校验证书",
    read_response: "读取响应",
    read_timeout_seconds: "读取超时(秒)",
    max_read_bytes: "最大读取字节数",
    max_body_bytes: "最大正文字节数",
    subject_prefix: "主题前缀"
  }[key] || key);
}

export function placeholderFor(key) {
  return { url: "https://example.com/api", host: "127.0.0.1", port: "8080", headers: "JSON 请求头" }[key] || "";
}

export function pageTitle(page) {
  if (page?.startsWith("monitor-details:") || page?.startsWith("monitor-records:")) return "执行记录";
  return { overview: "总览", monitors: "监控项", inbox: "通知中心", notifications: "通知渠道", plugins: "监控插件", settings: "系统设置" }[page];
}

export function formatDate(value, fallback = "尚未执行") {
  if (!value) return fallback;
  return new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}
