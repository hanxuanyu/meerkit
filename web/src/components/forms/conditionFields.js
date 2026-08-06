export const fieldReferenceOperators = new Set(["equals", "not_equals", "gt", "gte", "lt", "lte", "contains", "not_contains"]);
export const valueOperatorsWithoutInput = new Set(["changed", "is_true", "is_false", "exists", "not_exists"]);
export const previousExecutionSelection = "__previous_execution__";

export function normalizeFieldName(name) {
  return String(name || "").replace(/^(result|current|previous)\./, "");
}

export function fieldSelection(field) {
  return field ? `${field.source}|${field.name}` : "";
}

export function parseFieldSelection(selection, fields) {
  const separator = selection.indexOf("|");
  const source = separator > 0 ? selection.slice(0, separator) : "current";
  const name = normalizeFieldName(separator > 0 ? selection.slice(separator + 1) : selection);
  return fields.find((field) => field.source === source && field.name === name) || fields[0];
}

export function findRuleField(rule, fields) {
  const name = normalizeFieldName(rule.field);
  return fields.find((field) => field.source === "current" && field.name === name) || fields.find((field) => field.source === "current") || fields[0];
}

export function executionComparisonOperators(field) {
  if (["number", "integer"].includes(field?.type)) return ["equals", "not_equals", "gt", "gte", "lt", "lte"];
  if (["string", "text"].includes(field?.type)) return ["equals", "not_equals", "contains", "not_contains"];
  return ["equals", "not_equals"];
}

export function parseLiteralValue(value, field, operator) {
  if (!["number", "integer"].includes(field?.type) || operator === "between" || value.trim() === "") return value;
  const number = Number(value);
  return Number.isFinite(number) ? number : value;
}
