import React from "react";
import { Badge } from "../ui/Badge";
import { getParameterOptions, getParameters, isParameterVisible } from "../../lib/parameterSchema";

export function MonitorSummary({ monitor, descriptor }) {
  const values = monitor?.module_config || {};
  const parameters = getParameters(descriptor || {}).filter((parameter) => isParameterVisible(parameter, values));
  return <section className="monitor-summary"><div className="monitor-summary-heading"><div><span className="eyebrow">MONITOR CONFIGURATION</span><h2>监控内容</h2></div><Badge variant="outline">{monitor?.module_type?.toUpperCase()}</Badge></div><div className="monitor-summary-badges">{parameters.length ? parameters.map((parameter) => <SummaryField key={parameter.key} parameter={parameter} value={values[parameter.key]} values={values} />) : <FallbackFields values={values} />}</div></section>;
}

function SummaryField({ parameter, value, values }) {
  if (value === undefined || value === null || value === "" || (parameter.type === "map" && Object.keys(value || {}).length === 0)) return null;
  let display = value;
  if (parameter.type === "boolean") display = value ? "是" : "否";
  else if (parameter.type === "list") display = getParameterOptions(parameter, values).find((option) => String(option.value) === String(value))?.label || value;
  else if (typeof value === "object") display = formatStructuredValue(value);
  else display = String(value).replace(/\s*\n\s*/g, " ");
  const visibleValue = parameter.secret ? "••••••" : String(display);
  const tone = parameter.type === "boolean" ? (value ? "success" : "muted") : "neutral";
  return <Badge variant="outline" tone={tone} className="monitor-summary-badge" title={`${parameter.label}：${visibleValue}${parameter.unit || ""}`}><span>{parameter.label}</span><strong>{visibleValue}{parameter.unit && <small>{parameter.unit}</small>}</strong></Badge>;
}

function FallbackFields({ values }) {
  return Object.entries(values).map(([key, value]) => <Badge key={key} variant="outline" className="monitor-summary-badge" title={`${key}：${formatStructuredValue(value)}`}><span>{key}</span><strong>{formatStructuredValue(value)}</strong></Badge>);
}

function formatStructuredValue(value) {
  if (Array.isArray(value)) return value.map((item) => typeof item === "object" ? JSON.stringify(item) : String(item)).join(" · ");
  if (value && typeof value === "object") return Object.entries(value).map(([key, item]) => `${key}: ${typeof item === "object" ? JSON.stringify(item) : String(item)}`).join(" · ");
  return String(value ?? "");
}
