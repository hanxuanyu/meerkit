export const statusColorPresets = [
  { id: "green", name: "绿色", color: "#22c55e" },
  { id: "lime", name: "黄绿", color: "#84cc16" },
  { id: "yellow", name: "黄色", color: "#eab308" },
  { id: "amber", name: "琥珀", color: "#f59e0b" },
  { id: "orange", name: "橙色", color: "#f97316" },
  { id: "red", name: "红色", color: "#ef4444" }
];

export function statusColorPreset(id, level = "success") {
  const fallback = level === "failure" ? "red" : level === "warning" ? "amber" : "green";
  return statusColorPresets.find((preset) => preset.id === id) || statusColorPresets.find((preset) => preset.id === fallback) || statusColorPresets[0];
}
