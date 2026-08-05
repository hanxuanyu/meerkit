import React from "react";
import { Badge } from "../ui/Badge";
import { operators } from "../../lib/constants";
import { formatResultValue, getResultFields, getResultSets } from "../../lib/resultSchema";

export function ResultRenderer({ descriptor, result }) {
  const sets = getResultSets(descriptor);
  if (!sets.length) return <pre>{JSON.stringify(result || {}, null, 2)}</pre>;
  return <div className="result-sets">{sets.map((set) => <ResultSet key={set.key} set={set} result={result || {}} descriptor={descriptor} />)}</div>;
}

function ResultSet({ set, result, descriptor }) {
  const values = result?.[set.key] && typeof result[set.key] === "object" ? result[set.key] : result;
  return <section className="result-set"><div className="result-set-heading"><div><h3>{set.label}</h3>{set.description && <p>{set.description}</p>}</div></div><div className="result-field-grid">{(set.fields || []).map((field) => <ResultField key={field.name} field={field} value={values?.[field.name]} descriptor={descriptor} />)}</div></section>;
}

function ResultField({ field, value, descriptor }) {
  const type = field.type || "string";
  const display = formatResultValue(value, field);
  return <div className={`result-field result-field-${type}`}><div className="result-field-label"><span>{field.label}</span>{field.unit && <small>{field.unit}</small>}</div>{field.format === "condition_list" ? <ConditionList value={value} descriptor={descriptor} /> : type === "boolean" ? <Badge tone={value ? "success" : "muted"}>{value ? "是" : "否"}</Badge> : ["json", "object", "map", "array"].includes(type) ? <pre>{display}</pre> : <strong title={String(display)}>{display}</strong>}</div>;
}

function ConditionList({ value, descriptor }) {
  const details = Array.isArray(value) ? value : [];
  const fields = getResultFields(descriptor);
  if (!details.length) return <span className="result-condition-empty">没有配置触发条件</span>;
  return <div className="result-condition-list">{details.map((detail, index) => {
    const field = fields.find((item) => item.name === detail.field);
    const label = field?.label || detail.field || "未知字段";
    const operator = operators[detail.operator] || detail.operator || "判断";
    const expected = Object.prototype.hasOwnProperty.call(detail, "expected") ? formatResultValue(detail.expected, field || {}) : "";
    const actual = formatResultValue(detail.actual, field || {});
    const actualSource = detail.source === "previous" ? "上一次结果" : "当前结果";
    const expectedSource = detail.operator === "changed" ? "上一次结果" : detail.value_source === "previous" ? "上一次结果" : detail.value_source === "current" ? "当前结果" : "";
    const description = detail.operator === "changed" || expected === "" ? `${actualSource} · ${label} ${operator}` : `${actualSource} · ${label} ${operator} ${expectedSource ? `${expectedSource} ` : ""}${expected}`;
    const tone = detail.state === "true" ? "success" : detail.state === "unknown" ? "warning" : "muted";
    const stateLabel = detail.state === "true" ? "满足" : detail.state === "unknown" ? "未知" : "不满足";
    return <div className="result-condition-item" key={`${detail.source || "current"}-${detail.field}-${detail.operator}-${index}`}><div className="result-condition-main"><strong title={description}>{description}</strong><span title={`${actualSource}实际值：${actual}`}>实际值：{actual}</span>{detail.message && <small title={detail.message}>{detail.message}</small>}</div><Badge tone={tone}>{stateLabel}</Badge></div>;
  })}</div>;
}
