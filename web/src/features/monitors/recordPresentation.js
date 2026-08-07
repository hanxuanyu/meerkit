export function conditionMeta(state) {
  if (state === "true") return { label: "满足", tone: "success" };
  if (state === "false") return { label: "未满足", tone: "muted" };
  return { label: "未知", tone: "warning" };
}

export function eventMeta(type) {
  if (type === "triggered") return { label: "已触发", tone: "warning" };
  if (type === "recovered") return { label: "已恢复", tone: "success" };
  if (type === "trend_triggered") return { label: "趋势触发", tone: "warning" };
  if (type === "trend_recovered") return { label: "趋势恢复", tone: "success" };
  return { label: "无事件", tone: "muted" };
}

export function recordEventType(record) {
  if (record?.event_type && record.event_type !== "none") return record.event_type;
  return record?.notification_events?.find((event) => event.source === "status_trend")?.event_type || "none";
}
