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
  changed: "发生变化"
};

export const defaultModuleConfig = {
  http: { method: "GET", response_mode: "auto", normalize: "trim", verify_tls: true, timeout_seconds: 30 },
  tcp: { timeout_seconds: 10, read_response: false, read_timeout_seconds: 3 }
};
