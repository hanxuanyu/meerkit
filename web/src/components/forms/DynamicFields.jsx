import React from "react";
import { labelFor, placeholderFor } from "../../lib/formatters";
import { Input, Label } from "../ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/Select";
import { Switch } from "../ui/Switch";

export function DynamicFields({ properties, values, onChange }) {
  const entries = Object.entries(properties).filter(([key]) => !["headers"].includes(key));
  return <div className="dynamic-fields">{entries.map(([key, schema]) => {
    const type = schema.type || "string";
    const value = values[key] ?? schema.default ?? (type === "boolean" ? false : "");
    if (type === "boolean") return <Switch key={key} className="compact" checked={Boolean(value)} onCheckedChange={(checked) => onChange(key, checked)} label={labelFor(key, schema)} />;
    if (schema.enum) return <label className="field" key={key}><Label>{labelFor(key, schema)}</Label><Select value={String(value)} onValueChange={(next) => onChange(key, next)}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{schema.enum.map((option) => <SelectItem key={option} value={option}>{option}</SelectItem>)}</SelectContent></Select></label>;
    return <label className="field" key={key}><Label>{labelFor(key, schema)}</Label><Input type={schema.secret ? "password" : type === "integer" || type === "number" ? "number" : "text"} value={value} onChange={(event) => onChange(key, type === "integer" || type === "number" ? Number(event.target.value) : event.target.value)} placeholder={placeholderFor(key)} /></label>;
  })}</div>;
}
