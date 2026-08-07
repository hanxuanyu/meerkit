const commonPlaceholders = [
  { key: "monitor.id", label: "监控项 ID", type: "string" },
  { key: "monitor.name", label: "监控项名称", type: "string" },
  { key: "monitor.module_type", label: "监控模块", type: "string" },
  { key: "event.type", label: "事件类型", type: "string" },
  { key: "event.event_type", label: "事件类型（完整字段）", type: "string" },
  { key: "event.record_id", label: "执行记录 ID", type: "string" },
  { key: "event.detail_path", label: "执行详情路径", type: "string" },
  { key: "event.condition_state", label: "条件状态", type: "string" },
  { key: "event.summary", label: "执行摘要", type: "string" },
  { key: "event.triggered_at", label: "触发时间", type: "datetime" },
  { key: "status.item_id", label: "状态看板项 ID", type: "string" },
  { key: "status.item_name", label: "状态看板项名称", type: "string" },
  { key: "trend.rule_id", label: "趋势规则 ID", type: "string" },
  { key: "trend.rule_name", label: "趋势规则名称", type: "string" },
  { key: "trend.detail", label: "趋势计算详情", type: "json" },
  { key: "result", label: "完整结果 JSON", type: "json" },
  { key: "previous", label: "上一次结果 JSON", type: "json" }
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
  if (Array.isArray(descriptor?.fields) && descriptor.fields.length) return [{ key: "result", label: "执行结果", scope: "module", fields: descriptor.fields }];
  return [];
}

function resultSetScope(set) {
  return set.scope || (set.key === "summary" ? "common" : "module");
}

function getFieldsForSets(sets, source) {
  return sets.flatMap((set) => (set.fields || []).map((field) => {
    const type = field.type || "string";
    return {
      ...field,
      name: `${set.key}.${field.name}`,
      source,
      sourceLabel: source === "previous" ? "上一次结果" : "当前结果",
      source_name: field.name,
      set_key: set.key,
      set_label: set.label,
      scope: resultSetScope(set),
      label: `${set.label} · ${field.label}`,
      type,
      operators: field.operators === null ? [] : field.operators === undefined ? fallbackOperators[type] || fallbackOperators.string : field.operators
    };
  }));
}

export function getResultFieldGroups(descriptor, { includePrevious = true, onlyComparable = true } = {}) {
  const sets = getResultSets(descriptor);
  const groups = [];
  const sources = includePrevious ? ["current", "previous"] : ["current"];
  for (const source of sources) {
    for (const scope of ["common", "module"]) {
      const scopedSets = sets.filter((set) => resultSetScope(set) === scope);
      const fields = getFieldsForSets(scopedSets, source).filter((field) => !onlyComparable || field.operators.length > 0);
      if (!fields.length) continue;
      groups.push({
        key: `${source}-${scope}`,
        label: `${source === "previous" ? "上一次结果" : "当前结果"} · ${scope === "common" ? "公共结果集" : "模块结果集"}`,
        source,
        scope,
        fields
      });
    }
  }
  return groups;
}

export function getResultFields(descriptor) {
  return getResultFieldGroups(descriptor, { includePrevious: false }).flatMap((group) => group.fields);
}

export function getAvailablePlaceholders(descriptor) {
  return [
    ...commonPlaceholders,
    ...getResultFieldGroups(descriptor).flatMap((group) => group.fields.map((field) => ({
      key: `${field.source === "previous" ? "previous" : "result"}.${field.name}`,
      label: field.label,
      group: group.key,
      type: field.type,
      description: field.description
    })))
  ];
}

export function getPlaceholderGroups(descriptor) {
  const resultGroups = getResultFieldGroups(descriptor);
  const scopedGroups = [
    { key: "result-common", label: "公共结果", scope: "common" },
    { key: "result-module", label: "模块结果", scope: "module" }
  ].map((target) => ({
    ...target,
    sections: [
      { source: "current", label: "本次执行" },
      { source: "previous", label: "上次执行" }
    ].map((section) => ({
      ...section,
      items: resultGroups
        .filter((group) => group.scope === target.scope && group.source === section.source)
        .flatMap((group) => group.fields.map((field) => ({
          key: `${field.source === "previous" ? "previous" : "result"}.${field.name}`,
          label: field.label,
          type: field.type,
          description: field.description
        })))
    })).filter((section) => section.items.length)
  })).filter((group) => group.sections.length);
  return [{ key: "common", label: "公共字段", items: commonPlaceholders }, ...scopedGroups];
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
        if (!supported.has(key)) missing.add(key);
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
