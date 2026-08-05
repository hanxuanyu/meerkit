const fallbackLabels = {
  url: "URL", method: "请求方法", body_mode: "请求体类型", query: "查询参数", headers: "请求头", form_fields: "表单字段", json_body: "JSON 请求体", raw_body: "原始请求体", host: "主机", port: "端口",
  from: "发件人", to: "收件人", username: "用户名", password: "密码", token: "Token",
  timeout_seconds: "超时(秒)", response_mode: "响应模式", normalize: "内容规范化", verify_tls: "校验证书",
  read_response: "读取响应", read_timeout_seconds: "读取超时(秒)", max_read_bytes: "最大读取字节数",
  max_body_bytes: "最大正文字节数", subject_prefix: "主题前缀"
};

const hasOwn = (value, key) => Object.prototype.hasOwnProperty.call(value, key);

function fallbackType(schema = {}) {
  if (schema.enum || schema.options) return "list";
  if (schema.type === "object") return "map";
  if (schema.type === "string" && schema.multiline) return "text";
  if (schema.type === "string" && schema.format === "uri") return "url";
  if (schema.type === "string" && schema.format === "email") return "email";
  return schema.type || "string";
}

export function getParameters(descriptor) {
  if (Array.isArray(descriptor?.parameters) && descriptor.parameters.length) return orderParameters(descriptor.parameters);
  const properties = descriptor?.config_schema?.properties || {};
  return orderParameters(Object.entries(properties).map(([key, schema = {}]) => ({
    key, label: schema.title || fallbackLabels[key] || key, description: schema.description,
    order: schema.order, full_width: schema.full_width,
    type: fallbackType(schema), required: Boolean(schema.required), default: schema.default,
    placeholder: schema.placeholder, secret: Boolean(schema.secret),
    options: (schema.options || schema.enum || []).map((option) => typeof option === "string" ? { value: option, label: option } : option),
    options_when: (schema.options_when || []).map((set) => ({ when: set.when || [], options: (set.options || []).map((option) => typeof option === "string" ? { value: option, label: option } : option) })),
    minimum: schema.minimum, maximum: schema.maximum, step: schema.step, rows: schema.rows,
    format: schema.format, unit: schema.unit, visible_when: schema.visible_when || [], enabled_when: schema.enabled_when || []
  })));
}

function orderParameters(parameters) {
  return parameters.map((parameter, index) => ({ parameter, index })).sort((left, right) => {
    const leftOrder = left.parameter.order ?? left.index;
    const rightOrder = right.parameter.order ?? right.index;
    return leftOrder - rightOrder || left.index - right.index;
  }).map(({ parameter }) => parameter);
}

function emptyValue(parameter) {
  if (parameter.type === "boolean") return false;
  if (parameter.type === "map") return {};
  if (["integer", "number", "duration"].includes(parameter.type)) return "";
  return "";
}

export function getDefaultValues(descriptor, seed = {}) {
  return getParameters(descriptor).reduce((result, parameter) => {
    if (hasOwn(seed, parameter.key)) result[parameter.key] = seed[parameter.key];
    else if (parameter.default !== undefined) result[parameter.key] = parameter.default;
    else result[parameter.key] = emptyValue(parameter);
    return result;
  }, {});
}

function matchesCondition(condition, values) {
  const current = values?.[condition.field];
  switch (condition.operator || "equals") {
    case "not_equals": return current !== condition.value;
    case "in": return Array.isArray(condition.value) && condition.value.includes(current);
    case "not_in": return Array.isArray(condition.value) && !condition.value.includes(current);
    case "truthy": return Boolean(current);
    case "falsy": return !current;
    case "equals":
    default: return current === condition.value;
  }
}

export function isParameterVisible(parameter, values) {
  return (parameter.visible_when || parameter.visibleWhen || []).every((condition) => matchesCondition(condition, values));
}

export function isParameterEnabled(parameter, values) {
  return (parameter.enabled_when || parameter.enabledWhen || []).every((condition) => matchesCondition(condition, values));
}

export function getParameterOptions(parameter, values) {
  const optionSets = parameter.options_when || parameter.optionsWhen || [];
  const matchingSet = optionSets.find((set) => (set.when || []).every((condition) => matchesCondition(condition, values)));
  return matchingSet ? matchingSet.options || [] : parameter.options || [];
}

export function sanitizeValues(parameters, values) {
  return parameters.reduce((result, parameter) => {
    if (isParameterVisible(parameter, values) && hasOwn(values, parameter.key)) result[parameter.key] = values[parameter.key];
    return result;
  }, {});
}
