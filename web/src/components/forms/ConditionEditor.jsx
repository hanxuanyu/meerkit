import React from "react";
import { Trash2 } from "lucide-react";
import { operators } from "../../lib/constants";
import { Button } from "../ui/Button";
import { Input } from "../ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/Select";

export function ConditionEditor({ descriptor, value, onChange }) {
  const fields = descriptor.fields || [];
  const rules = value.rules || [];
  const updateRule = (index, patch) => onChange({ ...value, rules: rules.map((rule, itemIndex) => itemIndex === index ? { ...rule, ...patch } : rule) });
  const removeRule = (index) => onChange({ ...value, rules: rules.filter((_, itemIndex) => itemIndex !== index) });

  return <>
    <div className="condition-toolbar"><span>满足</span><Select value={value.logic || "ALL"} onValueChange={(logic) => onChange({ ...value, logic })}><SelectTrigger className="condition-select"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ALL">全部条件</SelectItem><SelectItem value="ANY">任一条件</SelectItem></SelectContent></Select><span>时触发通知</span></div>
    {rules.length ? <div className="condition-list">{rules.map((rule, index) => <ConditionRow key={`${rule.field}-${index}`} rule={rule} fields={fields} onChange={(patch) => updateRule(index, patch)} onRemove={() => removeRule(index)} />)}</div> : <div className="condition-empty">未设置条件，仅保存采集结果。</div>}
  </>;
}

function ConditionRow({ rule, fields, onChange, onRemove }) {
  const field = fields.find((item) => item.name === rule.field) || fields[0];
  const fieldOptions = fields.map((item) => <SelectItem key={item.name} value={item.name}>{item.label}</SelectItem>);
  const operatorsForField = field?.operators || ["equals"];
  const showValue = rule.operator !== "changed" && !["is_true", "is_false"].includes(rule.operator);
  return <div className="condition-row"><Select value={rule.field} onValueChange={(next) => onChange({ field: next, operator: fields.find((item) => item.name === next)?.operators?.[0] || "equals" })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{fieldOptions}</SelectContent></Select>{field?.path && <Input value={rule.path || ""} onChange={(event) => onChange({ path: event.target.value })} placeholder="JSON 路径，如 data.status" />}<Select value={rule.operator} onValueChange={(operator) => onChange({ operator })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{operatorsForField.map((operator) => <SelectItem key={operator} value={operator}>{operators[operator] || operator}</SelectItem>)}</SelectContent></Select>{showValue && <Input value={rule.value ?? ""} onChange={(event) => onChange({ value: event.target.value })} placeholder="比较值" />}<Button type="button" variant="ghost" size="icon" title="删除条件" onClick={onRemove}><Trash2 size={15} /></Button></div>;
}
