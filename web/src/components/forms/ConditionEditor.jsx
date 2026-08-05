import React from "react";
import { Trash2 } from "lucide-react";
import { operators } from "../../lib/constants";
import { getResultFieldGroups } from "../../lib/resultSchema";
import { IconButton } from "../ui/IconButton";
import { Input } from "../ui/Input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "../ui/Select";

const fieldReferenceOperators = new Set(["equals", "not_equals", "gt", "gte", "lt", "lte", "contains", "not_contains"]);
const valueOperatorsWithoutInput = new Set(["changed", "is_true", "is_false", "exists", "not_exists"]);

export function ConditionEditor({ descriptor, value, onChange }) {
  const fieldGroups = getResultFieldGroups(descriptor);
  const fields = fieldGroups.flatMap((group) => group.fields);
  const rules = value.rules || [];
  const notificationPolicy = value.notification_policy === "every" ? "every" : "once";
  const updateRule = (index, patch) => onChange({ ...value, rules: rules.map((rule, itemIndex) => itemIndex === index ? { ...rule, ...patch } : rule) });
  const removeRule = (index) => onChange({ ...value, rules: rules.filter((_, itemIndex) => itemIndex !== index) });

  return <>
    <div className="condition-toolbar"><span>满足</span><Select value={value.logic || "ALL"} onValueChange={(logic) => onChange({ ...value, logic })}><SelectTrigger className="condition-select"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ALL">全部条件</SelectItem><SelectItem value="ANY">任一条件</SelectItem></SelectContent></Select><span>时触发通知</span><span>通知策略</span><Select value={notificationPolicy} onValueChange={(notification_policy) => onChange({ ...value, notification_policy })}><SelectTrigger className="condition-policy-select"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="once">同一场景仅通知一次</SelectItem><SelectItem value="every">每次满足条件都通知</SelectItem></SelectContent></Select></div>
    <div className="condition-list">{rules.length ? rules.map((rule, index) => <ConditionRow key={`${rule.source || "current"}-${rule.field}-${index}`} rule={rule} fieldGroups={fieldGroups} fields={fields} onChange={(patch) => updateRule(index, patch)} onRemove={() => removeRule(index)} />) : <div className="condition-empty">点击右上角 + 添加触发条件</div>}</div>
  </>;
}

function normalizeFieldName(name) {
  return String(name || "").replace(/^(result|current|previous)\./, "");
}

function fieldSelection(field) {
  return field ? `${field.source}|${field.name}` : "";
}

function parseFieldSelection(selection, fields) {
  const separator = selection.indexOf("|");
  const source = separator > 0 ? selection.slice(0, separator) : "current";
  const name = normalizeFieldName(separator > 0 ? selection.slice(separator + 1) : selection);
  return fields.find((field) => field.source === source && field.name === name) || fields[0];
}

function findRuleField(rule, fields) {
  const source = rule.source === "previous" || String(rule.field || "").startsWith("previous.") ? "previous" : "current";
  const name = normalizeFieldName(rule.field);
  return fields.find((field) => field.source === source && field.name === name) || fields.find((field) => field.source === "current") || fields[0];
}

function ConditionRow({ rule, fieldGroups, fields, onChange, onRemove }) {
  const field = findRuleField(rule, fields);
  const operator = rule.operator || field?.operators?.[0] || "equals";
  const operatorsForField = field?.operators || ["equals"];
  const showValue = !valueOperatorsWithoutInput.has(operator);
  const canCompareField = showValue && fieldReferenceOperators.has(operator);
  const valueSource = canCompareField && (rule.value_source === "current" || rule.value_source === "previous") ? rule.value_source : "literal";
  const valueField = fields.find((item) => item.source === valueSource && item.name === normalizeFieldName(rule.value_field)) || fields.find((item) => item.source === valueSource);

  const changeField = (selection) => {
    const nextField = parseFieldSelection(selection, fields);
    onChange({ field: nextField?.name || "", source: nextField?.source || "current", path: "", operator: nextField?.operators?.[0] || "equals", value_source: "literal", value_field: "", value_path: "", value: "" });
  };
  const changeOperator = (nextOperator) => onChange({ operator: nextOperator, value_source: fieldReferenceOperators.has(nextOperator) ? (rule.value_source || "literal") : "literal", value_field: fieldReferenceOperators.has(nextOperator) ? rule.value_field : "", value_path: fieldReferenceOperators.has(nextOperator) ? rule.value_path : "" });
  const changeValueSource = (nextSource) => onChange({ value_source: nextSource, value_field: nextSource === "literal" ? "" : valueField?.name || "", value_path: "" });
  const changeValueField = (selection) => {
    const nextField = parseFieldSelection(selection, fields);
    onChange({ value_field: nextField?.name || "", value_path: "" });
  };
  const valueInputType = ["number", "integer"].includes(field?.type) ? "number" : "text";

  return <div className="condition-row">
    <div className="condition-left">
      <Select value={fieldSelection(field)} onValueChange={changeField}><SelectTrigger><SelectValue placeholder="选择结果字段" /></SelectTrigger><SelectContent>{fieldGroups.map((group) => <SelectGroup key={group.key}><SelectLabel>{group.label}</SelectLabel>{group.fields.map((item) => <SelectItem key={fieldSelection(item)} value={fieldSelection(item)}>{item.label}</SelectItem>)}</SelectGroup>)}</SelectContent></Select>
      {field?.path && <Input value={rule.path || ""} onChange={(event) => onChange({ path: event.target.value })} placeholder="JSON 路径，如 data.status" />}
    </div>
    <Select value={operator} onValueChange={changeOperator}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{operatorsForField.map((item) => <SelectItem key={item} value={item}>{operators[item] || item}</SelectItem>)}</SelectContent></Select>
    {showValue ? <div className="condition-comparison"><Select value={valueSource} onValueChange={changeValueSource}><SelectTrigger aria-label="比较值来源"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="literal">固定值</SelectItem>{fieldReferenceOperators.has(operator) && <><SelectItem value="current">当前结果</SelectItem><SelectItem value="previous">上一次结果</SelectItem></>}</SelectContent></Select>{valueSource === "literal" ? <Input className="condition-literal-input" type={valueInputType} value={rule.value ?? ""} onChange={(event) => onChange({ value: ["number", "integer"].includes(field?.type) ? (event.target.value === "" ? "" : Number(event.target.value)) : event.target.value })} placeholder={operator === "between" ? "下限,上限" : "固定比较值"} /> : <><Select value={fieldSelection(valueField)} onValueChange={changeValueField}><SelectTrigger aria-label="选择比较结果字段"><SelectValue placeholder="选择结果字段" /></SelectTrigger><SelectContent>{fieldGroups.filter((group) => group.source === valueSource).map((group) => <SelectGroup key={group.key}><SelectLabel>{group.label}</SelectLabel>{group.fields.map((item) => <SelectItem key={fieldSelection(item)} value={fieldSelection(item)}>{item.label}</SelectItem>)}</SelectGroup>)}</SelectContent></Select>{valueField?.path && <Input className="condition-path-input" value={rule.value_path || ""} onChange={(event) => onChange({ value_path: event.target.value })} placeholder="比较值 JSON 路径" />}</>}</div> : <span className="condition-value-placeholder" aria-hidden="true" />}
    <IconButton size="sm" title="删除条件" aria-label="删除条件" onClick={onRemove}><Trash2 size={14} /></IconButton>
  </div>;
}
