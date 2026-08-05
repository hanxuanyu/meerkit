export const operators = {
  equals: "等于",
  not_equals: "不等于",
  contains: "包含",
  not_contains: "不包含",
  regex: "正则匹配",
  gt: "大于",
  gte: "大于等于",
  lt: "小于",
  lte: "小于等于",
  is_true: "为真",
  is_false: "为假",
  exists: "存在",
  not_exists: "不存在",
  between: "位于范围内",
  length_gt: "长度大于",
  length_eq: "长度等于",
  changed: "发生变化"
};

export const defaultModuleConfig = {
  http: { method: "GET", body_mode: "none", response_mode: "auto", normalize: "trim", verify_tls: true, follow_redirects: true, max_redirects: 10, timeout_seconds: 30, max_body_bytes: 262144 },
  tcp: { timeout_seconds: 10, read_response: false, read_timeout_seconds: 3 }
};

export const cronPresets = [
  { value: "*/1 * * * *", label: "每分钟" },
  { value: "*/5 * * * *", label: "每 5 分钟" },
  { value: "*/15 * * * *", label: "每 15 分钟" },
  { value: "*/30 * * * *", label: "每 30 分钟" },
  { value: "0 * * * *", label: "每小时" },
  { value: "0 0 * * *", label: "每天零点" },
  { value: "0 9 * * 1-5", label: "工作日 09:00" },
  { value: "@hourly", label: "每小时（预设）" },
  { value: "@daily", label: "每天（预设）" }
];
