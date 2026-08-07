export function eventLabel(type) {
  return { triggered: "触发", recovered: "恢复", trend_triggered: "趋势触发", trend_recovered: "趋势恢复", test: "测试" }[type] || type || "通知";
}

export function browserNotificationLabel(status) {
  return { enabled: "接收系统级提醒", denied: "已被浏览器阻止", unsupported: "当前环境不可用" }[status] || "开启后接收系统级提醒";
}
