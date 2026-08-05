import { api } from "./api";

export function previewSchedule(expression) {
  return api("/api/v1/schedules/preview", { method: "POST", body: JSON.stringify({ expression: String(expression || "").trim() }) });
}

export function schedulePreviewLabel(preview) {
  if (!preview) return "";
  const next = preview.next_runs?.[0];
  return `${preview.description}${next ? ` · 下次 ${formatScheduleTime(next)}` : ""}`;
}

export function schedulePreviewTitle(preview) {
  const times = (preview?.next_runs || []).map(formatScheduleTime);
  return times.length ? `${preview.description}；接下来：${times.join("、")}` : preview?.description || "";
}

function formatScheduleTime(value) {
  return new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}
