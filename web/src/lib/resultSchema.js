const commonPlaceholders = [
  { key: "monitor.name", label: "监控项名称", type: "string" },
  { key: "monitor.module_type", label: "监控模块", type: "string" },
  { key: "event.type", label: "事件类型", type: "string" },
  { key: "event.condition_state", label: "条件状态", type: "string" },
  { key: "event.summary", label: "执行摘要", type: "string" },
  { key: "event.triggered_at", label: "触发时间", type: "datetime" },
  { key: "result", label: "完整结果 JSON", type: "json" }
];

const fallbackOperators = {
  string: ["equals", "not_equals", "contains", "not_contains", "regex", "changed"],
  text: ["equals", "not_equals", "contains", "not_contains", "regex", "changed"],
  number: ["equals", "not_equals", "gt", "gte", "lt", "lte", "between", "changed"],
  integer: ["equals", "not_equals", "gt", "gte", "lt", "lte", "between", "changed"],
  boolean: ["is_true", "is_false", "changed"],
  json: ["exists", "equals", "not_equals", "contains", "changed"],
  object: ["exists", "contains", "changed"],
  map: ["exists", "contains", "length_gt", "length_eq", "changed"],
  array: ["exists", "contains", "length_gt", "length_eq", "changed"],
  binary: ["exists", "equals", "changed"]
};

export function getResultSets(descriptor) {
  if (Array.isArray(descriptor?.result_sets) && descriptor.result_sets.length) return descriptor.result_sets;
  if (Array.isArray(descriptor?.fields) && descriptor.fields.length) return [{ key: "result", label: "执行结果", fields: descriptor.fields }];
  return [];
}

export function getResultFields(descriptor) {
  return getResultSets(descriptor).flatMap((set) => (set.fields || []).map((field) => ({
    ...field,
    name: `${set.key}.${field.name}`,
    source_name: field.name,
    set_key: set.key,
    set_label: set.label,
    label: set.key === "result" ? field.label : `${set.label} · ${field.label}`,
    type: field.type || "string",
    operators: field.operators?.length ? field.operators : fallbackOperators[field.type] || fallbackOperators.string
  })));
}

export function getAvailablePlaceholders(descriptor) {
  const fields = getResultFields(descriptor).map((field) => ({ key: `result.${field.name}`, label: field.label, type: field.type, description: field.description }));
  return [...commonPlaceholders, ...fields];
}

export function placeholderSet(descriptor) {
  return new Set(getAvailablePlaceholders(descriptor).map((item) => item.key));
}

export function findUnsupportedPlaceholders(value, descriptor) {
  const supported = placeholderSet(descriptor);
  const missing = new Set();
  const walk = (current) => {
    if (typeof current === "string") {
      for (const match of current.matchAll(/\{\{\s*([^{}]+?)\s*\}\}/g)) {
        const key = match[1].trim();
        if ((key === "result" || key.startsWith("result.")) && !supported.has(key)) missing.add(key);
      }
    } else if (Array.isArray(current)) current.forEach(walk);
    else if (current && typeof current === "object") Object.values(current).forEach(walk);
  };
  walk(value);
  return [...missing].sort();
}

export function formatResultValue(value, field = {}) {
  if (value === undefined || value === null) return "-";
  if (field.type === "binary") return `${String(value).slice(0, 48)}${String(value).length > 48 ? "…" : ""}`;
  if (typeof value === "object") return JSON.stringify(value, null, 2);
  return `${value}${field.unit ? ` ${field.unit}` : ""}`;
}
