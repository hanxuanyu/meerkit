export function conditionMeta(state) {
  if (state === "true") return { label: "满足", tone: "success" };
  if (state === "false") return { label: "未满足", tone: "muted" };
  return { label: "未知", tone: "warning" };
}

export function eventMeta(type) {
  if (type === "triggered") return { label: "已触发", tone: "warning" };
  if (type === "recovered") return { label: "已恢复", tone: "success" };
  return { label: "无事件", tone: "muted" };
}
