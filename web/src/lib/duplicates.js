function cloneValue(value) {
  if (typeof structuredClone === "function") return structuredClone(value);
  return JSON.parse(JSON.stringify(value));
}

function duplicateName(name) {
  const value = String(name || "未命名").trim();
  const match = value.match(/^(.*) 副本(?: (\d+))?$/);
  if (!match) return `${value} 副本`;
  return `${match[1]} 副本 ${(Number(match[2]) || 1) + 1}`;
}

export function duplicateMonitorDraft(monitor) {
  const draft = cloneValue(monitor);
  return { ...draft, id: "", name: duplicateName(draft.name), __duplicate: true };
}

export function duplicateChannelDraft(channel) {
  const draft = cloneValue(channel);
  return { ...draft, id: "", name: duplicateName(draft.name), built_in: false, __duplicate: true };
}

export function duplicateStatusBoardDraft(item) {
  const draft = cloneValue(item);
  return {
    ...draft,
    id: "",
    name: duplicateName(draft.name),
    trend_rules: (draft.trend_rules || []).map((rule) => ({ ...rule, id: crypto.randomUUID() })),
    runtime_state: undefined,
    samples: undefined,
    current: undefined,
    __duplicate: true
  };
}
