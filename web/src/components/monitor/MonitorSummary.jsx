import React from "react";
import { Badge } from "../ui/Badge";
import { getParameterOptions, isParameterVisible } from "../../lib/parameterSchema";

export function MonitorSummary({ monitor, descriptor }) {
  const values = monitor?.module_config || {};
  const parameters = (descriptor?.parameters || []).filter((parameter) => isParameterVisible(parameter, values));
  return <section className="monitor-summary"><div className="monitor-summary-heading"><div><span className="eyebrow">MONITOR CONFIGURATION</span><h2>监控内容</h2></div><Badge variant="outline">{monitor?.module_type?.toUpperCase()}</Badge></div><div className="monitor-summary-grid">{parameters.length ? parameters.map((parameter) => <SummaryField key={parameter.key} parameter={parameter} value={values[parameter.key]} values={values} />) : <pre>{JSON.stringify(values, null, 2)}</pre>}</div></section>;
}

function SummaryField({ parameter, value, values }) {
  if (value === undefined || value === null || value === "" || (parameter.type === "map" && Object.keys(value || {}).length === 0)) return null;
  let display = value;
  if (parameter.type === "boolean") display = value ? "是" : "否";
  else if (parameter.type === "list") display = getParameterOptions(parameter, values).find((option) => String(option.value) === String(value))?.label || value;
  else if (typeof value === "object") display = JSON.stringify(value, null, 2);
  return <div className={`monitor-summary-field${typeof display === "string" && display.includes("\n") ? " monitor-summary-field-wide" : ""}`}><span>{parameter.label}</span><strong title={typeof display === "string" ? display : undefined}>{parameter.secret ? "••••••" : display}{parameter.unit && <small>{parameter.unit}</small>}</strong>{parameter.description && <em>{parameter.description}</em>}</div>;
}
