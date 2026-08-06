import React from "react";
import { Trash2 } from "lucide-react";
import { operators } from "../../lib/constants";
import { getResultFieldGroups } from "../../lib/resultSchema";
import { IconButton } from "../ui/IconButton";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "../ui/Select";
import { EditableComparison, JsonPathInput } from "./ConditionValueEditor";
import { executionComparisonOperators, fieldReferenceOperators, fieldSelection, findRuleField, parseFieldSelection, previousExecutionSelection, valueOperatorsWithoutInput } from "./conditionFields";

export function ConditionEditor({ descriptor, value, onChange }) {
  const fieldGroups = getResultFieldGroups(descriptor, { includePrevious: false });
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

function ConditionRow({ rule, fieldGroups, fields, onChange, onRemove }) {
  const field = findRuleField(rule, fields);
  const comparesExecutions = rule.source === "previous" && rule.value_source === "current";
  const operatorsForField = comparesExecutions ? executionComparisonOperators(field) : field?.operators || ["equals"];
  const operator = operatorsForField.includes(rule.operator) ? rule.operator : operatorsForField[0] || "equals";
  const showValue = !valueOperatorsWithoutInput.has(operator);
  const canCompareField = showValue && fieldReferenceOperators.has(operator);

  const changeField = (selection) => {
    if (selection === previousExecutionSelection) {
      const comparisonOperators = executionComparisonOperators(field);
      const nextOperator = comparisonOperators.includes(operator) ? operator : comparisonOperators[0];
      onChange({ field: field?.name || "", source: "previous", path: rule.path || "", operator: nextOperator, value_source: "current", value_field: field?.name || "", value_path: rule.path || "", value: "" });
      return;
    }
    const nextField = parseFieldSelection(selection, fields);
    onChange({ field: nextField?.name || "", source: nextField?.source || "current", path: "", operator: nextField?.operators?.[0] || "equals", value_source: "literal", value_field: "", value_path: "", value: "" });
  };
  const changeOperator = (nextOperator) => comparesExecutions
    ? onChange({ operator: nextOperator, value_source: "current", value_field: field?.name || "", value_path: rule.path || "" })
    : onChange({ operator: nextOperator, value_source: fieldReferenceOperators.has(nextOperator) ? (rule.value_source || "literal") : "literal", value_field: fieldReferenceOperators.has(nextOperator) ? rule.value_field : "", value_path: fieldReferenceOperators.has(nextOperator) ? rule.value_path : "" });
  const changeExecutionField = (selection) => {
    const nextField = parseFieldSelection(selection, fields);
    const comparisonOperators = executionComparisonOperators(nextField);
    onChange({
      field: nextField?.name || "",
      source: "previous",
      path: "",
      operator: comparisonOperators.includes(operator) ? operator : comparisonOperators[0],
      value_source: "current",
      value_field: nextField?.name || "",
      value_path: "",
      value: ""
    });
  };
  const changeExecutionPath = (path) => onChange({ path, value_path: path });

  return <div className="condition-row">
    <div className={`condition-left${!comparesExecutions && field?.path ? " condition-json-operand" : ""}`}>
      <Select value={comparesExecutions ? previousExecutionSelection : fieldSelection(field)} onValueChange={changeField}><SelectTrigger><SelectValue placeholder="选择结果字段" /></SelectTrigger><SelectContent><SelectGroup><SelectLabel>执行对比</SelectLabel><SelectItem value={previousExecutionSelection}>上次执行</SelectItem></SelectGroup>{fieldGroups.map((group) => <SelectGroup key={group.key}><SelectLabel>{group.label}</SelectLabel>{group.fields.map((item) => <SelectItem key={fieldSelection(item)} value={fieldSelection(item)}>{item.label}</SelectItem>)}</SelectGroup>)}</SelectContent></Select>
      {!comparesExecutions && field?.path && <JsonPathInput value={rule.path || ""} onChange={(path) => onChange({ path })} placeholder="JSON 路径，如 data.status" />}
    </div>
    <Select value={operator} onValueChange={changeOperator}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{operatorsForField.map((item) => <SelectItem key={item} value={item}>{operators[item] || item}</SelectItem>)}</SelectContent></Select>
    {comparesExecutions ? <div className={`condition-comparison${field?.path ? " condition-json-operand" : ""}`}><Select value={fieldSelection(field)} onValueChange={changeExecutionField}><SelectTrigger aria-label="选择当前执行结果字段"><span>当前执行 · {field?.label || "选择结果字段"}</span></SelectTrigger><SelectContent>{fieldGroups.map((group) => <SelectGroup key={group.key}><SelectLabel>{group.label}</SelectLabel>{group.fields.map((item) => <SelectItem key={fieldSelection(item)} value={fieldSelection(item)}>{item.label}</SelectItem>)}</SelectGroup>)}</SelectContent></Select>{field?.path && <JsonPathInput value={rule.path || ""} onChange={changeExecutionPath} placeholder="当前与上次 JSON 路径" />}</div> : showValue ? <EditableComparison rule={rule} operator={operator} field={field} fieldGroups={fieldGroups} fields={fields} canCompareField={canCompareField} onChange={onChange} /> : <span className="condition-value-placeholder" aria-hidden="true" />}
    <IconButton size="sm" title="删除条件" aria-label="删除条件" onClick={onRemove}><Trash2 size={14} /></IconButton>
  </div>;
}
