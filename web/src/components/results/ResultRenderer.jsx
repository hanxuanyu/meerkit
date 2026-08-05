import React from "react";
import { Badge } from "../ui/Badge";
import { formatResultValue, getResultSets } from "../../lib/resultSchema";

export function ResultRenderer({ descriptor, result }) {
  const sets = getResultSets(descriptor);
  if (!sets.length) return <pre>{JSON.stringify(result || {}, null, 2)}</pre>;
  return <div className="result-sets">{sets.map((set) => <ResultSet key={set.key} set={set} result={result || {}} />)}</div>;
}

function ResultSet({ set, result }) {
  const values = result?.[set.key] && typeof result[set.key] === "object" ? result[set.key] : result;
  return <section className="result-set"><div className="result-set-heading"><div><h3>{set.label}</h3>{set.description && <p>{set.description}</p>}</div></div><div className="result-field-grid">{(set.fields || []).map((field) => <ResultField key={field.name} field={field} value={values?.[field.name]} />)}</div></section>;
}

function ResultField({ field, value }) {
  const type = field.type || "string";
  const display = formatResultValue(value, field);
  return <div className={`result-field result-field-${type}`}><div className="result-field-label"><span>{field.label}</span>{field.unit && <small>{field.unit}</small>}</div>{type === "boolean" ? <Badge tone={value ? "success" : "muted"}>{value ? "是" : "否"}</Badge> : ["json", "object", "map", "array"].includes(type) ? <pre>{display}</pre> : <strong title={String(display)}>{display}</strong>}</div>;
}
