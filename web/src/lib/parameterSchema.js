const hasOwn = (value, key) => Object.prototype.hasOwnProperty.call(value, key);

export function getParameters(descriptor) {
  return orderParameters(Array.isArray(descriptor?.parameters) ? descriptor.parameters : []);
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
  if (["integer", "number", "duration", "browser_window", "browser_tab"].includes(parameter.type)) return "";
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

function isEmptyRequiredValue(value) {
  if (value == null) return true;
  if (typeof value === "string") return value.trim() === "";
  if (typeof value === "number") return Number.isNaN(value);
  if (Array.isArray(value)) return value.length === 0;
  if (typeof value === "object") return Object.keys(value).length === 0;
  return false;
}

export function findMissingRequiredParameters(parameters, values = {}) {
  return parameters.filter((parameter) => parameter.required
    && isParameterVisible(parameter, values)
    && isParameterEnabled(parameter, values)
    && isEmptyRequiredValue(values[parameter.key]));
}

export function sanitizeValues(parameters, values) {
  return parameters.reduce((result, parameter) => {
    if (isParameterVisible(parameter, values) && hasOwn(values, parameter.key)) result[parameter.key] = values[parameter.key];
    return result;
  }, {});
}
